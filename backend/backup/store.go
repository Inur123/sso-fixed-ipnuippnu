package backup

import (
	"context"
	"crypto/md5" // Required by the S3 SSE-C protocol, not used for data integrity.
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Object struct {
	Body io.ReadCloser
	Size int64
}
type Store interface {
	Put(context.Context, string, io.ReadSeeker, int64, string) error
	Get(context.Context, string) (Object, error)
	Delete(context.Context, string) error
}

type R2Store struct {
	client                         *s3.Client
	bucket, namespace, key, keyMD5 string
}

var objectName = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}\.dump\.age$`)

func NewR2Store(c Config) *R2Store {
	checksum := md5.Sum(c.StorageKey)
	return &R2Store{
		client: s3.New(s3.Options{Region: "auto", BaseEndpoint: aws.String("https://" + c.AccountID + ".r2.cloudflarestorage.com"), Credentials: credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, "")}),
		bucket: c.Bucket, namespace: c.Namespace, key: base64.StdEncoding.EncodeToString(c.StorageKey), keyMD5: base64.StdEncoding.EncodeToString(checksum[:]),
	}
}

func validObjectKey(namespace, key string) bool {
	return strings.HasPrefix(key, namespace) && objectName.MatchString(strings.TrimPrefix(key, namespace))
}

func (s *R2Store) Put(ctx context.Context, key string, body io.ReadSeeker, size int64, checksum string) error {
	if !validObjectKey(s.namespace, key) {
		return errors.New("object backup di luar namespace")
	}
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key), Body: body, ContentLength: aws.Int64(size),
		ContentType: aws.String("application/octet-stream"), CacheControl: aws.String("private, no-store"),
		SSECustomerAlgorithm: aws.String("AES256"), SSECustomerKey: aws.String(s.key), SSECustomerKeyMD5: aws.String(s.keyMD5),
		Metadata: map[string]string{"sha256": checksum, "format": "pgdump-age-v1"},
	})
	if err != nil {
		return fmt.Errorf("upload R2 gagal: %w", err)
	}
	// Even authenticated readers MUST supply the storage key. Never fall back
	// to ordinary/public R2 objects if SSE-C is not enforced by the endpoint.
	plain, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err == nil {
		plain.Body.Close()
		return errors.New("R2 tidak mewajibkan kunci SSE-C; backup ditolak")
	}
	var status interface{ HTTPStatusCode() int }
	if !errors.As(err, &status) || (status.HTTPStatusCode() != 400 && status.HTTPStatusCode() != 403) {
		return errors.New("perlindungan akses R2 belum dapat diverifikasi")
	}
	return nil
}

func (s *R2Store) Get(ctx context.Context, key string) (Object, error) {
	if !validObjectKey(s.namespace, key) {
		return Object{}, errors.New("object backup di luar namespace")
	}
	r, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), SSECustomerAlgorithm: aws.String("AES256"), SSECustomerKey: aws.String(s.key), SSECustomerKeyMD5: aws.String(s.keyMD5)})
	if err != nil {
		return Object{}, err
	}
	return Object{Body: r.Body, Size: aws.ToInt64(r.ContentLength)}, nil
}

func (s *R2Store) Delete(ctx context.Context, key string) error {
	if !validObjectKey(s.namespace, key) {
		return errors.New("object backup di luar namespace")
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	return err
}
