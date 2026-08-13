package provisioning

import (
	"crypto/hmac"
	"encoding/json"
	"regexp"
	"testing"
	"time"
)

func TestSignIsStableAndBoundToTimestampAndBody(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	first := Sign(secret, "100", []byte(`{"id":"one"}`))
	second := Sign(secret, "100", []byte(`{"id":"one"}`))
	if !hmac.Equal([]byte(first), []byte(second)) {
		t.Fatal("signature should be stable")
	}
	if first == Sign(secret, "101", []byte(`{"id":"one"}`)) || first == Sign(secret, "100", []byte(`{"id":"two"}`)) {
		t.Fatal("signature must bind timestamp and exact body")
	}
}

func TestConfigureProductionRejectsInsecureTarget(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("BACKEND_PUBLIC_URL", "https://sso.example.test")
	t.Setenv("PROVISIONING_TARGETS_JSON", `{"client":{"url":"http://laci.example.test/internal/provisioning","secret":"0123456789abcdef0123456789abcdef"}}`)
	if err := Configure(); err == nil {
		t.Fatal("production must reject non-HTTPS provisioning target")
	}
}

func TestConfigureAllowsDisabledAndValidTarget(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("BACKEND_PUBLIC_URL", "https://sso.example.test")
	t.Setenv("PROVISIONING_TARGETS_JSON", `{}`)
	if err := Configure(); err != nil {
		t.Fatal(err)
	}
	if HasTarget("client") {
		t.Fatal("disabled config should have no target")
	}
	t.Setenv("PROVISIONING_TARGETS_JSON", `{"client":{"url":"https://laci.example.test/internal/provisioning","secret":"0123456789abcdef0123456789abcdef"}}`)
	if err := Configure(); err != nil {
		t.Fatal(err)
	}
	if !HasTarget("client") {
		t.Fatal("configured client target not loaded")
	}
}

func TestConfigureValidatesConcurrency(t *testing.T) {
	t.Setenv("PROVISIONING_TARGETS_JSON", `{}`)
	t.Setenv("PROVISIONING_CONCURRENCY", "17")
	if err := Configure(); err == nil {
		t.Fatal("concurrency above the safe bound must be rejected")
	}
	t.Setenv("PROVISIONING_CONCURRENCY", "4")
	if err := Configure(); err != nil {
		t.Fatal(err)
	}
}

func TestRetryDelayIsBounded(t *testing.T) {
	if got := retryDelay(1); got != 5*time.Second {
		t.Fatalf("unexpected first delay: %s", got)
	}
	if got := retryDelay(100); got != 1280*time.Second {
		t.Fatalf("unexpected capped delay: %s", got)
	}
}

func TestRandomUUID(t *testing.T) {
	first, err := randomUUID()
	if err != nil {
		t.Fatal(err)
	}
	second, err := randomUUID()
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("UUIDs must be unique")
	}
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(first) {
		t.Fatalf("invalid UUID v4: %s", first)
	}
}

func TestCloudEventUsesStandardDataEnvelope(t *testing.T) {
	payload := eventPayload{SpecVersion: "1.0", ID: "event", Type: EventAssigned,
		Source: "https://sso.example.test", Subject: "user", Time: time.Unix(0, 0).UTC(),
		DataContentType: "application/json", Data: eventData{Audience: "client", User: eventUser{Subject: "user"}}}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded["data"].(map[string]any); !ok {
		t.Fatal("CloudEvent data envelope missing")
	}
	if _, leaked := decoded["user"]; leaked {
		t.Fatal("extension data must not be placed at the CloudEvent root")
	}
}
