"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { CalendarClock, DatabaseBackup, Download, LoaderCircle, LockKeyhole, Plus, RefreshCw, ShieldAlert } from "lucide-react";
import { toast } from "sonner";
import { useAuth } from "@/components/auth-provider";
import { BackupContentSkeleton } from "@/components/backup-loading-skeleton";
import { PageHeader } from "@/components/page-header";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { PasswordInput } from "@/components/ui/password-input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { API_URL, apiFetch, getErrorMessage } from "@/lib/api";
import { formatJakartaDateTime } from "@/lib/date-time";
import { readBackupSQL } from "@/lib/backup-download";

type Backup = {
  id: string; kind: string; status: string; phase: string; size_bytes: number;
  created_at: string; finished_at?: string; next_attempt_at?: string; last_error?: string; sha256?: string;
};
type BackupStatus = {
  ready: boolean; issue?: string; retention: number; next_run_at?: string;
  download_ready: boolean; download_issue?: string;
  last_successful?: string; maintenance_error?: string; backups: Backup[]; total_successful: number;
};
type Action = { type: "create" } | { type: "download"; backup: Backup };
const statusLabels: Record<string, string> = { queued: "Dalam antrean", running: "Sedang berjalan", succeeded: "Berhasil", failed: "Menunggu ulang", pruning: "Menghapus file lama" };
const phaseLabels: Record<string, string> = { exporting: "Mengekspor & mengenkripsi", uploading: "Mengunggah ke R2", verifying: "Memeriksa integritas" };
function formatSize(bytes: number) {
  if (bytes <= 0) return "—";
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), 3);
  return `${(bytes / 1024 ** index).toLocaleString("id-ID", { maximumFractionDigits: 2 })} ${["B", "KiB", "MiB", "GiB"][index]}`;
}
function date(value?: string) { return value ? formatJakartaDateTime(value) : "—"; }

