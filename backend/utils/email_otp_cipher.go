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

const emailOTPEncryptionDomain = "pelajarnu-magetan-id/email-otp-outbox/v1"

func emailOTPEncryptionKey() ([]byte, error) {
	baseKey, err := clientSecretEncryptionKey()
	if err != nil {
		return nil, err
	}
	derived := sha256.Sum256(append([]byte(emailOTPEncryptionDomain+"\x00"), baseKey...))
	return derived[:], nil
}

// ValidateEmailOTPEncryptionConfiguration memastikan worker dapat membuka
// ciphertext setelah restart menggunakan kunci deployment yang persisten.
func ValidateEmailOTPEncryptionConfiguration() error {
	_, err := emailOTPEncryptionKey()
	return err
}

// EncryptEmailOTP mengenkripsi OTP dengan ID outbox sebagai authenticated
// additional data agar ciphertext tidak dapat dipindahkan ke pekerjaan lain.
func EncryptEmailOTP(outboxID, plaintext string) (string, error) {
	if strings.TrimSpace(outboxID) == "" || plaintext == "" {
		return "", errors.New("outbox ID and OTP are required")
	}
	key, err := emailOTPEncryptionKey()
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

func DecryptEmailOTP(outboxID, encoded string) (string, error) {
	if strings.TrimSpace(outboxID) == "" || !strings.HasPrefix(encoded, "v1:") {
		return "", errors.New("unsupported email OTP ciphertext")
	}
	key, err := emailOTPEncryptionKey()
	if err != nil {
		return "", err
	}
	value, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(encoded, "v1:"))
	if err != nil {
		return "", errors.New("invalid email OTP ciphertext")
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
		return "", errors.New("invalid email OTP ciphertext")
	}
	nonce, ciphertext := value[:gcm.NonceSize()], value[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(outboxID))
	if err != nil {
		return "", errors.New("email OTP ciphertext authentication failed")
	}
	return string(plaintext), nil
}
