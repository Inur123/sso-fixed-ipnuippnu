package utils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const defaultTurnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

var (
	turnstileHTTPClient = &http.Client{Timeout: 10 * time.Second}
	turnstileVerifyURL  = defaultTurnstileVerifyURL
)

type turnstileVerificationResponse struct {
	Success    bool     `json:"success"`
	Hostname   string   `json:"hostname"`
	Action     string   `json:"action"`
	ErrorCodes []string `json:"error-codes"`
}

// ValidateTurnstileConfiguration memastikan backend selalu memvalidasi token
// dan mencegah test key Cloudflare ikut dipakai pada deployment production.
func ValidateTurnstileConfiguration() error {
	secret := strings.TrimSpace(os.Getenv("TURNSTILE_SECRET_KEY"))
	if secret == "" {
		return errors.New("TURNSTILE_SECRET_KEY is required")
	}
	if len(turnstileAllowedHostnames()) == 0 {
		return errors.New("TURNSTILE_ALLOWED_HOSTNAMES is required")
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") && isTurnstileTestSecret(secret) {
		return errors.New("TURNSTILE_SECRET_KEY must not use a Cloudflare test key in production")
	}
	return nil
}

// VerifyTurnstile memvalidasi token sekali pakai ke Cloudflare Siteverify,
// kemudian mengikat hasilnya pada action dan hostname frontend yang diizinkan.
func VerifyTurnstile(ctx context.Context, token, remoteIP, expectedAction string) (bool, error) {
	secret := strings.TrimSpace(os.Getenv("TURNSTILE_SECRET_KEY"))
	if secret == "" {
		return false, errors.New("TURNSTILE_SECRET_KEY is not configured")
	}

	token = strings.TrimSpace(token)
	if token == "" || len(token) > 2048 {
		return false, nil
	}

	form := url.Values{
		"secret":   {secret},
		"response": {token},
	}
	if ip := strings.TrimSpace(remoteIP); ip != "" {
		form.Set("remoteip", ip)
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		turnstileVerifyURL,
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return false, fmt.Errorf("create Turnstile verification request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := turnstileHTTPClient.Do(request)
	if err != nil {
		return false, fmt.Errorf("verify Turnstile token: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return false, fmt.Errorf("Turnstile Siteverify returned HTTP %d", response.StatusCode)
	}

	var result turnstileVerificationResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(&result); err != nil {
		return false, fmt.Errorf("decode Turnstile verification response: %w", err)
	}
	if !result.Success {
		return false, nil
	}

	expectedAction = strings.TrimSpace(expectedAction)
	if expectedAction != "" && result.Action != expectedAction {
		// Test key resmi Cloudflare mengembalikan action "test". Pengecualian ini
		// hanya berlaku di non-production agar pengujian localhost tetap nyata.
		isDevelopmentTest := !strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") &&
			isTurnstileTestSecret(secret) && result.Action == "test"
		if !isDevelopmentTest {
			return false, nil
		}
	}

	_, hostnameAllowed := turnstileAllowedHostnames()[strings.ToLower(strings.TrimSpace(result.Hostname))]
	if !hostnameAllowed {
		return false, nil
	}
	return true, nil
}

func turnstileAllowedHostnames() map[string]struct{} {
	allowed := make(map[string]struct{})
	for _, rawHostname := range strings.Split(os.Getenv("TURNSTILE_ALLOWED_HOSTNAMES"), ",") {
		hostname := strings.ToLower(strings.TrimSpace(rawHostname))
		if hostname != "" {
			allowed[hostname] = struct{}{}
		}
	}
	return allowed
}

func isTurnstileTestSecret(secret string) bool {
	switch strings.TrimSpace(secret) {
	case "1x0000000000000000000000000000000AA",
		"2x0000000000000000000000000000000AA",
		"3x0000000000000000000000000000000AA":
		return true
	default:
		return false
	}
}
