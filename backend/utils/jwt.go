package utils

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const AccessTokenType = "access"

type AccessClaims struct {
	Scope     string `json:"scope"`
	ClientID  string `json:"client_id"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

func jwtSecret() ([]byte, error) {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if len(secret) < 32 {
		return nil, errors.New("JWT_SECRET must contain at least 32 characters")
	}
	return []byte(secret), nil
}

func ValidateJWTConfiguration() error {
	secret, err := jwtSecret()
	if err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
		lower := strings.ToLower(string(secret))
		if strings.Contains(lower, "generate") || strings.Contains(lower, "change-me") || strings.Contains(lower, "example") {
			return errors.New("JWT_SECRET must not use an example or placeholder value in production")
		}
	}
	return nil
}

func IssuerURL() string {
	return strings.TrimRight(strings.TrimSpace(os.Getenv("BACKEND_PUBLIC_URL")), "/")
}

func GenerateAccessToken(userID, clientID, scope, jti string, duration time.Duration) (string, error) {
	secret, err := jwtSecret()
	if err != nil {
		return "", err
	}

	now := time.Now().UTC()
	claims := AccessClaims{
		Scope:     NormalizeScope(scope),
		ClientID:  clientID,
		TokenType: AccessTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    IssuerURL(),
			Subject:   userID,
			Audience:  jwt.ClaimStrings{clientID},
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        jti,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

func ValidateAccessToken(tokenString string) (*AccessClaims, error) {
	secret, err := jwtSecret()
	if err != nil {
		return nil, err
	}

	claims := &AccessClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return secret, nil
	}, jwt.WithIssuer(IssuerURL()), jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		return nil, errors.New("invalid access token")
	}
	if claims.TokenType != AccessTokenType || claims.Subject == "" || claims.ID == "" || claims.ClientID == "" {
		return nil, errors.New("invalid access token claims")
	}
	audienceValid := false
	for _, audience := range claims.Audience {
		if audience == claims.ClientID {
			audienceValid = true
			break
		}
	}
	if !audienceValid {
		return nil, errors.New("invalid access token audience")
	}
	return claims, nil
}
