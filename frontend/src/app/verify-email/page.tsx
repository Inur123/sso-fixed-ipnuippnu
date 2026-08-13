"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useRef, useState } from "react";
import { AlertCircle, MailCheck, MailWarning, RotateCw } from "lucide-react";
import { toast } from "sonner";

import { AuthShell } from "@/components/auth-shell";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { apiFetch, getErrorMessage } from "@/lib/api";
import {
  clearOTPResendCooldown,
  readOTPResendDeadline,
  startOTPResendCooldown,
} from "@/lib/email-verification";

function VerifyEmailForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const initialEmail = searchParams.get("email") ?? "";
  const [email, setEmail] = useState(initialEmail);
  const [otp, setOtp] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [resending, setResending] = useState(false);
  const [resendDeadline, setResendDeadline] = useState(0);
  const [now, setNow] = useState(0);
  const otpRef = useRef<HTMLInputElement>(null);
  const normalizedEmail = email.trim().toLowerCase();
  const deliveryFailed = searchParams.get("delivery") === "failed";
  const resendIn =
    now > 0 ? Math.max(0, Math.ceil((resendDeadline - now) / 1000)) : 0;

  useEffect(() => {
    if (initialEmail) otpRef.current?.focus();
  }, [initialEmail]);

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => {
      setResendDeadline(readOTPResendDeadline(normalizedEmail));
      setNow(Date.now());
    });
    return () => window.cancelAnimationFrame(frame);
  }, [normalizedEmail]);

  useEffect(() => {
    if (resendDeadline <= Date.now()) return;
    const tick = () => setNow(Date.now());
    const timer = window.setInterval(tick, 1000);
    return () => window.clearInterval(timer);
  }, [resendDeadline]);

  async function verify(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setError("");
    try {
      await apiFetch("/api/auth/verify-email", {
        method: "POST",
        body: JSON.stringify({ email: email.trim(), otp }),
      });
      clearOTPResendCooldown(normalizedEmail);
      toast.success("Email berhasil diverifikasi. Silakan masuk.");
      router.replace("/login");
    } catch (verifyError) {
      const message = getErrorMessage(verifyError);
      setError(message);
      toast.error(message);
    } finally {
      setSubmitting(false);
    }
  }

  async function resend() {
    if (!normalizedEmail || resendIn > 0) return;
    setResending(true);
    setError("");
    try {
      const response = await apiFetch<{ message?: string }>(
        "/api/auth/resend-verification",
        {
          method: "POST",
          body: JSON.stringify({ email: normalizedEmail }),
        },
      );
      setResendDeadline(startOTPResendCooldown(normalizedEmail));
      setNow(Date.now());
      toast.message(
        response.message ?? "Permintaan kirim ulang OTP telah diproses.",
      );
    } catch (resendError) {
      const message = getErrorMessage(resendError);
      setError(message);
      toast.error(message);
    } finally {
      setResending(false);
    }
  }

  return (
    <form className="w-full min-w-0 space-y-5" onSubmit={verify}>
      <div className="flex items-start gap-3 rounded-xl border border-primary/20 bg-primary/5 p-4">
        <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
          <MailCheck className="size-5" />
        </span>
        <div className="space-y-1">
          <p className="text-sm font-medium">Periksa kotak masuk email</p>
          <p className="text-xs leading-5 text-muted-foreground">
            Masukkan kode OTP dari IPNU IPPNU ID. Kode hanya dapat digunakan
            satu kali dan memiliki batas waktu.
          </p>
        </div>
      </div>
      {deliveryFailed && !error && (
        <Alert>
          <MailWarning />
          <AlertDescription>
            Akun Anda sudah dibuat, tetapi pengiriman email pertama gagal.
            Periksa alamat email lalu pilih Kirim ulang kode.
          </AlertDescription>
        </Alert>
      )}
      {error && (
        <Alert variant="destructive">
          <AlertCircle />
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      <div className="space-y-2">
        <Label htmlFor="verification-email">Email</Label>
        <Input
          id="verification-email"
          type="email"
          autoComplete="email"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          placeholder="nama@organisasi.id"
          required
        />
      </div>
      <div className="space-y-2">
        <Label htmlFor="otp">Kode OTP</Label>
        <Input
          ref={otpRef}
          id="otp"
          inputMode="numeric"
          autoComplete="one-time-code"
          value={otp}
          onChange={(event) =>
            setOtp(event.target.value.replace(/\D/g, "").slice(0, 6))
          }
          placeholder="000000"
          className="h-12 max-w-full text-center text-xl tracking-[0.28em] tabular-nums sm:tracking-[0.4em]"
          minLength={6}
          maxLength={6}
          required
        />
      </div>
      <Button
        className="w-full min-w-0"
        size="lg"
        disabled={submitting || otp.length !== 6}
      >
        {submitting ? <Spinner /> : <MailCheck />}
        {submitting ? "Memverifikasi..." : "Verifikasi email"}
      </Button>
      <div className="flex flex-col items-center gap-3 text-center text-sm">
        <Button
          type="button"
          variant="ghost"
          className="h-auto min-h-9 max-w-full whitespace-normal"
          disabled={!normalizedEmail || resending || resendIn > 0}
          onClick={resend}
        >
          {resending ? <Spinner /> : <RotateCw />}
          {resendIn > 0
            ? `Kirim ulang dalam ${resendIn} detik`
            : "Kirim ulang kode"}
        </Button>
        <p className="text-muted-foreground">
          Sudah terverifikasi?{" "}
          <Link
            href="/login"
            className="font-medium text-primary underline-offset-4 hover:underline"
          >
            Kembali ke login
          </Link>
        </p>
      </div>
    </form>
  );
}

export default function VerifyEmailPage() {
  return (
    <AuthShell
      title="Verifikasi email"
      description="Selesaikan aktivasi email sebelum masuk ke layanan IPNU IPPNU ID."
    >
      <Suspense
        fallback={
          <div className="flex justify-center py-12">
            <Spinner className="size-6" />
          </div>
        }
      >
        <VerifyEmailForm />
      </Suspense>
    </AuthShell>
  );
}
