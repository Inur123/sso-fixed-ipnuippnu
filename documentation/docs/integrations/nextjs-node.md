---
title: Next.js / Node.js
description: Contoh pola BFF Next.js App Router dengan transaksi login server-side dan validasi ID token memakai jose.
---

# Integrasi Next.js / Node.js

Gunakan pola BFF: browser hanya menerima cookie sesi lokal; token dan `client_secret` tetap di server Node.js. Contoh berikut memakai Next.js App Router dan [`jose`](https://github.com/panva/jose).

```bash
npm install jose
```

```dotenv title=".env.local (server-only)"
SSO_ISSUER=https://api.pelajarnumagetan.id
SSO_CLIENT_ID=<client-id>
SSO_CLIENT_SECRET=<client-secret>
SSO_REDIRECT_URI=http://localhost:3002/api/auth/sso/callback
```

Jangan memberi prefix `NEXT_PUBLIC_` pada client secret atau token.

## Helper protokol

```ts title="src/lib/sso.ts"
import {createHash, randomBytes, timingSafeEqual} from 'node:crypto';
import {createRemoteJWKSet, jwtVerify} from 'jose';

export const env = {
  issuer: must('SSO_ISSUER').replace(/\/$/, ''),
  clientId: must('SSO_CLIENT_ID'),
  clientSecret: must('SSO_CLIENT_SECRET'),
  redirectUri: must('SSO_REDIRECT_URI'),
};

function must(name: string) {
  const value = process.env[name];
  if (!value) throw new Error(`${name} wajib diisi`);
  return value;
}

export const randomValue = () => randomBytes(32).toString('base64url');
export const challenge = (verifier: string) =>
  createHash('sha256').update(verifier, 'ascii').digest('base64url');

export function equal(a: string, b: string) {
  const left = Buffer.from(a);
  const right = Buffer.from(b);
  return left.length === right.length && timingSafeEqual(left, right);
}

let metadataPromise: Promise<any> | undefined;
export function metadata() {
  return metadataPromise ??= fetch(
    `${env.issuer}/.well-known/openid-configuration`,
    {cache: 'no-store'},
  ).then(async (response) => {
    if (!response.ok) throw new Error('Discovery gagal');
    const value = await response.json();
    if (value.issuer !== env.issuer) throw new Error('Issuer discovery tidak cocok');
    if (!value.id_token_signing_alg_values_supported?.includes('RS256')) {
      throw new Error('RS256 tidak diumumkan issuer');
    }
    return value;
  });
}

export async function verifyIdToken(raw: string, expectedNonce: string) {
  const configuration = await metadata();
  const jwks = createRemoteJWKSet(new URL(configuration.jwks_uri));
  const {payload, protectedHeader} = await jwtVerify(raw, jwks, {
    issuer: env.issuer,
    audience: env.clientId,
    algorithms: ['RS256'],
    clockTolerance: 60,
  });

  if (protectedHeader.alg !== 'RS256' || typeof protectedHeader.kid !== 'string') {
    throw new Error('Header ID token tidak valid');
  }
  if (typeof payload.sub !== 'string' || !payload.sub) throw new Error('sub tidak ada');
  if (typeof payload.iat !== 'number') throw new Error('iat tidak ada');
  if (payload.nonce !== expectedNonce) throw new Error('nonce tidak cocok');
  return payload;
}
```

`createRemoteJWKSet` menangani pemilihan `kid` dan cache JWKS. Batasi algoritma secara eksplisit agar token dengan algoritma lain ditolak.

## Menyimpan transaksi login

Implementasikan storage server-side berumur maksimal 10 menit:

```ts
type LoginTransaction = {
  state: string;
  nonce: string;
  verifier: string;
  returnTo: string;
};

// Implementasi Redis/database: key acak -> record, TTL 10 menit.
declare function putLoginTransaction(value: LoginTransaction): Promise<string>;
declare function takeLoginTransaction(id: string): Promise<LoginTransaction | null>;
```

`takeLoginTransaction` harus atomik (get-and-delete) supaya callback tidak dapat dipakai ulang. Cookie browser hanya menyimpan ID transaksi acak, bukan client secret atau token.

## Route mulai login

```ts title="app/api/auth/sso/start/route.ts"
import {cookies} from 'next/headers';
import {NextResponse} from 'next/server';
import {challenge, env, metadata, randomValue} from '@/lib/sso';

export async function GET() {
  const state = randomValue();
  const nonce = randomValue();
  const verifier = randomValue();
  const txId = await putLoginTransaction({state, nonce, verifier, returnTo: '/'});

  const jar = await cookies();
  jar.set('sso_tx', txId, {
    httpOnly: true,
    secure: process.env.NODE_ENV === 'production',
    sameSite: 'lax',
    path: '/api/auth/sso/callback',
    maxAge: 600,
  });

  const oidc = await metadata();
  const url = new URL(oidc.authorization_endpoint);
  url.search = new URLSearchParams({
    response_type: 'code',
    client_id: env.clientId,
    redirect_uri: env.redirectUri,
    scope: 'openid profile email',
    state,
    nonce,
    code_challenge: challenge(verifier),
    code_challenge_method: 'S256',
  }).toString();
  return NextResponse.redirect(url);
}
```

## Route callback

```ts title="app/api/auth/sso/callback/route.ts"
import {cookies} from 'next/headers';
import {NextRequest, NextResponse} from 'next/server';
import {env, equal, metadata, verifyIdToken} from '@/lib/sso';

export async function GET(request: NextRequest) {
  const jar = await cookies();
  const txId = jar.get('sso_tx')?.value;
  jar.delete('sso_tx');
  const tx = txId ? await takeLoginTransaction(txId) : null;
  if (!tx) return new NextResponse('Transaksi login kedaluwarsa', {status: 400});

  const query = request.nextUrl.searchParams;
  if (!equal(query.get('state') ?? '', tx.state)) {
    return new NextResponse('state tidak valid', {status: 400});
  }
  if (query.get('iss') !== env.issuer) {
    return new NextResponse('issuer callback tidak valid', {status: 400});
  }
  if (query.has('error')) {
    // Log hanya kode error, jangan seluruh URL callback.
    return new NextResponse(`SSO gagal: ${query.get('error')}`, {status: 400});
  }
  const code = query.get('code');
  if (!code) return new NextResponse('code tidak ada', {status: 400});

  const oidc = await metadata();
  const tokenResponse = await fetch(oidc.token_endpoint, {
    method: 'POST',
    headers: {'content-type': 'application/x-www-form-urlencoded'},
    body: new URLSearchParams({
      grant_type: 'authorization_code',
      client_id: env.clientId,
      client_secret: env.clientSecret,
      redirect_uri: env.redirectUri,
      code,
      code_verifier: tx.verifier,
    }),
    cache: 'no-store',
  });
  const tokens = await tokenResponse.json();
  if (!tokenResponse.ok || typeof tokens.id_token !== 'string') {
    return new NextResponse('Token exchange gagal', {status: 400});
  }

  const identity = await verifyIdToken(tokens.id_token, tx.nonce);
  const userInfoResponse = await fetch(oidc.userinfo_endpoint, {
    headers: {authorization: `Bearer ${tokens.access_token}`},
    cache: 'no-store',
  });
  const userInfo = await userInfoResponse.json();
  if (!userInfoResponse.ok || userInfo.sub !== identity.sub) {
    return new NextResponse('UserInfo tidak valid', {status: 400});
  }

  // Simpan token terenkripsi di server; hasilkan ID sesi lokal acak.
  const sessionId = await createAppSession({identity, userInfo, tokens});
  jar.set('app_session', sessionId, {
    httpOnly: true, secure: true, sameSite: 'lax', path: '/',
  });
  return NextResponse.redirect(new URL(tx.returnTo, request.url));
}
```

`createAppSession` adalah batas aplikasi: upsert akun berdasarkan `(env.issuer, identity.sub)`, terapkan otorisasi lokal, enkripsi token saat disimpan, dan jangan pernah memasukkan token ke cookie.

## Logout

Hapus sesi lokal, lalu panggil revocation endpoint dari server menggunakan refresh token terbaru. Jangan mengarahkan browser ke revocation endpoint karena request membutuhkan client secret.
