"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";
import {
  AlertCircle,
  ArrowLeft,
  KeyRound,
  Mail,
  MailCheck,
  ShieldCheck,
} from "lucide-react";
import { toast } from "sonner";

import { AuthShell } from "@/components/auth-shell";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { PasswordInput } from "@/components/ui/password-input";
import { Spinner } from "@/components/ui/spinner";
import { apiFetch, getErrorMessage } from "@/lib/api";

const MINIMUM_PASSWORD_LENGTH = 8;

function PasswordResetForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [token] = useState(() => searchParams.get("token")?.trim() ?? "");
  const [email, setEmail] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [requestSent, setRequestSent] = useState(false);
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const isResetFlow = token.length > 0;
  const passwordLength = Array.from(newPassword).length;
  const passwordTooLong = new TextEncoder().encode(newPassword).length > 72;
  const passwordTooShort =
    passwordLength > 0 && passwordLength < MINIMUM_PASSWORD_LENGTH;
  const confirmationMismatch =
    confirmPassword.length > 0 && newPassword !== confirmPassword;

  useEffect(() => {
    if (!token) return;
    const cleanURL = new URL(window.location.href);
    cleanURL.searchParams.delete("token");
    window.history.replaceState(
      window.history.state,
      "",
      `${cleanURL.pathname}${cleanURL.search}${cleanURL.hash}`,
    );
  }, [token]);

  async function requestReset(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setError("");
    try {
      await apiFetch<{ message: string }>("/api/auth/reset-password/request", {
        method: "POST",
        body: JSON.stringify({ email: email.trim() }),
      });
      setRequestSent(true);
    } catch (requestError) {
      const message = getErrorMessage(requestError);
      setError(message);
      toast.error(message);
    } finally {
      setSubmitting(false);
    }
  }

  async function confirmReset(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (passwordLength < MINIMUM_PASSWORD_LENGTH) {
      setError("Kata sandi baru minimal 8 karakter.");
      return;
    }
    if (passwordTooLong) {
      setError("Kata sandi maksimal 72 byte. Kurangi jumlah karakter atau simbol.");
      return;
    }
    if (newPassword !== confirmPassword) {
      setError("Konfirmasi kata sandi belum sama.");
      return;
    }
    setSubmitting(true);
    setError("");
    try {
      await apiFetch<{ message: string }>("/api/auth/reset-password/confirm", {
        method: "POST",
        body: JSON.stringify({ token, new_password: newPassword }),
      });
      toast.success("Kata sandi berhasil diperbarui.");
      router.replace("/login?reset=success");
      router.refresh();
    } catch (resetError) {
      const message = getErrorMessage(resetError);
      setError(message);
      toast.error(message);
    } finally {
      setSubmitting(false);
    }
  }

  if (!isResetFlow && requestSent) {
    return (
      <AuthShell
        title="Periksa email Anda"
        description="Permintaan pengaturan ulang telah diproses dengan aman."
        panelBadge="Pemulihan akun"
        panelTitle="Kembali ke akun Anda dengan aman."
        panelDescription="Tautan reset berlaku selama 15 menit dan hanya dapat digunakan sekali. Jangan bagikan tautan ini kepada siapa pun."
      >
        <div className="space-y-6" aria-live="polite">
          <div className="flex items-start gap-3 rounded-xl border border-primary/20 bg-primary/5 p-4">
            <span className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
              <MailCheck className="size-5" />
            </span>
            <div className="space-y-1">
              <p className="text-sm font-medium">Permintaan telah diterima</p>
              <p className="text-xs leading-5 text-muted-foreground">
                Jika akun terdaftar, aktif, dan emailnya sudah terverifikasi,
                tautan pengaturan ulang akan dikirim. Periksa juga folder spam.
              </p>
            </div>
          </div>
          <Button className="w-full" size="lg" asChild>
            <Link href="/login">
              <ArrowLeft /> Kembali ke halaman masuk
            </Link>
          </Button>
        </div>
      </AuthShell>
    );
  }

  return (
    <AuthShell
      title={isResetFlow ? "Buat kata sandi baru" : "Atur ulang kata sandi"}
      description={
        isResetFlow
          ? "Gunakan kata sandi baru untuk mengamankan kembali akun PelajarNU Magetan ID Anda."
          : "Masukkan email akun PelajarNU Magetan ID untuk menerima tautan pengaturan ulang."
      }
      panelBadge="Pemulihan akun"
      panelTitle="Kembali ke akun Anda dengan aman."
      panelDescription="Tautan reset dibatasi waktu, hanya dapat digunakan sekali, dan seluruh sesi lama dicabut setelah kata sandi diperbarui."
    >
      <form
        className="w-full min-w-0 space-y-5"
        onSubmit={isResetFlow ? confirmReset : requestReset}
      >
        {error && (
          <Alert variant="destructive">
            <AlertCircle />
            <AlertDescription>
              {error}
              {isResetFlow && (
                <a className="mt-2 block font-medium underline" href="/reset-password">
                  Minta tautan reset baru
                </a>
              )}
            </AlertDescription>
          </Alert>
        )}

        {isResetFlow ? (
          <>
            <div className="flex items-start gap-3 rounded-xl border border-primary/20 bg-primary/5 p-4">
              <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
                <ShieldCheck className="size-5" />
              </span>
              <p className="text-xs leading-5 text-muted-foreground">
                Setelah berhasil, seluruh sesi dan akses aplikasi lama akan
                dicabut. Anda perlu masuk kembali menggunakan kata sandi baru.
              </p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="new-password">Kata sandi baru</Label>
              <PasswordInput
                id="new-password"
                autoComplete="new-password"
                value={newPassword}
                onChange={(event) => setNewPassword(event.target.value)}
                required
                minLength={MINIMUM_PASSWORD_LENGTH}
                maxLength={72}
              />
              {passwordTooShort && (
                <p className="flex items-center gap-2 text-xs text-destructive">
                  <AlertCircle className="size-3.5" /> Gunakan minimal 8 karakter
                </p>
              )}
              {passwordTooLong && (
                <p className="text-xs text-destructive">Kata sandi maksimal 72 byte. Kurangi jumlah karakter atau simbol.</p>
              )}
            </div>
            <div className="space-y-2">
              <Label htmlFor="confirm-password">Konfirmasi kata sandi baru</Label>
              <PasswordInput
                id="confirm-password"
                autoComplete="new-password"
                value={confirmPassword}
                onChange={(event) => setConfirmPassword(event.target.value)}
                required
                minLength={MINIMUM_PASSWORD_LENGTH}
                maxLength={72}
              />
              {confirmationMismatch && (
                <p className="flex items-center gap-2 text-xs text-destructive">
                  <AlertCircle className="size-3.5" /> Kata sandi belum sama
                </p>
              )}
            </div>
            <Button
              className="w-full min-w-0"
              size="lg"
              disabled={
                submitting ||
                passwordLength < MINIMUM_PASSWORD_LENGTH ||
                passwordTooLong ||
                newPassword !== confirmPassword
              }
            >
              {submitting ? <Spinner /> : <KeyRound />}
              {submitting ? "Menyimpan..." : "Simpan kata sandi baru"}
            </Button>
          </>
        ) : (
          <>
            <div className="space-y-2">
              <Label htmlFor="reset-email">Email</Label>
              <Input
                id="reset-email"
                type="email"
                autoComplete="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                placeholder="nama@organisasi.id"
                required
                maxLength={254}
              />
            </div>
            <Button
              className="w-full min-w-0"
              size="lg"
              disabled={submitting}
            >
              {submitting ? <Spinner /> : <Mail />}
              {submitting ? "Memproses..." : "Kirim tautan reset"}
            </Button>
          </>
        )}

        <p className="text-center text-sm leading-6 text-muted-foreground">
          Ingat kata sandi Anda?{" "}
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

export default function ResetPasswordPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-svh items-center justify-center">
          <Spinner className="size-6" />
        </div>
      }
    >
      <PasswordResetForm />
    </Suspense>
  );
}
