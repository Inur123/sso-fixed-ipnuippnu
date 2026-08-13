"use client";

import { useState } from "react";
import { KeyRound, LockKeyhole, Save, ShieldAlert } from "lucide-react";
import { toast } from "sonner";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { PasswordInput } from "@/components/ui/password-input";
import { Spinner } from "@/components/ui/spinner";
import { apiFetch, getErrorMessage } from "@/lib/api";

export default function KeamananPage() {
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [saving, setSaving] = useState(false);

  async function changePassword(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (newPassword !== confirmPassword) {
      toast.error("Konfirmasi kata sandi tidak sama.");
      return;
    }
    setSaving(true);
    try {
      await apiFetch("/api/profile/password", { method: "POST", body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }) });
      setCurrentPassword(""); setNewPassword(""); setConfirmPassword("");
      toast.success("Kata sandi diperbarui dan sesi lain telah dicabut.");
    } catch (error) {
      toast.error(getErrorMessage(error));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="space-y-7">
      <PageHeader title="Keamanan akun" description="Kelola kredensial dan pahami lapisan perlindungan yang aktif pada akun Anda." />
      <div className="grid gap-6 xl:grid-cols-[minmax(0,1.15fr)_minmax(320px,.85fr)]">
        <Card>
          <CardHeader><div className="mb-2 flex size-10 items-center justify-center rounded-xl bg-primary/10 text-primary"><KeyRound className="size-5" /></div><CardTitle>Ubah kata sandi</CardTitle><CardDescription>Perubahan akan mencabut seluruh sesi lain dan token aplikasi.</CardDescription></CardHeader>
          <CardContent>
            <form className="space-y-4" onSubmit={changePassword}>
              <div className="space-y-2"><Label htmlFor="current-password">Kata sandi saat ini</Label><PasswordInput id="current-password" autoComplete="current-password" value={currentPassword} onChange={(event) => setCurrentPassword(event.target.value)} required /></div>
              <div className="space-y-2"><Label htmlFor="new-password">Kata sandi baru</Label><PasswordInput id="new-password" autoComplete="new-password" value={newPassword} onChange={(event) => setNewPassword(event.target.value)} minLength={8} maxLength={72} required /></div>
              <div className="space-y-2"><Label htmlFor="confirm-password">Ulangi kata sandi baru</Label><PasswordInput id="confirm-password" autoComplete="new-password" value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} minLength={8} maxLength={72} required /></div>
              <Button className="w-full sm:w-auto" disabled={saving}>{saving ? <Spinner /> : <Save />}{saving ? "Memperbarui..." : "Perbarui kata sandi"}</Button>
            </form>
          </CardContent>
        </Card>
        <Card className="h-fit">
          <CardHeader><div className="flex items-center justify-between"><div className="flex size-10 items-center justify-center rounded-xl bg-muted"><LockKeyhole className="size-5" /></div><Badge variant="secondary">Roadmap</Badge></div><CardTitle>Autentikasi dua langkah</CardTitle><CardDescription>2FA belum diaktifkan pada server ini.</CardDescription></CardHeader>
          <CardContent><Alert><ShieldAlert /><AlertTitle>Tidak ada keamanan palsu</AlertTitle><AlertDescription>Tombol aktivasi dinonaktifkan sampai penyimpanan secret TOTP, recovery codes, dan verifikasi login tersedia di backend.</AlertDescription></Alert><Button className="mt-4" variant="outline" disabled>Aktifkan 2FA</Button></CardContent>
        </Card>
      </div>
    </div>
  );
}
