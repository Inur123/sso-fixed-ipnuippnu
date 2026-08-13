package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"sso-backend/models"
)

func TestGetSessionAnonymousReturnsNull(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)

	GetSession(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Body.String(); got != "{\"user\":null}" {
		t.Fatalf("unexpected anonymous session response: %s", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control=%q", got)
	}
}

func TestAccountAccessRequiresActiveAndVerified(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name string
		user models.User
		code string
		ok   bool
	}{
		{name: "inactive", user: models.User{IsActive: false, EmailVerifiedAt: &now}, code: "account_inactive", ok: false},
		{name: "unverified", user: models.User{IsActive: true}, code: "email_unverified", ok: false},
		{name: "eligible", user: models.User{IsActive: true, EmailVerifiedAt: &now}, ok: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, code, _, denied := accountAccessError(&test.user)
			if denied == test.ok || code != test.code {
				t.Fatalf("denied=%v code=%q", denied, code)
			}
		})
	}
}

func TestEmailOTPLifetimeUsesSafeBounds(t *testing.T) {
	tests := []struct {
		value string
		want  time.Duration
	}{
		{value: "", want: 10 * time.Minute},
		{value: "abc", want: 10 * time.Minute},
		{value: "4", want: 10 * time.Minute},
		{value: "5", want: 5 * time.Minute},
		{value: "30", want: 30 * time.Minute},
		{value: "31", want: 10 * time.Minute},
	}
	for _, test := range tests {
		t.Setenv("MAIL_OTP_TTL_MINUTES", test.value)
		if got := emailOTPLifetime(); got != test.want {
			t.Fatalf("value=%q got=%s want=%s", test.value, got, test.want)
		}
	}
}

func TestRoleAfterEmailVerification(t *testing.T) {
	t.Setenv("SUPER_ADMIN_EMAIL", "Configured.Admin@Example.test")
	if got := roleAfterEmailVerification("configured.admin@example.test", models.RoleAnggota); got != models.RoleSuperAdmin {
		t.Fatalf("configured verified account got role %q", got)
	}
	if got := roleAfterEmailVerification("anggota@example.com", models.RoleAnggota); got != models.RoleAnggota {
		t.Fatalf("ordinary verified account got role %q", got)
	}
}
