"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { ArrowLeft, Copy, ExternalLink, Search, ShieldCheck, Trash2, UserRound, Users } from "lucide-react";
import { toast } from "sonner";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { DataPagination } from "@/components/data-pagination";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from "@/components/ui/alert-dialog";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiFetch, getErrorMessage, type ClientAssignmentsResponse, type OAuthClient, type OAuthClientAssignment } from "@/lib/api";

const PAGE_SIZE = 20;

export function ApplicationDetail({ clientID }: { clientID: string }) {
  const [client, setClient] = useState<OAuthClient | null>(null);
  const [assignments, setAssignments] = useState<OAuthClientAssignment[]>([]);
  const [loading, setLoading] = useState(true);
  const [errorMessage, setErrorMessage] = useState("");
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [totalPages, setTotalPages] = useState(1);
  const [reloadKey, setReloadKey] = useState(0);
  const [revokingUserID, setRevokingUserID] = useState<string | null>(null);
  const latestRequest = useRef(0);

  useEffect(() => {
    const timeout = window.setTimeout(() => {
      const value = search.trim();
      setDebouncedSearch((current) => {
        if (current === value) return current;
        setPage(1);
        setLoading(true);
        return value;
      });
    }, 350);
    return () => window.clearTimeout(timeout);
  }, [search]);

  useEffect(() => {
    const requestID = ++latestRequest.current;
    const controller = new AbortController();
    const query = new URLSearchParams({ page: String(page), page_size: String(PAGE_SIZE) });
    if (debouncedSearch) query.set("search", debouncedSearch);

    apiFetch<ClientAssignmentsResponse>(`/api/clients/${encodeURIComponent(clientID)}/assignments?${query.toString()}`, { signal: controller.signal })
      .then((data) => {
        if (requestID !== latestRequest.current) return;
        setErrorMessage("");
        setClient(data.client);
        setAssignments(Array.isArray(data.assignments) ? data.assignments : []);
        setTotal(typeof data.total === "number" ? data.total : 0);
        setTotalPages(Math.max(1, typeof data.total_pages === "number" ? data.total_pages : 1));
      })
      .catch((error) => {
        if (controller.signal.aborted || requestID !== latestRequest.current) return;
        setErrorMessage(getErrorMessage(error));
      })
      .finally(() => {
        if (!controller.signal.aborted && requestID === latestRequest.current) setLoading(false);
      });
    return () => controller.abort();
  }, [clientID, debouncedSearch, page, reloadKey]);

  async function copy(value: string) {
    try {
      await navigator.clipboard.writeText(value);
      toast.success("Disalin ke clipboard.");
    } catch {
      toast.error("Tidak dapat menyalin ke clipboard.");
    }
  }

  async function revokeAssignment(assignment: OAuthClientAssignment) {
    if (!client || assignment.user_id === client.owner_id || revokingUserID) return;
    setRevokingUserID(assignment.user_id);
    try {
      await apiFetch(`/api/clients/${encodeURIComponent(client.client_id)}/assignments/${encodeURIComponent(assignment.user_id)}`, { method: "DELETE" });
      const remainingOnPage = assignments.length - 1;
      const nextTotal = Math.max(0, total - 1);
      setAssignments((items) => items.filter((item) => item.user_id !== assignment.user_id));
      setTotal(nextTotal);
      setClient((current) => current ? { ...current, assignment_count: Math.max(0, current.assignment_count - 1) } : current);
      if (remainingOnPage === 0 && page > 1) {
        setLoading(true);
        setPage((value) => value - 1);
      } else {
        setReloadKey((value) => value + 1);
      }
      toast.success("Akses pengguna berhasil dicabut.");
    } catch (error) {
      toast.error(getErrorMessage(error));
    } finally {
      setRevokingUserID(null);
    }
  }

  if (loading && !client) return <ApplicationDetailLoading />;

  if (!client) {
    return (
      <div className="space-y-6">
        <Button variant="ghost" asChild className="-ml-3"><Link href="/dashboard/aplikasi"><ArrowLeft />Kembali ke aplikasi</Link></Button>
        <Alert variant="destructive"><AlertTitle>Detail aplikasi tidak dapat dimuat</AlertTitle><AlertDescription className="flex flex-wrap items-center justify-between gap-3"><span>{errorMessage || "Aplikasi tidak ditemukan."}</span><Button variant="outline" size="sm" onClick={() => { setErrorMessage(""); setLoading(true); setReloadKey((value) => value + 1); }}>Coba lagi</Button></AlertDescription></Alert>
      </div>
    );
  }

  return (
    <div className="space-y-7">
      <div className="flex items-start justify-between gap-4">
        <div className="flex min-w-0 items-start gap-2">
          <Button variant="ghost" size="icon" asChild className="mt-0.5 shrink-0" aria-label="Kembali ke daftar aplikasi"><Link href="/dashboard/aplikasi"><ArrowLeft /></Link></Button>
          <div className="min-w-0 space-y-1.5">
            <h1 className="truncate text-2xl font-semibold tracking-tight sm:text-3xl">{client.name}</h1>
            <p className="max-w-2xl text-sm leading-6 text-muted-foreground">{client.description || "Detail aplikasi OAuth yang terhubung ke IPNU IPPNU ID."}</p>
          </div>
        </div>
        <Badge className="mt-2 shrink-0" variant={client.status === "active" ? "default" : "destructive"}>{client.status === "active" ? "Aktif" : "Ditangguhkan"}</Badge>
      </div>

      <div className="grid gap-4 lg:grid-cols-[minmax(0,1.35fr)_minmax(280px,.65fr)]">
        <Card>
          <CardHeader className="border-b"><CardTitle>Informasi aplikasi</CardTitle><CardDescription>Identitas client dan alamat callback yang telah didaftarkan.</CardDescription></CardHeader>
          <CardContent className="grid gap-5 sm:grid-cols-2">
            <DetailItem label="Client ID" className="sm:col-span-2">
              <div className="flex min-w-0 items-center gap-2"><code className="min-w-0 flex-1 truncate rounded-md bg-muted p-2.5 text-xs">{client.client_id}</code><Button variant="outline" size="icon" onClick={() => copy(client.client_id)} aria-label="Salin client ID"><Copy /></Button></div>
            </DetailItem>
            <DetailItem label="Kebijakan akses"><Badge variant={client.access_policy === "assigned_only" ? "default" : "secondary"}>{client.access_policy === "assigned_only" ? "Pengguna ditugaskan" : "Semua akun aktif"}</Badge></DetailItem>
            <DetailItem label="Scope"><div className="flex flex-wrap gap-1.5">{client.allowed_scopes.map((scope) => <Badge key={scope} variant="outline">{scope}</Badge>)}</div></DetailItem>
            <DetailItem label="Redirect URI" className="sm:col-span-2">
              <div className="space-y-2">{client.redirect_uris.map((uri) => <div key={uri} className="flex items-start gap-2 rounded-lg border bg-muted/20 px-3 py-2.5"><ExternalLink className="mt-0.5 size-4 shrink-0 text-muted-foreground" /><code className="break-all text-xs leading-5">{uri}</code></div>)}</div>
            </DetailItem>
          </CardContent>
        </Card>

        <Card className="h-fit">
          <CardHeader><span className="flex size-10 items-center justify-center rounded-xl bg-primary/10 text-primary"><Users /></span><CardTitle className="mt-3">Akses pengguna</CardTitle><CardDescription>{client.access_policy === "assigned_only" ? "Hanya pengguna yang ditugaskan dapat menyelesaikan login." : "Semua akun aktif dan terverifikasi dapat masuk."}</CardDescription></CardHeader>
          <CardContent><p className="text-3xl font-semibold tabular-nums">{client.access_policy === "assigned_only" ? total : "Semua"}</p><p className="mt-1 text-xs text-muted-foreground">{client.access_policy === "assigned_only" ? "pengguna ditugaskan" : "akun aktif"}</p></CardContent>
        </Card>
      </div>

      {client.access_policy === "assigned_only" ? (
        <Card>
          <CardHeader className="gap-4 border-b sm:flex-row sm:items-center sm:justify-between">
            <div><CardTitle>Pengguna yang ditugaskan</CardTitle><CardDescription className="mt-1">Daftar akun yang diizinkan masuk ke aplikasi ini.</CardDescription></div>
            <div className="relative w-full sm:w-80"><Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" /><Input value={search} onChange={(event) => { setErrorMessage(""); setSearch(event.target.value); }} placeholder="Cari nama, email, atau UUID" className="pl-9" aria-label="Cari pengguna aplikasi" /></div>
          </CardHeader>
          <CardContent className="px-0" aria-busy={loading}>
            {errorMessage && <Alert variant="destructive" className="mx-4 mb-4 w-auto"><AlertTitle>Daftar pengguna gagal dimuat</AlertTitle><AlertDescription>{errorMessage}</AlertDescription></Alert>}
            {loading && assignments.length === 0 ? <div className="space-y-3 px-4">{[1, 2, 3].map((item) => <Skeleton key={item} className="h-16 w-full" />)}</div> : assignments.length === 0 ? (
              <Empty><EmptyHeader><EmptyMedia variant="icon"><UserRound /></EmptyMedia><EmptyTitle>Pengguna tidak ditemukan</EmptyTitle><EmptyDescription>{search ? "Coba kata kunci lain." : "Belum ada pengguna yang ditugaskan."}</EmptyDescription></EmptyHeader></Empty>
            ) : (
              <>
                <div className={loading ? "pointer-events-none opacity-60" : undefined}>
                  <div className="hidden md:block"><Table><TableHeader><TableRow><TableHead className="pl-4">Pengguna</TableHead><TableHead>ID pengguna</TableHead><TableHead>Keterangan</TableHead><TableHead className="pr-4 text-right">Aksi</TableHead></TableRow></TableHeader><TableBody>{assignments.map((assignment) => <AssignmentRow key={assignment.id} assignment={assignment} ownerID={client.owner_id} revoking={revokingUserID === assignment.user_id} onRevoke={revokeAssignment} />)}</TableBody></Table></div>
                  <div className="grid gap-3 px-4 md:hidden">{assignments.map((assignment) => <AssignmentCard key={assignment.id} assignment={assignment} ownerID={client.owner_id} revoking={revokingUserID === assignment.user_id} onRevoke={revokeAssignment} />)}</div>
                </div>
              </>
            )}
          </CardContent>
          {!loading && assignments.length > 0 && <DataPagination page={page} totalPages={totalPages} total={total} itemLabel="pengguna" onPageChange={(nextPage) => { setErrorMessage(""); setLoading(true); setPage(nextPage); }} />}
        </Card>
      ) : (
        <Alert><ShieldCheck /><AlertTitle>Tidak ada daftar assignment individual</AlertTitle><AlertDescription>Kebijakan aplikasi ini mengizinkan seluruh akun aktif dan terverifikasi. Ubah kebijakan menjadi “Pengguna ditugaskan” jika membutuhkan daftar akses terbatas.</AlertDescription></Alert>
      )}
    </div>
  );
}

