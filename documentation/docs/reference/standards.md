---
title: Standar dan sumber resmi
description: Spesifikasi protokol dan dokumentasi framework yang menjadi rujukan panduan ini.
---

# Standar dan sumber resmi

Dokumentasi integrasi ini mengikuti sumber primer berikut.

## OAuth dan OpenID Connect

- [OAuth 2.0 Authorization Framework — RFC 6749](https://www.rfc-editor.org/rfc/rfc6749.html)
- [Proof Key for Code Exchange — RFC 7636](https://www.rfc-editor.org/rfc/rfc7636.html)
- [OAuth 2.0 Token Revocation — RFC 7009](https://www.rfc-editor.org/rfc/rfc7009.html)
- [OAuth 2.0 Authorization Server Metadata — RFC 8414](https://www.rfc-editor.org/rfc/rfc8414.html)
- [OAuth 2.0 Security Best Current Practice — RFC 9700](https://www.rfc-editor.org/rfc/rfc9700.html)
- [JWT Profile for OAuth 2.0 Access Tokens — RFC 9068](https://www.rfc-editor.org/rfc/rfc9068.html)
- [OpenID Connect Core 1.0](https://openid.net/specs/openid-connect-core-1_0.html)
- [OpenID Connect Discovery 1.0](https://openid.net/specs/openid-connect-discovery-1_0.html)
- [JSON Web Token — RFC 7519](https://www.rfc-editor.org/rfc/rfc7519.html)
- [JSON Web Key — RFC 7517](https://www.rfc-editor.org/rfc/rfc7517.html)
- [SCIM Core Schema — RFC 7643](https://www.rfc-editor.org/rfc/rfc7643.html)

OAuth dan OpenID Connect tidak menetapkan struktur organisasi atau nama role bisnis. Assignment pengguna/grup dan kosakata role adalah kebijakan layanan, sedangkan error protokol, validasi token, serta claim standar mengikuti spesifikasi di atas. SCIM relevan bila provisioning pengguna dan grup kelak dilakukan lintas sistem; SCIM tidak menggantikan pemeriksaan authorization di aplikasi.

## Contoh framework

- [Next.js Route Handlers](https://nextjs.org/docs/app/getting-started/route-handlers)
- [Next.js `cookies`](https://nextjs.org/docs/app/api-reference/functions/cookies)
- [Laravel HTTP Client](https://laravel.com/docs/http-client)
- [Laravel Session](https://laravel.com/docs/session)
- [Go `net/http`](https://pkg.go.dev/net/http)
- [Go OAuth 2.0 package](https://pkg.go.dev/golang.org/x/oauth2)
- [Go OIDC package](https://pkg.go.dev/github.com/coreos/go-oidc/v3/oidc)

## Situs dokumentasi

Situs ini memakai struktur TypeScript template classic dan docs-only resmi:

- [Docusaurus installation](https://docusaurus.io/docs/installation)
- [Docusaurus docs-only mode](https://docusaurus.io/docs/docs-introduction#docs-only-mode)
- [Docusaurus configuration](https://docusaurus.io/docs/configuration)
- [Docusaurus deployment](https://docusaurus.io/docs/deployment)

Versi package Docusaurus dipin seragam ke `3.10.2`, versi stabil yang tercantum pada dokumentasi resmi saat situs ini dibuat. Node.js minimum adalah versi 20.
