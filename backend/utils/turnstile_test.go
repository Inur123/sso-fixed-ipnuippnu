package utils

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const passingTurnstileTestSecret = "1x0000000000000000000000000000000AA"

func TestVerifyTurnstile(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("TURNSTILE_SECRET_KEY", passingTurnstileTestSecret)
	t.Setenv("TURNSTILE_ALLOWED_HOSTNAMES", "localhost")

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", request.Method)
		}
		if err := request.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if request.Form.Get("secret") != passingTurnstileTestSecret || request.Form.Get("response") != "valid-token" {
			t.Fatalf("unexpected Siteverify payload: %v", request.Form)
		}
		if request.Form.Get("remoteip") != "127.0.0.1" {
			t.Fatalf("expected remote IP to be forwarded")
		}
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{"success":true,"hostname":"localhost","action":"register"}`)
	}))
	defer server.Close()

	previousURL := turnstileVerifyURL
	turnstileVerifyURL = server.URL
	t.Cleanup(func() { turnstileVerifyURL = previousURL })

	valid, err := VerifyTurnstile(context.Background(), "valid-token", "127.0.0.1", "register")
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if !valid {
		t.Fatal("expected token to be valid")
	}
}

func TestVerifyTurnstileRejectsMismatchedActionAndHostname(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("TURNSTILE_SECRET_KEY", "production-secret")
	t.Setenv("TURNSTILE_ALLOWED_HOSTNAMES", "pelajarnumagetan.id")

	tests := []struct {
		name     string
		response string
	}{
		{name: "action", response: `{"success":true,"hostname":"pelajarnumagetan.id","action":"login"}`},
		{name: "hostname", response: `{"success":true,"hostname":"attacker.example","action":"register"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("Content-Type", "application/json")
				fmt.Fprint(writer, test.response)
			}))
			defer server.Close()

			previousURL := turnstileVerifyURL
			turnstileVerifyURL = server.URL
			t.Cleanup(func() { turnstileVerifyURL = previousURL })

			valid, err := VerifyTurnstile(context.Background(), "valid-token", "", "register")
			if err != nil {
				t.Fatalf("verify token: %v", err)
			}
			if valid {
				t.Fatal("expected token to be rejected")
			}
		})
	}
}

func TestVerifyTurnstileRejectsOversizedTokenWithoutRequest(t *testing.T) {
	t.Setenv("TURNSTILE_SECRET_KEY", passingTurnstileTestSecret)
	t.Setenv("TURNSTILE_ALLOWED_HOSTNAMES", "localhost")

	invalidToken := strings.Repeat("x", 2049)
	valid, err := VerifyTurnstile(context.Background(), invalidToken, "", "register")
	if err != nil {
		t.Fatalf("verify oversized token: %v", err)
	}
	if valid {
		t.Fatal("expected oversized token to be rejected")
	}
}

func TestValidateTurnstileConfigurationRejectsTestSecretInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("TURNSTILE_SECRET_KEY", passingTurnstileTestSecret)
	t.Setenv("TURNSTILE_ALLOWED_HOSTNAMES", "pelajarnumagetan.id")

	if err := ValidateTurnstileConfiguration(); err == nil {
		t.Fatal("production must reject Cloudflare Turnstile test secrets")
	}
}

func TestTurnstileRequestUsesFormEncoding(t *testing.T) {
	values := url.Values{"response": {"token with spaces"}}
	if !strings.Contains(values.Encode(), "response=token+with+spaces") {
		t.Fatal("expected URL-encoded form payload")
	}
}
