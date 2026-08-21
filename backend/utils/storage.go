package utils

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	s3Client     *s3.Client
	r2BucketName string
	r2PublicURL  string
)

// InitR2Client membuat S3-compatible client yang mengarah ke Cloudflare R2.
// Dipanggil sekali saat startup dari main.go.
func InitR2Client() {
	accountID := strings.TrimSpace(os.Getenv("R2_ACCOUNT_ID"))
	accessKeyID := strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID"))
	secretAccessKey := strings.TrimSpace(os.Getenv("R2_SECRET_ACCESS_KEY"))
	r2BucketName = strings.TrimSpace(os.Getenv("R2_BUCKET_NAME"))
	r2PublicURL = strings.TrimRight(strings.TrimSpace(os.Getenv("R2_PUBLIC_URL")), "/")

	for _, pair := range []struct{ name, value string }{
		{"R2_ACCOUNT_ID", accountID},
		{"R2_ACCESS_KEY_ID", accessKeyID},
		{"R2_SECRET_ACCESS_KEY", secretAccessKey},
		{"R2_BUCKET_NAME", r2BucketName},
		{"R2_PUBLIC_URL", r2PublicURL},
	} {
		if pair.value == "" {
			log.Fatalf("%s is required for R2 storage", pair.name)
		}
	}

	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)

	s3Client = s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: aws.String(endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
	})

	log.Printf("R2 storage client initialised (bucket: %s)", r2BucketName)
}

// UploadToR2 mengunggah data ke R2 dan mengembalikan URL publik.
func UploadToR2(ctx context.Context, key, contentType string, body io.Reader) (string, error) {
	_, err := s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(r2BucketName),
		Key:          aws.String(key),
		Body:         body,
		ContentType:  aws.String(contentType),
		CacheControl: aws.String("public, max-age=31536000, immutable"),
	})
	if err != nil {
		return "", fmt.Errorf("r2 upload failed: %w", err)
	}
	return r2PublicURL + "/" + key, nil
}

// DeleteFromR2 menghapus objek dari R2 berdasarkan key.
func DeleteFromR2(ctx context.Context, key string) error {
	_, err := s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(r2BucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("r2 delete failed: %w", err)
	}
	return nil
}

// R2KeyFromPublicURL mengekstrak object key dari URL publik avatar.
// Mengembalikan string kosong jika URL bukan milik R2 bucket ini.
func R2KeyFromPublicURL(publicURL string) string {
	prefix := r2PublicURL + "/"
	if !strings.HasPrefix(publicURL, prefix) {
		return ""
	}
	return strings.TrimPrefix(publicURL, prefix)
}
