---
title: Discovery dan JWKS RS256
description: Temukan endpoint dan validasi ID token menggunakan metadata OIDC serta JWKS.
---

# Discovery dan JWKS RS256

Gunakan discovery pada startup atau secara berkala agar konfigurasi endpoint tidak tersebar di source code.

## Dokumen discovery

OpenID Connect:

```text
https://api.pelajarnumagetan.id/.well-known/openid-configuration
```

OAuth Authorization Server Metadata:

```text
https://api.pelajarnumagetan.id/.well-known/oauth-authorization-server
```

Contoh field penting OIDC:

```json
{
  "issuer": "https://api.pelajarnumagetan.id",
  "authorization_endpoint": "https://api.pelajarnumagetan.id/oauth/authorize",
  "token_endpoint": "https://api.pelajarnumagetan.id/oauth/token",
  "revocation_endpoint": "https://api.pelajarnumagetan.id/oauth/revoke",
  "userinfo_endpoint": "https://api.pelajarnumagetan.id/v1/user/me",
  "jwks_uri": "https://api.pelajarnumagetan.id/oauth/jwks",
  "response_types_supported": ["code"],
  "grant_types_supported": ["authorization_code", "refresh_token"],
  "id_token_signing_alg_values_supported": ["RS256"],
  "token_endpoint_auth_methods_supported": ["client_secret_post"],
  "code_challenge_methods_supported": ["S256"]
}
```

RP wajib membandingkan `issuer` hasil discovery dengan issuer yang dikonfigurasi menggunakan exact string comparison. Jangan menerima discovery dari URL yang dikirim pengguna.

## JWKS

`jwks_uri` mengembalikan public key RSA:

```json
{
  "keys": [
    {
      "kty": "RSA",
      "use": "sig",
      "alg": "RS256",
      "kid": "<key-id>",
      "n": "<modulus-base64url>",
      "e": "AQAB"
    }
  ]
}
```

Validator memilih key berdasarkan `kid` header ID token, memastikan `alg=RS256`, lalu memverifikasi signature dan claims. Cache JWKS sesuai header HTTP (`Cache-Control: public, max-age=300`). Bila `kid` tidak ditemukan, refresh JWKS satu kali untuk mengakomodasi rotasi; jangan terus-menerus fetch pada token invalid.

:::danger JWKS bukan untuk access token
JWKS issuer hanya mendokumentasikan public key ID token RS256. Access token IPNU IPPNU ID adalah kredensial bearer internal; RP harus memperlakukannya opaque dan mengirimkannya ke UserInfo.
:::

## Rotasi key

Saat production, issuer memakai private key RSA persisten minimal 2048 bit. Operator perlu mempertahankan public key lama di JWKS selama ID token lama masih mungkin valid ketika melakukan rotasi. RP harus memilih key berdasarkan `kid`, bukan mengasumsikan hanya ada satu key.

Dalam development, bila `OIDC_PRIVATE_KEY_PATH` kosong, backend boleh membuat key ephemeral. Restart akan mengganti key dan ID token lama tidak lagi dapat diverifikasi; perilaku ini tidak boleh digunakan di production.
