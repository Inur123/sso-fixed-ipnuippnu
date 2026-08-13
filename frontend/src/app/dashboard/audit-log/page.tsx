"use client";

import { useEffect, useState } from "react";
import {
  Activity,
  ChevronLeft,
  ChevronRight,
  Clock3,
  Filter,
  Globe2,
  Search,
  ShieldAlert,
  UserRound,
} from "lucide-react";
import { toast } from "sonner";

import { useAuth } from "@/components/auth-provider";
import { PageHeader } from "@/components/page-header";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiFetch, getErrorMessage } from "@/lib/api";

const PAGE_SIZE = 20;

const ACTION_OPTIONS = [
  { value: "all", label: "Semua aktivitas" },
  { value: "user.register", label: "Registrasi pengguna" },
  { value: "email.verify", label: "Verifikasi email" },
  { value: "auth.login", label: "Login" },
  { value: "auth.login_failed", label: "Login gagal" },
  { value: "auth.logout", label: "Logout" },
  { value: "user.status_update", label: "Perubahan status akun" },
  { value: "user.role_update", label: "Perubahan role" },
  { value: "user.profile_update", label: "Perubahan profil" },
  { value: "user.password_update", label: "Perubahan kata sandi" },
  { value: "oauth.client_create", label: "Pembuatan aplikasi OAuth" },
  { value: "oauth.client_delete", label: "Penghapusan aplikasi OAuth" },
  { value: "oauth.client_secret_regenerate", label: "Regenerate client secret" },
  { value: "oauth.client_update", label: "Memperbarui aplikasi" },
  { value: "oauth.client_assignment_update", label: "Memperbarui akses aplikasi" },
  { value: "oauth.client_assignment_delete", label: "Mencabut akses aplikasi" },
  { value: "user.delete", label: "Menghapus pengguna" },
  { value: "oauth.consent", label: "Persetujuan OAuth" },
  { value: "oauth.grant", label: "Pemberian akses OAuth" },
  { value: "oauth.token_revoke", label: "Pencabutan token OAuth" },
  { value: "oauth.connection_revoke", label: "Pencabutan koneksi OAuth" },
] as const;

const actionLabels = Object.fromEntries(ACTION_OPTIONS.map((option) => [option.value, option.label]));

interface AuditLog {
  id: string;
  actor_id?: string | null;
  actor_name?: string | null;
  actor_email?: string | null;
  action: string;
  target_type: string;
  target_id?: string | null;
  description: string;
  ip_address: string;
  created_at: string;
}

interface AuditLogsResponse {
  logs: AuditLog[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}

function actionLabel(action: string) {
  return actionLabels[action] ?? action.replaceAll(/[._-]+/g, " ");
}

function formatDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";
  return date.toLocaleString("id-ID", {
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function actorLabel(log: AuditLog) {
  return log.actor_name || log.actor_email || "Sistem";
}

export default function AuditLogPage() {
  const { user } = useAuth();
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [action, setAction] = useState("all");
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [totalPages, setTotalPages] = useState(1);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setDebouncedSearch(search.trim());
      setPage(1);
    }, 350);
    return () => window.clearTimeout(timer);
  }, [search]);

  useEffect(() => {
    if (user?.role !== "super_admin") return;
    let active = true;
    const query = new URLSearchParams({
      page: String(page),
      page_size: String(PAGE_SIZE),
    });
    if (debouncedSearch) query.set("search", debouncedSearch);
    if (action !== "all") query.set("action", action);

    apiFetch<AuditLogsResponse>(`/api/admin/audit-logs?${query.toString()}`)
      .then((data) => {
        if (!active) return;
        setLogs(data.logs ?? []);
        setTotal(data.total ?? 0);
        const nextTotalPages = Math.max(1, data.total_pages ?? 1);
        setTotalPages(nextTotalPages);
        if (page > nextTotalPages) setPage(nextTotalPages);
      })
      .catch((error) => {
        if (active) toast.error(getErrorMessage(error));
      })
      .finally(() => {
        if (active) setLoading(false);
      });

    return () => {
      active = false;
    };
  }, [action, debouncedSearch, page, user?.role]);

  if (user?.role !== "super_admin") {
    return (
      <Alert variant="destructive">
        <ShieldAlert />
        <AlertTitle>Akses ditolak</AlertTitle>
        <AlertDescription>Audit log hanya tersedia untuk super admin.</AlertDescription>
      </Alert>
    );
  }

  return (
    <div className="min-w-0 space-y-7">
      <PageHeader
        title="Audit log"
        description="Telusuri aktivitas keamanan dan perubahan administratif pada IPNU IPPNU ID."
      />

      <Card className="min-w-0 overflow-hidden">
        <CardHeader className="gap-4 border-b pb-5">
          <div className="min-w-0">
            <CardTitle>Riwayat aktivitas</CardTitle>
            <CardDescription className="mt-1">
              Catatan bersifat hanya-baca dan diurutkan dari aktivitas terbaru.
            </CardDescription>
          </div>
          <div className="grid min-w-0 gap-3 sm:grid-cols-[minmax(0,1fr)_16rem]">
            <div className="relative min-w-0">
              <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                className="w-full pl-9"
              value={search}
                onChange={(event) => {
                  setLoading(true);
                  setSearch(event.target.value);
                }}
                placeholder="Cari aktor, target, IP, atau deskripsi..."
                aria-label="Cari audit log"
              />
            </div>
            <Select
              value={action}
              onValueChange={(value) => {
                setLoading(true);
                setAction(value);
                setPage(1);
              }}
            >
              <SelectTrigger className="w-full" aria-label="Filter jenis aktivitas">
                <Filter className="text-muted-foreground" />
                <SelectValue placeholder="Semua aktivitas" />
              </SelectTrigger>
              <SelectContent position="popper" align="end">
                {ACTION_OPTIONS.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </CardHeader>

        <CardContent className="min-w-0 px-0">
          {loading ? (
            <div className="space-y-3 px-4">
              {[1, 2, 3, 4, 5].map((item) => <Skeleton key={item} className="h-20 w-full" />)}
            </div>
          ) : logs.length === 0 ? (
            <Empty>
              <EmptyHeader>
                <EmptyMedia variant="icon"><Activity /></EmptyMedia>
                <EmptyTitle>Aktivitas tidak ditemukan</EmptyTitle>
                <EmptyDescription>Coba hapus filter atau gunakan kata kunci pencarian lain.</EmptyDescription>
              </EmptyHeader>
            </Empty>
          ) : (
            <>
              <div className="hidden md:block">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="pl-4">Waktu</TableHead>
                      <TableHead>Aktor</TableHead>
                      <TableHead>Aktivitas</TableHead>
                      <TableHead>Detail</TableHead>
                      <TableHead className="pr-4">IP</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {logs.map((log) => <AuditTableRow key={log.id} log={log} />)}
                  </TableBody>
                </Table>
              </div>
              <div className="grid min-w-0 gap-3 px-4 md:hidden">
                {logs.map((log) => <AuditMobileCard key={log.id} log={log} />)}
              </div>
            </>
          )}
        </CardContent>

        {!loading && logs.length > 0 && (
          <div className="flex flex-col gap-3 border-t px-4 pt-4 sm:flex-row sm:items-center sm:justify-between">
            <p className="text-xs text-muted-foreground">Halaman {page} dari {totalPages} · {total} aktivitas</p>
            <div className="grid grid-cols-2 gap-2 sm:flex">
              <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => { setLoading(true); setPage((value) => Math.max(1, value - 1)); }}>
                <ChevronLeft />Sebelumnya
              </Button>
              <Button variant="outline" size="sm" disabled={page >= totalPages} onClick={() => { setLoading(true); setPage((value) => Math.min(totalPages, value + 1)); }}>
                Berikutnya<ChevronRight />
              </Button>
            </div>
          </div>
        )}
      </Card>
    </div>
  );
}

