"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { AlertCircle, Check, UserPlus } from "lucide-react";
import { toast } from "sonner";

import { AuthShell } from "@/components/auth-shell";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { PasswordInput } from "@/components/ui/password-input";
import { Spinner } from "@/components/ui/spinner";
import { apiFetch, getErrorMessage } from "@/lib/api";
import {
  clearOTPResendCooldown,
  startOTPResendCooldown,
} from "@/lib/email-verification";

interface RegisterResponse {
  message?: string;
  verification_email_sent: boolean;
}

export default function RegisterPage() {
  const router = useRouter();
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setError("");
    try {
      const normalizedEmail = email.trim().toLowerCase();
      const response = await apiFetch<RegisterResponse>("/api/auth/register", {
        method: "POST",
        body: JSON.stringify({ name, email: normalizedEmail, password }),
      });
      if (response.verification_email_sent) {
        startOTPResendCooldown(normalizedEmail);
        toast.success(
          response.message ??
            "Akun berhasil dibuat dan OTP telah dikirim ke email Anda.",
        );
      } else {
        clearOTPResendCooldown(normalizedEmail);
        toast.warning(
          response.message ??
            "Akun berhasil dibuat, tetapi email OTP belum terkirim. Silakan kirim ulang kode.",
        );
      }
      const query = new URLSearchParams({ email: normalizedEmail });
      if (!response.verification_email_sent) query.set("delivery", "failed");
      router.replace(`/verify-email?${query.toString()}`);
    } catch (submitError) {
      setError(getErrorMessage(submitError));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <AuthShell
      title="Buat akun anggota"
      description="Daftar sebagai anggota IPNU IPPNU ID, kemudian verifikasi email Anda."
    >
      <form className="w-full min-w-0 space-y-5" onSubmit={handleSubmit}>
        {error && (
          <Alert variant="destructive">
            <AlertCircle />
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}
        <div className="space-y-2">
          <Label htmlFor="name">Nama lengkap</Label>
          <Input
            id="name"
            autoComplete="name"
            value={name}
            onChange={(event) => setName(event.target.value)}
            minLength={2}
            maxLength={120}
            required
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="email">Email</Label>
          <Input
            id="email"
            type="email"
            autoComplete="email"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            required
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="password">Kata sandi</Label>
          <PasswordInput
            id="password"
            autoComplete="new-password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            minLength={8}
            maxLength={72}
            required
          />
          <div className="flex items-start gap-2 text-xs leading-5 text-muted-foreground">
            <Check className="mt-0.5 size-3.5 shrink-0 text-primary" />
            <p>Gunakan minimal 8 karakter</p>
          </div>
        </div>
        <Button className="w-full min-w-0" size="lg" disabled={submitting}>
          {submitting ? <Spinner /> : <UserPlus />}
          {submitting ? "Membuat akun..." : "Buat akun"}
        </Button>
        <p className="break-words text-center text-sm leading-6 text-muted-foreground">
          Sudah punya akun?{" "}
          <Link
            href="/login"
            className="font-medium text-primary underline-offset-4 hover:underline"
          >
            Masuk
          </Link>
        </p>
      </form>
    </AuthShell>
  );
}
