---
title: Go
description: Contoh integrasi Go menggunakan coreos/go-oidc dan golang.org/x/oauth2.
---

# Integrasi Go

Gunakan library OIDC untuk discovery, verifikasi signature, issuer, audience, dan expiry.

```bash
go get github.com/coreos/go-oidc/v3/oidc
go get golang.org/x/oauth2
```

```go title="sso.go"
package sso

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"os"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type LoginTransaction struct {
	State, Nonce, Verifier, ReturnTo string
}

type Server struct {
	Issuer   string
	Provider *oidc.Provider
	OAuth    oauth2.Config
	// Transactions wajib storage server-side dengan TTL dan atomic take.
	Transactions TransactionStore
	Sessions     SessionStore
}

func New(ctx context.Context) (*Server, error) {
	issuer := os.Getenv("SSO_ISSUER")
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil { return nil, err }
	endpoint := provider.Endpoint()
	endpoint.AuthStyle = oauth2.AuthStyleInParams // client_secret_post
	return &Server{
		Issuer: issuer,
		Provider: provider,
		OAuth: oauth2.Config{
			ClientID: os.Getenv("SSO_CLIENT_ID"),
			ClientSecret: os.Getenv("SSO_CLIENT_SECRET"),
			RedirectURL: os.Getenv("SSO_REDIRECT_URI"),
			Endpoint: endpoint,
			Scopes: []string{oidc.ScopeOpenID, "profile", "email"},
		},
	}, nil
}

func randomValue() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil { return "", err }
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *Server) Start(w http.ResponseWriter, r *http.Request) {
	state, e1 := randomValue()
	nonce, e2 := randomValue()
	verifier := oauth2.GenerateVerifier()
	if e1 != nil || e2 != nil { http.Error(w, "random gagal", 500); return }

	txID, err := s.Transactions.Put(r.Context(), LoginTransaction{
		State: state, Nonce: nonce, Verifier: verifier, ReturnTo: "/",
	})
	if err != nil { http.Error(w, "storage gagal", 500); return }
	http.SetCookie(w, &http.Cookie{
		Name: "sso_tx", Value: txID, Path: "/auth/sso/callback",
		MaxAge: 600, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	})

	url := s.OAuth.AuthCodeURL(
		state,
		oidc.Nonce(nonce),
		oauth2.S256ChallengeOption(verifier),
	)
	http.Redirect(w, r, url, http.StatusFound)
}
```

Untuk localhost tanpa TLS, buat flag development eksplisit bagi `Secure`; production harus selalu `true`.

## Callback

```go
func (s *Server) Callback(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("sso_tx")
	if err != nil { http.Error(w, "transaksi tidak ada", 400); return }
	http.SetCookie(w, &http.Cookie{Name: "sso_tx", Value: "", Path: "/auth/sso/callback", MaxAge: -1})
	tx, err := s.Transactions.Take(r.Context(), cookie.Value) // get-and-delete atomik
	if err != nil { http.Error(w, "transaksi kedaluwarsa", 400); return }

	if r.URL.Query().Get("state") != tx.State {
		http.Error(w, "state salah", 400); return
	}
	if r.URL.Query().Get("iss") != s.Issuer {
		http.Error(w, "issuer salah", 400); return
	}
	if code := r.URL.Query().Get("error"); code != "" {
		http.Error(w, "SSO gagal: "+code, 400); return
	}

	token, err := s.OAuth.Exchange(
		r.Context(), r.URL.Query().Get("code"), oauth2.VerifierOption(tx.Verifier),
	)
	if err != nil { http.Error(w, "token exchange gagal", 400); return }
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok { http.Error(w, "ID token tidak ada", 400); return }

	idToken, err := s.Provider.Verifier(&oidc.Config{
		ClientID: s.OAuth.ClientID,
		SupportedSigningAlgs: []string{"RS256"},
	}).Verify(r.Context(), rawIDToken)
	if err != nil { http.Error(w, "ID token tidak valid", 400); return }
	if idToken.Nonce != tx.Nonce { http.Error(w, "nonce salah", 400); return }

	var claims struct { Subject string `json:"sub"`; EmailVerified bool `json:"email_verified"` }
	if err := idToken.Claims(&claims); err != nil || claims.Subject == "" {
		http.Error(w, "claims tidak valid", 400); return
	}
	// Store mengenkripsi token server-side dan membuat sesi lokal acak.
	sessionID, err := s.Sessions.Create(r.Context(), s.Issuer, claims.Subject, token)
	if err != nil { http.Error(w, "sesi gagal", 500); return }
	http.SetCookie(w, &http.Cookie{
		Name: "app_session", Value: sessionID, Path: "/",
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, tx.ReturnTo, http.StatusFound)
}

```

`Provider.Verifier` memakai discovery/JWKS dan memvalidasi issuer, audience, expiry, serta signature. Setelah itu, panggil UserInfo dengan `s.Provider.UserInfo(ctx, oauth2.StaticTokenSource(token))` dan pastikan `UserInfo.Subject == idToken.Subject` sebelum mempercayai profil.

Pada deployment multi-instance, transaksi dan sesi harus berada di shared store (misalnya Redis/database), bukan map proses. Gunakan comparison constant-time untuk `state` pada implementasi hardened.
