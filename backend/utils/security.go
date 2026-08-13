package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
)

func RandomToken(byteLength int) (string, error) {
	if byteLength < 32 {
		return "", errors.New("token entropy must be at least 32 bytes")
	}
	value := make([]byte, byteLength)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func VerifyPKCES256(verifier, challenge string) bool {
	if len(verifier) < 43 || len(verifier) > 128 || challenge == "" {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	actual := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(actual), []byte(challenge)) == 1
}

func NormalizeScope(scope string) string {
	seen := make(map[string]struct{})
	for _, item := range strings.Fields(scope) {
		seen[item] = struct{}{}
	}
	items := make([]string, 0, len(seen))
	for item := range seen {
		items = append(items, item)
	}
	sort.Strings(items)
	return strings.Join(items, " ")
}

func ScopeAllowed(requested, allowed string) bool {
	allowedSet := make(map[string]struct{})
	for _, item := range strings.Fields(allowed) {
		allowedSet[item] = struct{}{}
	}
	for _, item := range strings.Fields(requested) {
		if _, ok := allowedSet[item]; !ok {
			return false
		}
	}
	return requested != ""
}