function AuditTableRow({ log }: { log: AuditLog }) {
  return (
    <TableRow>
      <TableCell className="w-40 pl-4 text-xs whitespace-normal text-muted-foreground">{formatDate(log.created_at)}</TableCell>
      <TableCell className="max-w-48 whitespace-normal">
        <p className="truncate text-sm font-medium">{actorLabel(log)}</p>
        {log.actor_name && log.actor_email && <p className="truncate text-xs text-muted-foreground">{log.actor_email}</p>}
      </TableCell>
      <TableCell className="whitespace-normal"><Badge variant="secondary">{actionLabel(log.action)}</Badge></TableCell>
      <TableCell className="max-w-md whitespace-normal">
        <p className="break-words text-sm leading-5">{log.description || "—"}</p>
        <p className="mt-1 truncate text-xs text-muted-foreground">
          {log.target_type || "sistem"}{log.target_id ? ` · ${log.target_id}` : ""}
        </p>
      </TableCell>
      <TableCell className="pr-4 font-mono text-xs whitespace-normal text-muted-foreground">{log.ip_address || "—"}</TableCell>
    </TableRow>
  );
}

function AuditMobileCard({ log }: { log: AuditLog }) {
  return (
    <article className="min-w-0 rounded-xl border p-4">
      <div className="flex min-w-0 flex-wrap items-start justify-between gap-2">
        <Badge variant="secondary" className="max-w-full whitespace-normal">{actionLabel(log.action)}</Badge>
        <span className="flex items-center gap-1.5 text-xs text-muted-foreground"><Clock3 className="size-3.5" />{formatDate(log.created_at)}</span>
      </div>
      <p className="mt-3 break-words text-sm leading-6">{log.description || "—"}</p>
      <dl className="mt-4 grid min-w-0 gap-2 border-t pt-3 text-xs">
        <div className="flex min-w-0 items-center gap-2">
          <UserRound className="size-3.5 shrink-0 text-muted-foreground" />
          <dt className="sr-only">Aktor</dt>
          <dd className="min-w-0 truncate">{actorLabel(log)}{log.actor_name && log.actor_email ? ` · ${log.actor_email}` : ""}</dd>
        </div>
        <div className="flex min-w-0 items-center gap-2">
          <Activity className="size-3.5 shrink-0 text-muted-foreground" />
          <dt className="sr-only">Target</dt>
          <dd className="min-w-0 truncate text-muted-foreground">{log.target_type || "sistem"}{log.target_id ? ` · ${log.target_id}` : ""}</dd>
        </div>
        <div className="flex min-w-0 items-center gap-2">
          <Globe2 className="size-3.5 shrink-0 text-muted-foreground" />
          <dt className="sr-only">Alamat IP</dt>
          <dd className="min-w-0 truncate font-mono text-muted-foreground">{log.ip_address || "—"}</dd>
        </div>
      </dl>
    </article>
  );
}
