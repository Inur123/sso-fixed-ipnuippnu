package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const clientSecretKeyDomain = "ipnu-ippnu-id/client-secret-encryption/v1"

// ValidateClientSecretEncryptionConfiguration memastikan deployment production
// selalu menggunakan kunci AES terpisah. Development dan test boleh memakai
// kunci deterministik yang diturunkan secara domain-separated dari JWT_SECRET.
func ValidateClientSecretEncryptionConfiguration() error {
	_, err := clientSecretEncryptionKey()
	return err
}

func clientSecretEncryptionKey() ([]byte, error) {
	configured := strings.TrimSpace(os.Getenv("CLIENT_SECRET_ENCRYPTION_KEY"))
	if configured != "" {
		key, err := base64.StdEncoding.DecodeString(configured)
		if err != nil {
			return nil, errors.New("CLIENT_SECRET_ENCRYPTION_KEY must be valid standard base64")
		}
		if len(key) != 32 {
			return nil, errors.New("CLIENT_SECRET_ENCRYPTION_KEY must decode to exactly 32 bytes")
		}
		return key, nil
	}

	if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
		return nil, errors.New("CLIENT_SECRET_ENCRYPTION_KEY is required in production")
	}
	jwtKey, err := jwtSecret()
	if err != nil {
		return nil, fmt.Errorf("client secret encryption fallback: %w", err)
	}
	derived := sha256.Sum256(append([]byte(clientSecretKeyDomain+"\x00"), jwtKey...))
	return derived[:], nil
}

// EncryptClientSecret menghasilkan blob versioned base64(nonce|ciphertext).
// clientID digunakan sebagai authenticated additional data agar ciphertext
// tidak dapat dipindahkan ke record aplikasi lain.
func EncryptClientSecret(clientID, plaintext string) (string, error) {
	if strings.TrimSpace(clientID) == "" || plaintext == "" {
		return "", errors.New("client ID and secret are required")
	}
	key, err := clientSecretEncryptionKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, nonce, []byte(plaintext), []byte(clientID))
	value := append(nonce, sealed...)
	return "v1:" + base64.RawURLEncoding.EncodeToString(value), nil
}

func DecryptClientSecret(clientID, encoded string) (string, error) {
	if strings.TrimSpace(clientID) == "" || !strings.HasPrefix(encoded, "v1:") {
		return "", errors.New("unsupported client secret ciphertext")
	}
	key, err := clientSecretEncryptionKey()
	if err != nil {
		return "", err
	}
	value, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(encoded, "v1:"))
	if err != nil {
		return "", errors.New("invalid client secret ciphertext")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(value) < gcm.NonceSize()+gcm.Overhead() {
		return "", errors.New("invalid client secret ciphertext")
	}
	nonce, ciphertext := value[:gcm.NonceSize()], value[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(clientID))
	if err != nil {
		return "", errors.New("client secret ciphertext authentication failed")
	}
	return string(plaintext), nil
}
