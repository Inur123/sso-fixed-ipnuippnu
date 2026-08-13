package utils

import (
	"fmt"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// ValidateRuntimeConfiguration memastikan nilai deployment dibaca dari
// environment. Source code tidak menyediakan URL, port, email, atau identitas
// deployment pengganti yang berpotensi ikut terbawa ke production.
func ValidateRuntimeConfiguration() error {
	required := []string{
		"APP_ENV",
		"APP_NAME",
		"BACKEND_PORT",
		"BACKEND_PUBLIC_URL",
		"FRONTEND_PUBLIC_URL",
		"BACKEND_CORS_ALLOWED_ORIGINS",
		"SESSION_COOKIE_NAME",
		"DB_SSLMODE",
		"SUPER_ADMIN_EMAIL",
		"MAIL_MAILER",
		"MAIL_HOST",
		"MAIL_PORT",
		"MAIL_USERNAME",
		"MAIL_PASSWORD",
		"MAIL_ENCRYPTION",
		"MAIL_FROM_ADDRESS",
		"MAIL_FROM_NAME",
		"MAIL_OTP_TTL_MINUTES",
		"OTP_HASH_SECRET",
	}
	for _, name := range required {
		if strings.TrimSpace(os.Getenv(name)) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}

	port, err := strconv.Atoi(strings.TrimSpace(os.Getenv("BACKEND_PORT")))
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("BACKEND_PORT must be between 1 and 65535")
	}
	if _, err := strconv.Atoi(strings.TrimSpace(os.Getenv("MAIL_PORT"))); err != nil {
		return fmt.Errorf("MAIL_PORT must be numeric")
	}
	mailOTPTTL, err := strconv.Atoi(strings.TrimSpace(os.Getenv("MAIL_OTP_TTL_MINUTES")))
	if err != nil || mailOTPTTL < 5 || mailOTPTTL > 30 {
		return fmt.Errorf("MAIL_OTP_TTL_MINUTES must be between 5 and 30")
	}

	for _, name := range []string{"BACKEND_PUBLIC_URL", "FRONTEND_PUBLIC_URL"} {
		if err := validatePublicBaseURL(name, os.Getenv(name)); err != nil {
			return err
		}
	}
	backendURL, _ := url.Parse(strings.TrimSpace(os.Getenv("BACKEND_PUBLIC_URL")))
	frontendURL, _ := url.Parse(strings.TrimSpace(os.Getenv("FRONTEND_PUBLIC_URL")))
	cookieDomain := strings.TrimSpace(os.Getenv("SESSION_COOKIE_DOMAIN"))
	if backendURL.Hostname() != frontendURL.Hostname() && cookieDomain == "" {
		return fmt.Errorf("SESSION_COOKIE_DOMAIN is required when backend and frontend use different hosts")
	}
	if cookieDomain != "" {
		normalizedDomain := strings.TrimPrefix(strings.ToLower(cookieDomain), ".")
		for _, host := range []string{backendURL.Hostname(), frontendURL.Hostname()} {
			lowerHost := strings.ToLower(host)
			if lowerHost != normalizedDomain && !strings.HasSuffix(lowerHost, "."+normalizedDomain) {
				return fmt.Errorf("SESSION_COOKIE_DOMAIN must cover backend and frontend hosts")
			}
		}
	}
	for _, rawOrigin := range strings.Split(os.Getenv("BACKEND_CORS_ALLOWED_ORIGINS"), ",") {
		if err := validatePublicBaseURL("BACKEND_CORS_ALLOWED_ORIGINS", rawOrigin); err != nil {
			return err
		}
	}
	for _, name := range []string{"SUPER_ADMIN_EMAIL", "MAIL_USERNAME", "MAIL_FROM_ADDRESS"} {
		if _, err := mail.ParseAddress(strings.TrimSpace(os.Getenv(name))); err != nil {
			return fmt.Errorf("%s must contain a valid email address", name)
		}
	}
	if _, err := loadSMTPConfig(); err != nil {
		return err
	}
	return nil
}

func validatePublicBaseURL(name, value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s must contain an absolute URL", name)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%s must use HTTP or HTTPS", name)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return fmt.Errorf("%s must contain an origin without path, query, or fragment", name)
	}
	return nil
}
