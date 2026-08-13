package utils

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
)

const EmailOTPDigits = 6

// GenerateEmailOTP menggunakan crypto/rand dan mempertahankan leading zero.
func GenerateEmailOTP() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", value.Int64()), nil
}

func otpHashSecret() ([]byte, error) {
	secret := strings.TrimSpace(os.Getenv("OTP_HASH_SECRET"))
	if secret == "" {
		// JWT_SECRET sudah diwajibkan pada startup dan aman dipakai sebagai
		// fallback HMAC. Deployment dapat memisahkan kunci lewat OTP_HASH_SECRET.
		secret = strings.TrimSpace(os.Getenv("JWT_SECRET"))
	}
	if len(secret) < 32 {
		return nil, errors.New("OTP_HASH_SECRET or JWT_SECRET must contain at least 32 characters")
	}
	return []byte(secret), nil
}

// HashEmailOTP memakai HMAC agar ruang OTP enam digit tidak dapat dibongkar
// secara offline hanya dari isi database.
func HashEmailOTP(userID, code string) (string, error) {
	secret, err := otpHashSecret()
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(userID))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func VerifyEmailOTP(userID, code, expectedHash string) bool {
	actual, err := HashEmailOTP(userID, code)
	if err != nil || len(actual) != len(expectedHash) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expectedHash)) == 1
}