export default function BackupDatabasePage() {
  const { user } = useAuth();
  const [status, setStatus] = useState<BackupStatus | null>(null);
  const [error, setError] = useState("");
  const [refreshing, setRefreshing] = useState(false);
  const [action, setAction] = useState<Action | null>(null);
  const [password, setPassword] = useState("");
  const [actionError, setActionError] = useState("");
  const [busy, setBusy] = useState(false);
  const requestRef = useRef<AbortController | null>(null);
  const operationRef = useRef<AbortController | null>(null);
  const running = status?.backups.some((backup) => ["queued", "running"].includes(backup.status)) ?? false;
  // Like "Tambah aplikasi", the header action only opens a dialog. Keep its
  // appearance independent of fetching; gate the actual submission below.
  const createIssue = error
    ? "Status backup belum dapat diperbarui. Coba muat ulang status sebelum melanjutkan."
    : !status
      ? "Status backup sedang dimuat. Tunggu sebentar sebelum memulai backup."
      : !status.ready
        ? status.issue || "Konfigurasi backup belum siap."
        : running
          ? "Backup lain masih menunggu atau berjalan. Tunggu sampai selesai."
          : status.maintenance_error || "";

  const refresh = useCallback(async () => {
    requestRef.current?.abort();
    const controller = new AbortController(); requestRef.current = controller;
    setRefreshing(true);
    try {
      const data = await apiFetch<BackupStatus>("/api/admin/backups", { signal: controller.signal });
      if (!controller.signal.aborted) { setStatus(data); setError(""); }
    } catch (error) {
      if (!controller.signal.aborted) setError(getErrorMessage(error));
    } finally { if (!controller.signal.aborted) setRefreshing(false); }
  }, []);

  useEffect(() => {
    if (user?.role !== "super_admin") return;
    // Poll only while this page is visible. No background requests on other menus.
    const tick = () => { if (document.visibilityState === "visible") void refresh(); };
    const initial = window.setTimeout(tick, 0);
    const interval = window.setInterval(tick, running ? 5000 : 30000);
    document.addEventListener("visibilitychange", tick);
    return () => {
      window.clearTimeout(initial); window.clearInterval(interval);
      document.removeEventListener("visibilitychange", tick); requestRef.current?.abort();
    };
  }, [refresh, running, user?.role]);
  useEffect(() => () => operationRef.current?.abort(), []);

  function openAction(next: Action | null) {
    if (busy) return;
    setPassword(""); setActionError(""); setAction(next);
  }
  async function confirm(event: React.FormEvent) {
    event.preventDefault();
    if (!action || busy) return;
    if (action.type === "create" && createIssue) {
      setActionError(createIssue);
      return;
    }
    const controller = new AbortController(); operationRef.current = controller;
    setBusy(true); setActionError("");
    const body = JSON.stringify({ password });
    setPassword("");
    try {
      if (action.type === "create") {
        await apiFetch("/api/admin/backups", { method: "POST", body, signal: controller.signal });
        toast.success("Backup masuk antrean. Anda boleh meninggalkan halaman ini.");
      } else {
        const response = await fetch(`${API_URL}/api/admin/backups/${action.backup.id}/download`, {
          method: "POST", credentials: "include", cache: "no-store", headers: { "Content-Type": "application/json" }, body, signal: controller.signal,
        });
        const blob = await readBackupSQL(response);
        const url = URL.createObjectURL(blob);
        const link = document.createElement("a"); link.href = url;
        link.download = `backup-${action.backup.id}.sql`; link.click();
        window.setTimeout(() => URL.revokeObjectURL(url), 60000);
        toast.success("File .sql siap diimpor ke PostgreSQL tanpa dekripsi manual. Simpan file secara privat.");
      }
      if (!controller.signal.aborted) { setAction(null); void refresh(); }
    } catch (error) { if (!controller.signal.aborted) setActionError(getErrorMessage(error)); }
    finally { if (!controller.signal.aborted) setBusy(false); }
  }

  if (user?.role !== "super_admin") return <Alert variant="destructive"><ShieldAlert /><AlertTitle>Akses ditolak</AlertTitle><AlertDescription>Backup database hanya tersedia untuk super admin.</AlertDescription></Alert>;

  return <div className="min-w-0 space-y-6">
    <PageHeader title="Backup database" description="Backup manual dan otomatis dengan unduhan SQL yang siap diimpor ke PostgreSQL."
      action={<Button onClick={() => openAction({ type: "create" })}><Plus />Buat backup</Button>} />

    {error && <Alert variant="destructive"><ShieldAlert /><AlertTitle>Status belum dapat diperbarui</AlertTitle><AlertDescription>{error}<Button className="mt-2 w-fit" size="sm" variant="outline" disabled={refreshing} onClick={() => void refresh()}>Coba lagi</Button></AlertDescription></Alert>}
    {status && !status.ready && <Alert><LockKeyhole /><AlertTitle>Konfigurasi backup belum siap</AlertTitle><AlertDescription>{status.issue || "Periksa konfigurasi backup pada backend."} Kunci dikelola pada konfigurasi backend privat, bukan di halaman ini.</AlertDescription></Alert>}
    {status?.ready && !status.download_ready && <Alert><LockKeyhole /><AlertTitle>Unduhan SQL belum siap</AlertTitle><AlertDescription>{status.download_issue || "Pengelola server perlu melengkapi konfigurasi unduhan SQL di backend. Backup tetap dapat dibuat."} Konfigurasi ini dikelola di server, bukan dimasukkan saat impor file SQL.</AlertDescription></Alert>}
    {status?.maintenance_error && <Alert variant="destructive"><ShieldAlert /><AlertTitle>Retensi perlu diperiksa</AlertTitle><AlertDescription>{status.maintenance_error}</AlertDescription></Alert>}

    {!status && (!error || refreshing) ? <BackupContentSkeleton /> : status && <>
      <section aria-label="Jadwal dan penyimpanan" className="grid gap-5 rounded-xl border bg-card p-5 text-sm sm:grid-cols-2 lg:grid-cols-3">
        <div className="space-y-1.5"><p className="flex items-center gap-2 font-medium"><CalendarClock className="size-4 text-primary" />Minggu, 02.00 WIB</p><p className="text-muted-foreground">{status.ready ? `Berikutnya: ${date(status.next_run_at)}` : "Jadwal aktif setelah konfigurasi siap."}</p></div>
        <div className="space-y-1.5"><p className="font-medium">Maksimal {status.retention} backup</p><p className="text-muted-foreground">{status.total_successful} file berhasil disimpan · manual & otomatis</p></div>
        <div className="space-y-1.5"><p className="font-medium">Backup terakhir berhasil</p><p className="text-muted-foreground">{status.last_successful ? date(status.last_successful) : "Belum ada backup berhasil."}</p></div>
      </section>

      <Card className="min-w-0 overflow-hidden">
        <CardHeader className="flex flex-row items-start justify-between gap-3 border-b pb-5"><div><CardTitle>Riwayat backup</CardTitle><CardDescription className="mt-1">File lama dihapus hanya setelah backup baru berhasil diverifikasi.</CardDescription></div><Button variant="outline" size="icon" aria-label="Perbarui riwayat backup" disabled={refreshing} onClick={() => void refresh()}><RefreshCw className={refreshing ? "animate-spin motion-reduce:animate-none" : ""} /></Button></CardHeader>
        <CardContent className="p-0">
          {status.backups.length === 0 ? <div className="flex flex-col items-center px-6 py-14 text-center"><DatabaseBackup className="mb-4 size-8 text-muted-foreground" /><h2 className="font-medium">Belum ada cadangan database</h2><p className="mt-2 max-w-md text-sm leading-6 text-muted-foreground">{status.ready ? "Buat backup pertama sekarang, atau tunggu jadwal otomatis hari Minggu." : "Siapkan konfigurasi backend untuk mulai membuat backup."}</p></div> : <Table>
            <TableHeader><TableRow><TableHead className="pl-5">Waktu & sumber</TableHead><TableHead>Status</TableHead><TableHead>Ukuran arsip R2</TableHead><TableHead className="pr-5 text-right">File</TableHead></TableRow></TableHeader>
            <TableBody>{status.backups.map((item) => <TableRow key={item.id}>
              <TableCell className="py-4 pl-5 align-top"><p className="font-medium tabular-nums">{date(item.created_at)}</p><p className="mt-1 text-xs text-muted-foreground">{item.kind === "manual" ? "Manual" : "Otomatis"}</p></TableCell>
              <TableCell className="min-w-48 max-w-80 py-4 align-top whitespace-normal"><Badge variant={item.status === "failed" ? "destructive" : item.status === "succeeded" ? "secondary" : "outline"}>{item.status === "running" && <LoaderCircle className="animate-spin motion-reduce:animate-none" />}{statusLabels[item.status] ?? item.status}</Badge>
                {item.status === "running" && <p className="mt-1.5 text-xs text-muted-foreground" role="status">{phaseLabels[item.phase] ?? "Memproses backup"}</p>}
                {item.last_error && <p className="mt-2 text-xs leading-5 text-muted-foreground">{item.last_error}</p>}
                {item.status === "failed" && item.next_attempt_at && <p className="mt-1 text-xs text-muted-foreground">Coba otomatis: {date(item.next_attempt_at)}</p>}
              </TableCell>
              <TableCell className="py-4 align-top tabular-nums">{item.status === "succeeded" ? formatSize(item.size_bytes) : "—"}</TableCell>
              <TableCell className="py-4 pr-5 text-right align-top"><Button variant="outline" size="sm" disabled={busy || item.status !== "succeeded" || !status.download_ready || !!error} aria-label={`Unduh SQL backup ${date(item.created_at)}`} onClick={() => openAction({ type: "download", backup: item })}><Download />Unduh SQL</Button>
                {item.status === "succeeded" && item.sha256 && <details className="mt-2 text-xs text-muted-foreground"><summary className="cursor-pointer">SHA-256 arsip R2</summary><code className="mt-2 block max-w-48 break-all text-left whitespace-normal">{item.sha256}</code><p className="mt-1 max-w-48 text-left">Checksum arsip terenkripsi, bukan file SQL unduhan.</p></details>}
              </TableCell>
            </TableRow>)}</TableBody>
          </Table>}
        </CardContent>
      </Card>
      <div className="flex gap-2.5 text-sm leading-6 text-muted-foreground"><LockKeyhole className="mt-1 size-4 shrink-0" /><p>Unduh file .sql dengan konfirmasi kata sandi akun SSO, lalu impor ke PostgreSQL menggunakan psql atau native restore yang mendukung SQL. Tidak perlu access key R2, recovery key, atau dekripsi manual saat impor. File SQL tidak terenkripsi; simpan privat dan pastikan database tujuan benar karena tabel dalam backup akan diganti. Arsip di R2 tetap terenkripsi; backend menyiapkan SQL secara otomatis.</p></div>
    </>}

    <Dialog open={action !== null} onOpenChange={(open) => { if (!open) openAction(null); }}>
      <DialogContent showCloseButton={!busy} onInteractOutside={(event) => { if (busy) event.preventDefault(); }} onEscapeKeyDown={(event) => { if (busy) event.preventDefault(); }}>
        <DialogHeader><DialogTitle>{action?.type === "download" ? "Unduh SQL database" : "Buat backup database"}</DialogTitle><DialogDescription>{action?.type === "download" ? "Konfirmasi kata sandi akun SSO untuk mengunduh file .sql. File langsung diimpor ke PostgreSQL tanpa recovery key atau dekripsi manual. SQL tidak terenkripsi; simpan privat dan periksa tujuan impor karena tabel dalam backup akan diganti." : "Masukkan kata sandi akun SSO Anda. Backup berjalan di latar belakang; setelah berhasil, file dapat diunduh sebagai SQL."}</DialogDescription></DialogHeader>
        <form onSubmit={confirm} className="space-y-4"><div className="space-y-2"><Label htmlFor="backup-password">Kata sandi akun SSO</Label><PasswordInput id="backup-password" autoComplete="current-password" required disabled={busy} value={password} onChange={(event) => setPassword(event.target.value)} aria-invalid={!!actionError} aria-describedby={actionError ? "backup-action-error" : undefined} /></div>
          {action?.type === "create" && createIssue && <p role="status" className="text-sm text-muted-foreground">{createIssue}</p>}
          {actionError && <p id="backup-action-error" role="alert" className="text-sm text-destructive">{actionError}</p>}
          <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end"><Button type="button" variant="outline" disabled={busy} onClick={() => openAction(null)}>Batal</Button><Button type="submit" disabled={busy || !password || (action?.type === "create" && !!createIssue)}>{busy && <LoaderCircle className="animate-spin motion-reduce:animate-none" />}{busy ? (action?.type === "download" ? "Menyiapkan SQL…" : "Memproses…") : action?.type === "download" ? "Unduh SQL" : "Mulai backup"}</Button></div>
        </form>
      </DialogContent>
    </Dialog>
  </div>;
}
