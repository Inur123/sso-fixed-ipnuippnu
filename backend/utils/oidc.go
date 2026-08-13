package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const oidcMinimumRSAKeyBits = 2048

type IDTokenClaims struct {
	Nonce         string `json:"nonce,omitempty"`
	AuthTime      int64  `json:"auth_time"`
	Name          string `json:"name,omitempty"`
	Email         string `json:"email,omitempty"`
	EmailVerified bool   `json:"email_verified,omitempty"`
	jwt.RegisteredClaims
}

type JSONWebKey struct {
	KeyType   string `json:"kty"`
	Use       string `json:"use"`
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
	Modulus   string `json:"n"`
	Exponent  string `json:"e"`
}

type JSONWebKeySet struct {
	Keys []JSONWebKey `json:"keys"`
}

var oidcSigningKeyCache struct {
	sync.Mutex
	identity string
	key      *rsa.PrivateKey
	kid      string
	err      error
}

// ValidateOIDCConfiguration memuat dan memvalidasi signing key lebih awal agar
// konfigurasi produksi yang tidak aman menggagalkan startup, bukan token request.
func ValidateOIDCConfiguration() error {
	_, _, err := oidcSigningKey()
	return err
}

// GenerateIDToken memakai satu signing key milik issuer. Client memverifikasi
// RS256 signature melalui JWKS tanpa pernah menerima material private key.
func GenerateIDToken(userID, name, email string, emailVerified bool, clientID, nonce, scope string, authTime time.Time) (string, error) {
	privateKey, kid, err := oidcSigningKey()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	claims := IDTokenClaims{
		Nonce:    nonce,
		AuthTime: authTime.UTC().Unix(),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    IssuerURL(),
			Subject:   userID,
			Audience:  jwt.ClaimStrings{clientID},
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	if ScopeAllowed("profile", scope) {
		claims.Name = name
	}
	if ScopeAllowed("email", scope) {
		claims.Email = email
		claims.EmailVerified = emailVerified
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	return token.SignedString(privateKey)
}

func OIDCJWKS() (JSONWebKeySet, error) {
	privateKey, kid, err := oidcSigningKey()
	if err != nil {
		return JSONWebKeySet{}, err
	}
	publicKey := &privateKey.PublicKey
	return JSONWebKeySet{Keys: []JSONWebKey{{
		KeyType:   "RSA",
		Use:       "sig",
		Algorithm: "RS256",
		KeyID:     kid,
		Modulus:   base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes()),
		Exponent:  encodeJWKInteger(int64(publicKey.E)),
	}}}, nil
}

func oidcSigningKey() (*rsa.PrivateKey, string, error) {
	path := strings.TrimSpace(os.Getenv("OIDC_PRIVATE_KEY_PATH"))
	production := strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production")
	identity := fmt.Sprintf("production=%t;path=%s", production, path)

	oidcSigningKeyCache.Lock()
	defer oidcSigningKeyCache.Unlock()
	if oidcSigningKeyCache.identity == identity && (oidcSigningKeyCache.key != nil || oidcSigningKeyCache.err != nil) {
		return oidcSigningKeyCache.key, oidcSigningKeyCache.kid, oidcSigningKeyCache.err
	}

	var key *rsa.PrivateKey
	var err error
	if path == "" {
		if production {
			err = errors.New("OIDC_PRIVATE_KEY_PATH is required in production")
		} else {
			// Development convenience only. Restarting invalidates outstanding ID
			// token signatures; production must use a persistent PEM file.
			key, err = rsa.GenerateKey(rand.Reader, oidcMinimumRSAKeyBits)
		}
	} else {
		key, err = readRSAPrivateKey(path)
	}
	if err == nil {
		err = validateRSAPrivateKey(key)
	}
	kid := ""
	if err == nil {
		kid, err = rsaJWKThumbprint(&key.PublicKey)
	}

	oidcSigningKeyCache.identity = identity
	oidcSigningKeyCache.key = key
	oidcSigningKeyCache.kid = kid
	oidcSigningKeyCache.err = err
	return key, kid, err
}

func readRSAPrivateKey(path string) (*rsa.PrivateKey, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read OIDC private key: %w", err)
	}
	block, _ := pem.Decode(contents)
	if block == nil {
		return nil, errors.New("OIDC private key must be PEM encoded")
	}
	if x509.IsEncryptedPEMBlock(block) {
		return nil, errors.New("encrypted OIDC private keys are not supported")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("OIDC private key must be RSA PKCS#1 or PKCS#8 PEM")
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("OIDC private key must use RSA")
	}
	return key, nil
}

func validateRSAPrivateKey(key *rsa.PrivateKey) error {
	if key == nil || key.N == nil {
		return errors.New("OIDC RSA private key is missing")
	}
	if key.N.BitLen() < oidcMinimumRSAKeyBits {
		return fmt.Errorf("OIDC RSA private key must be at least %d bits", oidcMinimumRSAKeyBits)
	}
	if err := key.Validate(); err != nil {
		return fmt.Errorf("OIDC RSA private key is invalid: %w", err)
	}
	return nil
}

// rsaJWKThumbprint implements the canonical RSA JWK thumbprint from RFC 7638.
func rsaJWKThumbprint(publicKey *rsa.PublicKey) (string, error) {
	canonical, err := json.Marshal(struct {
		Exponent string `json:"e"`
		KeyType  string `json:"kty"`
		Modulus  string `json:"n"`
	}{
		Exponent: encodeJWKInteger(int64(publicKey.E)),
		KeyType:  "RSA",
		Modulus:  base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes()),
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

func encodeJWKInteger(value int64) string {
	return base64.RawURLEncoding.EncodeToString(big.NewInt(value).Bytes())
}

// DecodeJWKPublicKey tersedia untuk validasi interoperabilitas dan konsumen
// internal; relying party normalnya memakai library OIDC/JWT mereka sendiri.
func DecodeJWKPublicKey(key JSONWebKey) (*rsa.PublicKey, error) {
	modulus, err := base64.RawURLEncoding.DecodeString(key.Modulus)
	if err != nil {
		return nil, fmt.Errorf("decode JWK modulus: %w", err)
	}
	exponent, err := base64.RawURLEncoding.DecodeString(key.Exponent)
	if err != nil {
		return nil, fmt.Errorf("decode JWK exponent: %w", err)
	}
	if len(modulus) == 0 || len(exponent) == 0 {
		return nil, errors.New("JWK RSA parameters are empty")
	}
	exponentInt := new(big.Int).SetBytes(exponent)
	if !exponentInt.IsInt64() || exponentInt.Sign() <= 0 {
		return nil, errors.New("JWK exponent is invalid")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(modulus), E: int(exponentInt.Int64())}, nil
}
