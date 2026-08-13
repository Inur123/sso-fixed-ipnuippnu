"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useState } from "react";
import { AlertCircle, Ban, LocateFixed, LogIn, MailWarning } from "lucide-react";
import { toast } from "sonner";

import { AuthShell } from "@/components/auth-shell";
import { useAuth } from "@/components/auth-provider";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { PasswordInput } from "@/components/ui/password-input";
import { Spinner } from "@/components/ui/spinner";
import { APIError, apiFetch, getErrorMessage, type User } from "@/lib/api";

const DEFAULT_LOGIN_DESTINATION = "/dashboard/profil";

function requestLocation(): Promise<GeolocationPosition> {
  return new Promise((resolve, reject) => {
    if (!navigator.geolocation) {
      reject(new Error("Browser ini tidak mendukung akses lokasi."));
      return;
    }
    navigator.geolocation.getCurrentPosition(resolve, reject, {
      enableHighAccuracy: false,
      timeout: 15000,
      maximumAge: 60000,
    });
  });
}

function locationErrorMessage(error: unknown) {
  if (typeof error === "object" && error !== null && "code" in error) {
    const code = Number(error.code);
    if (code === 1) return "Izin lokasi wajib diberikan untuk masuk.";
    if (code === 3) return "Lokasi tidak berhasil diperoleh. Silakan coba lagi.";
  }
  return error instanceof Error ? error.message : "Lokasi tidak berhasil diperoleh.";
}

function safeLoginDestination(value: string | null) {
  if (!value || typeof window === "undefined") {
    return DEFAULT_LOGIN_DESTINATION;
  }

  try {
    const target = new URL(value, window.location.origin);
    if (
      target.origin !== window.location.origin ||
      target.pathname !== "/oauth/authorize"
    ) {
      return DEFAULT_LOGIN_DESTINATION;
    }

    const scope = target.searchParams.get("scope") ?? "";
    const hasRequiredOAuthParameters =
      target.searchParams.get("response_type") === "code" &&
      target.searchParams.get("code_challenge_method") === "S256" &&
      Boolean(target.searchParams.get("client_id")) &&
      Boolean(target.searchParams.get("redirect_uri")) &&
      Boolean(target.searchParams.get("state")) &&
      Boolean(target.searchParams.get("code_challenge")) &&
      Boolean(scope) &&
      (!scope.split(" ").includes("openid") ||
        Boolean(target.searchParams.get("nonce")));

    return hasRequiredOAuthParameters
      ? `${target.pathname}${target.search}`
      : DEFAULT_LOGIN_DESTINATION;
  } catch {
    return DEFAULT_LOGIN_DESTINATION;
  }
}

function LoginForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { setUser } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [errorCode, setErrorCode] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setError("");
    setErrorCode("");
    try {
      const position = await requestLocation();
      const data = await apiFetch<{ user: User }>("/api/auth/login", {
        method: "POST",
        body: JSON.stringify({
          email,
          password,
          location: {
            latitude: position.coords.latitude,
            longitude: position.coords.longitude,
            accuracy: position.coords.accuracy,
          },
          device: {
            platform: navigator.platform || "Browser",
            language: navigator.language,
            timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
          },
        }),
      });
      setUser(data.user);
      toast.success("Login berhasil. Selamat datang kembali.");
      router.replace(safeLoginDestination(searchParams.get("callbackUrl")));
      router.refresh();
    } catch (submitError) {
      const message = submitError instanceof APIError ? getErrorMessage(submitError) : locationErrorMessage(submitError);
      setError(message);
      toast.error(message);
      if (submitError instanceof APIError) setErrorCode(submitError.code ?? "");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form className="min-w-0 space-y-5" onSubmit={handleSubmit}>
      {error && (
        <Alert variant="destructive">
          {errorCode === "email_unverified" ? <MailWarning /> : errorCode === "account_inactive" ? <Ban /> : <AlertCircle />}
          <AlertDescription className="space-y-2">
            <p>{error}</p>
            {errorCode === "email_unverified" && (
              <Button variant="link" className="h-auto p-0 text-destructive" asChild>
                <Link href={`/verify-email?email=${encodeURIComponent(email)}`}>Verifikasi email sekarang</Link>
              </Button>
            )}
            {errorCode === "account_inactive" && (
              <p className="text-xs">Hubungi super admin IPNU IPPNU ID untuk mengaktifkan kembali akun Anda.</p>
            )}
          </AlertDescription>
        </Alert>
      )}
      <div className="space-y-2">
        <Label htmlFor="email">Email</Label>
        <Input id="email" type="email" autoComplete="email" value={email} onChange={(event) => setEmail(event.target.value)} placeholder="nama@organisasi.id" required />
      </div>
      <div className="space-y-2">
        <Label htmlFor="password">Kata sandi</Label>
        <PasswordInput id="password" autoComplete="current-password" value={password} onChange={(event) => setPassword(event.target.value)} required />
      </div>
      <Button className="w-full min-w-0" size="lg" disabled={submitting}>
        {submitting ? <Spinner /> : <LogIn />}{submitting ? "Memeriksa lokasi dan akun..." : "Masuk"}
      </Button>
      <p className="flex items-start gap-2 text-xs leading-5 text-muted-foreground"><LocateFixed className="mt-0.5 size-3.5 shrink-0" />Login memerlukan izin lokasi. Lokasi, IP, dan perangkat dicatat untuk keamanan akun.</p>
      <p className="break-words text-center text-sm leading-6 text-muted-foreground">
        Belum memiliki akun? <Link href="/register" className="font-medium text-primary underline-offset-4 hover:underline">Daftar</Link>
      </p>
    </form>
  );
}

export default function LoginPage() {
  return (
    <AuthShell title="Selamat datang kembali" description="Masuk menggunakan akun IPNU IPPNU ID Anda.">
      <Suspense fallback={<div className="flex justify-center py-12"><Spinner className="size-6" /></div>}><LoginForm /></Suspense>
    </AuthShell>
  );
}
