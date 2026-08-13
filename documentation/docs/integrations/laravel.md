---
title: Laravel / PHP
description: Contoh integrasi Laravel menggunakan session, HTTP client, dan verifikasi JWT RS256.
---

# Integrasi Laravel / PHP

Contoh berikut menggunakan HTTP client dan session bawaan Laravel serta `firebase/php-jwt` untuk validasi signature.

```bash
composer require firebase/php-jwt
```

```dotenv title=".env"
SSO_ISSUER=http://localhost:8080
SSO_CLIENT_ID=<client-id>
SSO_CLIENT_SECRET=<client-secret>
SSO_REDIRECT_URI=http://localhost:8000/auth/sso/callback
```

Expose konfigurasi melalui `config/services.php`; jangan memanggil `env()` langsung di controller production karena Laravel config dapat di-cache.

```php title="config/services.php"
'sso' => [
    'issuer' => rtrim(env('SSO_ISSUER', ''), '/'),
    'client_id' => env('SSO_CLIENT_ID'),
    'client_secret' => env('SSO_CLIENT_SECRET'),
    'redirect_uri' => env('SSO_REDIRECT_URI'),
],
```

## Routes

```php title="routes/web.php"
use App\Http\Controllers\SsoController;

Route::get('/auth/sso', [SsoController::class, 'redirect'])->name('sso.redirect');
Route::get('/auth/sso/callback', [SsoController::class, 'callback'])->name('sso.callback');
Route::post('/logout', [SsoController::class, 'logout'])->name('logout');
```

## Controller

```php title="app/Http/Controllers/SsoController.php"
<?php

namespace App\Http\Controllers;

use Firebase\JWT\JWK;
use Firebase\JWT\JWT;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Http;

class SsoController extends Controller
{
    private function b64url(string $value): string
    {
        return rtrim(strtr(base64_encode($value), '+/', '-_'), '=');
    }

    private function discovery(): array
    {
        $issuer = config('services.sso.issuer');
        $metadata = Http::acceptJson()
            ->timeout(5)
            ->get("{$issuer}/.well-known/openid-configuration")
            ->throw()->json();
        abort_unless(($metadata['issuer'] ?? null) === $issuer, 500, 'Issuer discovery tidak cocok');
        return $metadata;
    }

    public function redirect(Request $request)
    {
        $state = $this->b64url(random_bytes(32));
        $nonce = $this->b64url(random_bytes(32));
        $verifier = $this->b64url(random_bytes(32));
        $request->session()->put('sso_tx', compact('state', 'nonce', 'verifier'));

        $metadata = $this->discovery();
        $query = http_build_query([
            'response_type' => 'code',
            'client_id' => config('services.sso.client_id'),
            'redirect_uri' => config('services.sso.redirect_uri'),
            'scope' => 'openid profile email',
            'state' => $state,
            'nonce' => $nonce,
            'code_challenge' => $this->b64url(hash('sha256', $verifier, true)),
            'code_challenge_method' => 'S256',
        ], '', '&', PHP_QUERY_RFC3986);

        return redirect()->away($metadata['authorization_endpoint'].'?'.$query);
    }

    public function callback(Request $request)
    {
        // pull() menghabiskan transaksi agar callback tidak dapat digunakan lagi.
        $tx = $request->session()->pull('sso_tx');
        abort_unless(is_array($tx), 400, 'Transaksi login kedaluwarsa');
        abort_unless(
            is_string($request->query('state')) && hash_equals($tx['state'], $request->query('state')),
            400,
            'state tidak valid'
        );
        abort_unless($request->query('iss') === config('services.sso.issuer'), 400, 'issuer tidak valid');
        abort_if($request->filled('error'), 400, 'SSO gagal: '.$request->query('error'));

        $metadata = $this->discovery();
        $tokens = Http::asForm()->acceptJson()->timeout(10)->post($metadata['token_endpoint'], [
            'grant_type' => 'authorization_code',
            'client_id' => config('services.sso.client_id'),
            'client_secret' => config('services.sso.client_secret'),
            'redirect_uri' => config('services.sso.redirect_uri'),
            'code' => $request->query('code'),
            'code_verifier' => $tx['verifier'],
        ])->throw()->json();

        $jwks = Http::acceptJson()->timeout(5)->get($metadata['jwks_uri'])->throw()->json();
        JWT::$leeway = 60;
        $claims = JWT::decode($tokens['id_token'], JWK::parseKeySet($jwks, 'RS256'));

        $audience = is_array($claims->aud ?? null) ? $claims->aud : [$claims->aud ?? null];
        abort_unless(($claims->iss ?? null) === config('services.sso.issuer'), 400, 'iss ID token salah');
        abort_unless(in_array(config('services.sso.client_id'), $audience, true), 400, 'aud salah');
        abort_unless(($claims->nonce ?? null) === $tx['nonce'], 400, 'nonce salah');
        abort_unless(is_string($claims->sub ?? null) && $claims->sub !== '', 400, 'sub tidak ada');

        $userInfo = Http::withToken($tokens['access_token'])
            ->acceptJson()->get($metadata['userinfo_endpoint'])->throw()->json();
        abort_unless(hash_equals($claims->sub, $userInfo['sub'] ?? ''), 400, 'sub UserInfo berbeda');

        // Upsert berdasarkan [issuer, sub], terapkan policy lokal, simpan token terenkripsi.
        $user = app(SsoAccountService::class)->login($claims, $userInfo, $tokens);
        auth()->login($user);
        $request->session()->regenerate();
        return redirect()->intended('/dashboard');
    }
}
```

`firebase/php-jwt` memvalidasi signature serta claim waktu seperti `exp`/`nbf`; kode tetap memeriksa issuer, audience, nonce, subject, dan algoritma/key dari JWKS. Cache discovery/JWKS di server sesuai TTL dan refresh sekali bila `kid` baru belum ditemukan.

## Penyimpanan sesi dan token

- Gunakan driver session server-side seperti Redis/database pada production.
- Enkripsi access dan refresh token at rest; jangan simpan di session cookie client-side.
- Simpan refresh token baru dan tandai token lama tidak dapat dipakai dalam satu transaksi.
- Saat logout, panggil revocation endpoint dari backend lalu invalidate dan regenerate sesi Laravel.
