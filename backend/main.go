package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"sso-backend/controllers"
	"sso-backend/database"
	"sso-backend/provisioning"
	"sso-backend/utils"
)

func CORSMiddleware() gin.HandlerFunc {
	allowedOrigins := make(map[string]bool)
	for _, origin := range strings.Split(os.Getenv("BACKEND_CORS_ALLOWED_ORIGINS"), ",") {
		allowedOrigins[strings.TrimSpace(origin)] = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && !allowedOrigins[origin] {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Vary", "Origin")
		}
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PATCH, DELETE")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func MaxRequestBody(bytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, bytes)
		}
		c.Next()
	}
}

func trustedProxiesFromEnv() ([]string, error) {
	raw := strings.TrimSpace(os.Getenv("BACKEND_TRUSTED_PROXIES"))
	if raw == "" {
		return nil, nil
	}
	items := strings.Split(raw, ",")
	proxies := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" || value == "0.0.0.0/0" || value == "::/0" {
			return nil, fmt.Errorf("BACKEND_TRUSTED_PROXIES contains an unsafe value")
		}
		if net.ParseIP(value) == nil {
			if _, _, err := net.ParseCIDR(value); err != nil {
				return nil, fmt.Errorf("BACKEND_TRUSTED_PROXIES contains invalid IP/CIDR %q", value)
			}
		}
		proxies = append(proxies, value)
	}
	return proxies, nil
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, relying on environment variables")
	}
	if err := utils.ValidateRuntimeConfiguration(); err != nil {
		log.Fatal(err)
	}
	if err := utils.ValidateJWTConfiguration(); err != nil {
		log.Fatal(err)
	}
	if err := utils.ValidateClientSecretEncryptionConfiguration(); err != nil {
		log.Fatal(err)
	}
	if err := utils.ValidateOIDCConfiguration(); err != nil {
		log.Fatal(err)
	}
	if err := provisioning.Configure(); err != nil {
		log.Fatal(err)
	}
	trustedProxies, err := trustedProxiesFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	if strings.EqualFold(os.Getenv("APP_ENV"), "production") {
		gin.SetMode(gin.ReleaseMode)
		if !strings.HasPrefix(utils.IssuerURL(), "https://") {
			log.Fatal("BACKEND_PUBLIC_URL must use HTTPS in production")
		}
		frontendURL := strings.TrimSpace(os.Getenv("FRONTEND_PUBLIC_URL"))
		if !strings.HasPrefix(frontendURL, "https://") {
			log.Fatal("FRONTEND_PUBLIC_URL must use HTTPS in production")
		}
		for _, origin := range strings.Split(os.Getenv("BACKEND_CORS_ALLOWED_ORIGINS"), ",") {
			if !strings.HasPrefix(strings.TrimSpace(origin), "https://") {
				log.Fatal("every BACKEND_CORS_ALLOWED_ORIGINS value must use HTTPS in production")
			}
		}
	}

	appCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	database.Connect()
	if err := provisioning.ReconcileExistingAssignments(database.DB); err != nil {
		log.Fatal(err)
	}
	provisioning.Start(appCtx, database.DB)

	r := gin.Default()
	if err := r.SetTrustedProxies(trustedProxies); err != nil {
		log.Fatal(err)
	}
	r.Use(CORSMiddleware())
	r.Use(MaxRequestBody(1 << 20)) // Batas 1 MiB cukup untuk seluruh payload API.

	appName := strings.TrimSpace(os.Getenv("APP_NAME"))
	r.GET("/api/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "Backend " + appName + " siap menerima request."})
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "OK",
			"message": "SSO " + appName + " berjalan dengan baik.",
		})
	})
	r.GET("/ready", func(c *gin.Context) {
		sqlDB, err := database.DB.DB()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "NOT_READY"})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := sqlDB.PingContext(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "NOT_READY"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "READY"})
	})

	// Auth Endpoints
	r.POST("/api/auth/register", controllers.RateLimit(10, time.Minute), controllers.Register)
	r.POST("/api/auth/login", controllers.RateLimit(10, time.Minute), controllers.Login)
	r.POST("/api/auth/verify-email", controllers.RateLimit(10, 15*time.Minute), controllers.VerifyEmail)
	r.POST("/api/auth/resend-verification", controllers.RateLimit(5, 15*time.Minute), controllers.ResendVerification)
	// Pemeriksaan status sesi bersifat aman untuk halaman publik: pengguna anonim
	// menerima {"user": null}, sementara seluruh endpoint dashboard tetap dijaga
	// RequireSession.
	r.GET("/api/auth/session", controllers.GetSession)
	r.POST("/api/auth/logout", controllers.Logout)

	// OAuth2 Endpoints
	r.GET("/.well-known/oauth-authorization-server", controllers.AuthorizationServerMetadata)
	r.GET("/.well-known/openid-configuration", controllers.OpenIDConfiguration)
	r.GET("/oauth/jwks", controllers.OIDCJWKS)
	r.GET("/api/oauth/client-info", controllers.GetOAuthClientInfo)
	r.GET("/oauth/authorize", controllers.OAuthAuthorizationEndpoint)
	r.POST("/oauth/authorize", controllers.RequireSession, controllers.OAuthAuthorize)
	r.POST("/oauth/token", controllers.RateLimit(30, time.Minute), controllers.OAuthToken)
	r.POST("/oauth/revoke", controllers.RateLimit(30, time.Minute), controllers.OAuthRevoke)

	// User Endpoints
	r.GET("/v1/user/me", controllers.RequireAuthToken, controllers.GetMe)
	r.PATCH("/api/profile", controllers.RequireSession, controllers.UpdateProfile)
	r.POST("/api/profile/password", controllers.RequireSession, controllers.ChangePassword)
	// Aplikasi SSO yang masih memiliki grant aktif milik pengguna.
	r.GET("/api/connections", controllers.RequireSession, controllers.GetApplicationConnections)
	r.DELETE("/api/connections/:id", controllers.RequireSession, controllers.RevokeApplicationConnection)

	// Setiap anggota terverifikasi dapat mengelola OAuth client miliknya sendiri.
	// Semua controller client juga memverifikasi owner_id pada setiap operasi.
	r.POST("/api/clients", controllers.RequireSession, controllers.CreateClient)
	r.GET("/api/clients", controllers.RequireSession, controllers.GetClients)
	r.GET("/api/clients/:id", controllers.RequireSession, controllers.GetClient)
	r.PATCH("/api/clients/:id", controllers.RequireSession, controllers.UpdateClient)
	r.GET("/api/clients/:id/secret", controllers.RequireSession, controllers.GetClientSecret)
	r.POST("/api/clients/:id/secret/regenerate", controllers.RequireSession, controllers.RegenerateClientSecret)
	r.GET("/api/clients/:id/assignments", controllers.RequireSession, controllers.GetClientAssignments)
	r.POST("/api/clients/:id/assignment-lookup", controllers.RequireSession, controllers.LookupClientAssignmentUser)
	r.POST("/api/clients/:id/assignments", controllers.RequireSession, controllers.AssignClientUser)
	r.DELETE("/api/clients/:id/assignments/:userId", controllers.RequireSession, controllers.DeleteClientAssignment)
	r.DELETE("/api/clients/:id", controllers.RequireSession, controllers.DeleteClient)

	// RBAC administration.
	r.GET("/api/admin/users", controllers.RequireSession, controllers.RequireRole("super_admin"), controllers.AdminGetUsers)
	r.GET("/api/admin/audit-logs", controllers.RequireSession, controllers.RequireRole("super_admin"), controllers.AdminGetAuditLogs)
	r.GET("/api/admin/provisioning", controllers.RequireSession, controllers.RequireRole("super_admin"), controllers.AdminProvisioningStatus)
	r.POST("/api/admin/provisioning/:id/retry", controllers.RequireSession, controllers.RequireRole("super_admin"), controllers.AdminRetryProvisioningEvent)
	r.PATCH("/api/admin/users/:id/role", controllers.RequireSession, controllers.RequireRole("super_admin"), controllers.AdminUpdateRole)
	r.PATCH("/api/admin/users/:id/status", controllers.RequireSession, controllers.RequireRole("super_admin"), controllers.AdminUpdateStatus)
	r.DELETE("/api/admin/users/:id", controllers.RequireSession, controllers.RequireRole("super_admin"), controllers.AdminDeleteUser)

	port := strings.TrimSpace(os.Getenv("BACKEND_PORT"))

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.ListenAndServe()
	}()
	select {
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	case <-appCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server forced shutdown: %v", err)
		}
	}
}
