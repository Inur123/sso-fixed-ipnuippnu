package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

const passwordEmailEncryptionDomain = "pelajarnu-magetan-id/password-email-outbox/v1"

func passwordEmailEncryptionKey() ([]byte, error) {
	baseKey, err := clientSecretEncryptionKey()
	if err != nil {
		return nil, err
	}
	derived := sha256.Sum256(append([]byte(passwordEmailEncryptionDomain+"\x00"), baseKey...))
	return derived[:], nil
}

// ValidatePasswordEmailEncryptionConfiguration memastikan token reset di
// antrean email tetap dapat dibuka setelah worker atau server dimulai ulang.
func ValidatePasswordEmailEncryptionConfiguration() error {
	_, err := passwordEmailEncryptionKey()
	return err
}

func EncryptPasswordEmailPayload(outboxID, plaintext string) (string, error) {
	if strings.TrimSpace(outboxID) == "" || plaintext == "" {
		return "", errors.New("outbox ID and password email payload are required")
	}
	key, err := passwordEmailEncryptionKey()
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
	sealed := gcm.Seal(nil, nonce, []byte(plaintext), []byte(outboxID))
	return "v1:" + base64.RawURLEncoding.EncodeToString(append(nonce, sealed...)), nil
}

func DecryptPasswordEmailPayload(outboxID, encoded string) (string, error) {
	if strings.TrimSpace(outboxID) == "" || !strings.HasPrefix(encoded, "v1:") {
		return "", errors.New("unsupported password email ciphertext")
	}
	key, err := passwordEmailEncryptionKey()
	if err != nil {
		return "", err
	}
	value, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(encoded, "v1:"))
	if err != nil {
		return "", errors.New("invalid password email ciphertext")
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
		return "", errors.New("invalid password email ciphertext")
	}
	nonce, ciphertext := value[:gcm.NonceSize()], value[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(outboxID))
	if err != nil {
		return "", errors.New("password email ciphertext authentication failed")
	}
	return string(plaintext), nil
}