function DetailItem({ label, className, children }: { label: string; className?: string; children: React.ReactNode }) {
  return <div className={className}><p className="mb-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">{label}</p>{children}</div>;
}

function AssignmentIdentity({ assignment }: { assignment: OAuthClientAssignment }) {
  const initials = assignment.name.split(" ").map((part) => part[0]).join("").slice(0, 2).toUpperCase();
  return <div className="flex min-w-0 items-center gap-3"><Avatar className="size-10"><AvatarImage src={assignment.avatar} alt={assignment.name} /><AvatarFallback>{initials || <UserRound />}</AvatarFallback></Avatar><div className="min-w-0"><p className="truncate font-medium">{assignment.name}</p><p className="truncate text-xs text-muted-foreground">{assignment.email}</p></div></div>;
}

function RevokeAssignmentButton({ assignment, revoking, onRevoke }: { assignment: OAuthClientAssignment; revoking: boolean; onRevoke: (assignment: OAuthClientAssignment) => Promise<void> }) {
  return (
    <AlertDialog>
      <AlertDialogTrigger asChild><Button type="button" size="sm" variant="outline" className="text-destructive hover:text-destructive" disabled={revoking}>{revoking ? <Spinner /> : <Trash2 />}Cabut</Button></AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader><AlertDialogTitle>Cabut akses pengguna?</AlertDialogTitle><AlertDialogDescription>{assignment.name} tidak dapat login lagi ke aplikasi ini. Token dan authorization code aktif pengguna juga akan dicabut.</AlertDialogDescription></AlertDialogHeader>
        <AlertDialogFooter><AlertDialogCancel>Batal</AlertDialogCancel><AlertDialogAction variant="destructive" onClick={() => onRevoke(assignment)}>Cabut akses</AlertDialogAction></AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

function AssignmentRow({ assignment, ownerID, revoking, onRevoke }: { assignment: OAuthClientAssignment; ownerID: string; revoking: boolean; onRevoke: (assignment: OAuthClientAssignment) => Promise<void> }) {
  const isOwner = assignment.user_id === ownerID;
  return <TableRow><TableCell className="pl-4"><AssignmentIdentity assignment={assignment} /></TableCell><TableCell><code className="text-xs text-muted-foreground">{assignment.user_id}</code></TableCell><TableCell>{isOwner ? <Badge variant="secondary">Pemilik aplikasi</Badge> : <Badge variant="outline">Ditugaskan</Badge>}</TableCell><TableCell className="pr-4 text-right">{!isOwner && <RevokeAssignmentButton assignment={assignment} revoking={revoking} onRevoke={onRevoke} />}</TableCell></TableRow>;
}

function AssignmentCard({ assignment, ownerID, revoking, onRevoke }: { assignment: OAuthClientAssignment; ownerID: string; revoking: boolean; onRevoke: (assignment: OAuthClientAssignment) => Promise<void> }) {
  const isOwner = assignment.user_id === ownerID;
  return <div className="rounded-xl border p-4"><div className="flex items-start justify-between gap-3"><AssignmentIdentity assignment={assignment} />{isOwner ? <Badge variant="secondary">Pemilik</Badge> : <Badge variant="outline">Ditugaskan</Badge>}</div><code className="mt-3 block break-all text-[11px] text-muted-foreground">{assignment.user_id}</code>{!isOwner && <div className="mt-4 flex justify-end border-t pt-3"><RevokeAssignmentButton assignment={assignment} revoking={revoking} onRevoke={onRevoke} /></div>}</div>;
}

function ApplicationDetailLoading() {
  return <div className="space-y-7"><div className="flex items-start gap-2"><Skeleton className="size-9 shrink-0" /><div className="min-w-0 flex-1 space-y-2"><Skeleton className="h-9 w-64 max-w-full" /><Skeleton className="h-4 w-96 max-w-full" /></div></div><div className="grid gap-4 lg:grid-cols-[minmax(0,1.35fr)_minmax(280px,.65fr)]"><Skeleton className="h-80 w-full" /><Skeleton className="h-52 w-full" /></div><Skeleton className="h-80 w-full" /></div>;
}
