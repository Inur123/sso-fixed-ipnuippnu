"use client";

import { useEffect, useRef, useState } from "react";
import { Ban, BadgeCheck, CircleUserRound, MoreHorizontal, Search, ShieldAlert, ShieldCheck, Trash2, UserCheck, UsersRound } from "lucide-react";
import { toast } from "sonner";

import { useAuth } from "@/components/auth-provider";
import { DataPagination } from "@/components/data-pagination";
import { PageHeader } from "@/components/page-header";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogMedia, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiFetch, getErrorMessage, type Role, type User } from "@/lib/api";

const PAGE_SIZE = 20;

interface UsersResponse {
  users: User[];
  total?: number;
  page?: number;
  page_size?: number;
  total_pages?: number;
}

export default function PenggunaPage() {
  const { user: currentUser, loading: authLoading } = useAuth();
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [errorMessage, setErrorMessage] = useState("");
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [totalPages, setTotalPages] = useState(1);
  const [reloadKey, setReloadKey] = useState(0);
  const [pendingStatusUser, setPendingStatusUser] = useState<User | null>(null);
  const [pendingDeleteUser, setPendingDeleteUser] = useState<User | null>(null);
  const [deletingUser, setDeletingUser] = useState(false);
  const latestRequest = useRef(0);
  const latestDebouncedSearch = useRef("");

  useEffect(() => {
    const timer = window.setTimeout(() => {
      const normalizedSearch = search.trim();
      if (normalizedSearch === latestDebouncedSearch.current) return;
      latestDebouncedSearch.current = normalizedSearch;
      setLoading(true);
      setErrorMessage("");
      setDebouncedSearch(normalizedSearch);
      setPage(1);
    }, 350);
    return () => window.clearTimeout(timer);
  }, [search]);

  useEffect(() => {
    if (currentUser?.role !== "super_admin") return;
    const requestID = ++latestRequest.current;
    const controller = new AbortController();
    let movingToValidPage = false;
    const query = new URLSearchParams({ page: String(page), page_size: String(PAGE_SIZE) });
    if (debouncedSearch) query.set("search", debouncedSearch);
    apiFetch<UsersResponse>(`/api/admin/users?${query.toString()}`, { signal: controller.signal })
      .then((data) => {
        if (requestID !== latestRequest.current) return;
        const nextUsers = Array.isArray(data.users) ? data.users : [];
        const nextTotal = typeof data.total === "number" ? data.total : nextUsers.length;
        const nextTotalPages = Math.max(1, typeof data.total_pages === "number" ? data.total_pages : 1);

        // Jika data terakhir pada halaman akhir terhapus, pindah ke halaman
        // valid dahulu dan biarkan efek berikutnya mengambil datanya.
        if (page > nextTotalPages) {
          movingToValidPage = true;
          setPage(nextTotalPages);
          return;
        }

        setUsers(nextUsers);
        setTotal(nextTotal);
        setTotalPages(nextTotalPages);
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted || requestID !== latestRequest.current) return;
        const message = getErrorMessage(error);
        setErrorMessage(message);
        toast.error(message);
      })
      .finally(() => {
        if (!movingToValidPage && !controller.signal.aborted && requestID === latestRequest.current) setLoading(false);
      });
    return () => controller.abort();
  }, [currentUser?.role, debouncedSearch, page, reloadKey]);

  if (authLoading) {
    return <div className="space-y-3"><Skeleton className="h-9 w-52" /><Skeleton className="h-72 w-full" /></div>;
  }

  if (currentUser?.role !== "super_admin") {
    return <Alert variant="destructive"><ShieldAlert /><AlertTitle>Akses ditolak</AlertTitle><AlertDescription>Manajemen pengguna hanya tersedia untuk super admin.</AlertDescription></Alert>;
  }

  async function updateRole(user: User, role: Role) {
    try {
      await apiFetch(`/api/admin/users/${user.id}/role`, { method: "PATCH", body: JSON.stringify({ role }) });
      setUsers((items) => items.map((item) => item.id === user.id ? { ...item, role } : item));
      toast.success(`Role ${user.name} diperbarui menjadi ${role === "super_admin" ? "super admin" : "anggota"}.`);
    } catch (error) {
      toast.error(getErrorMessage(error));
    }
  }

  async function updateStatus() {
    if (!pendingStatusUser) return;
    const nextStatus = !pendingStatusUser.is_active;
    try {
      await apiFetch(`/api/admin/users/${pendingStatusUser.id}/status`, {
        method: "PATCH",
        body: JSON.stringify({ is_active: nextStatus }),
      });
      setUsers((items) => items.map((item) => item.id === pendingStatusUser.id ? { ...item, is_active: nextStatus } : item));
      toast.success(nextStatus ? `Akun ${pendingStatusUser.name} diaktifkan.` : `Akun ${pendingStatusUser.name} dinonaktifkan dan aksesnya dicabut.`);
    } catch (error) {
      toast.error(getErrorMessage(error));
    } finally {
      setPendingStatusUser(null);
    }
  }

  async function deleteUser() {
    if (!pendingDeleteUser) return;
    setDeletingUser(true);
    try {
      await apiFetch(`/api/admin/users/${pendingDeleteUser.id}`, { method: "DELETE" });
      toast.success(`Data ${pendingDeleteUser.name} berhasil dihapus permanen.`);
      setPendingDeleteUser(null);
      setLoading(true);
      setReloadKey((value) => value + 1);
    } catch (error) {
      toast.error(getErrorMessage(error));
    } finally {
      setDeletingUser(false);
    }
  }

  const activeCount = users.filter((user) => user.is_active).length;
  const verifiedCount = users.filter((user) => user.email_verified).length;

  return (
    <div className="space-y-7">
      <PageHeader title="Manajemen pengguna" description="Lihat seluruh akun, periksa status verifikasi, atur role, dan kendalikan akses login anggota." />

      <div className="grid gap-4 sm:grid-cols-3">
        <Card size="sm"><CardContent className="flex items-center gap-3"><span className="flex size-10 items-center justify-center rounded-xl bg-primary/10 text-primary"><UsersRound className="size-5" /></span><div><p className="text-xs text-muted-foreground">Total pengguna</p><p className="text-xl font-semibold tabular-nums">{loading ? "—" : total}</p></div></CardContent></Card>
        <Card size="sm"><CardContent className="flex items-center gap-3"><span className="flex size-10 items-center justify-center rounded-xl bg-emerald-500/10 text-emerald-700"><UserCheck className="size-5" /></span><div><p className="text-xs text-muted-foreground">Aktif di halaman ini</p><p className="text-xl font-semibold tabular-nums">{loading ? "—" : activeCount}</p></div></CardContent></Card>
        <Card size="sm"><CardContent className="flex items-center gap-3"><span className="flex size-10 items-center justify-center rounded-xl bg-sky-500/10 text-sky-700"><BadgeCheck className="size-5" /></span><div><p className="text-xs text-muted-foreground">Email terverifikasi</p><p className="text-xl font-semibold tabular-nums">{loading ? "—" : verifiedCount}</p></div></CardContent></Card>
      </div>

      <Card>
        <CardHeader className="gap-4 border-b pb-4 sm:flex-row sm:items-center sm:justify-between">
          <div><CardTitle>Direktori akun</CardTitle><CardDescription className="mt-1">Data dimuat langsung dari server dan dapat dicari berdasarkan nama atau email.</CardDescription></div>
          <div className="relative w-full sm:w-80"><Search className="absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" /><Input className="pl-9" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Cari nama atau email..." aria-label="Cari pengguna" /></div>
        </CardHeader>
        <CardContent className="px-0" aria-busy={loading}>
          {errorMessage && <Alert variant="destructive" className="mx-4 mb-4 w-auto"><ShieldAlert /><AlertTitle>Data pengguna gagal dimuat</AlertTitle><AlertDescription className="flex flex-wrap items-center justify-between gap-3"><span>{errorMessage}</span><Button size="sm" variant="outline" onClick={() => { setLoading(true); setErrorMessage(""); setReloadKey((value) => value + 1); }}>Coba lagi</Button></AlertDescription></Alert>}
          {loading && users.length === 0 ? <div className="space-y-3 px-4">{[1, 2, 3, 4].map((item) => <Skeleton key={item} className="h-16 w-full" />)}</div> : users.length === 0 ? <Empty><EmptyHeader><EmptyMedia variant="icon"><UsersRound /></EmptyMedia><EmptyTitle>Pengguna tidak ditemukan</EmptyTitle><EmptyDescription>Coba gunakan kata kunci lain atau periksa kembali pencarian Anda.</EmptyDescription></EmptyHeader></Empty> : (
            <>
              <div className={loading ? "pointer-events-none opacity-60 transition-opacity" : "transition-opacity"}>
              <div className="hidden overflow-x-auto md:block">
                <Table>
                  <TableHeader><TableRow><TableHead className="pl-4">Pengguna</TableHead><TableHead>Status akun</TableHead><TableHead>Verifikasi email</TableHead><TableHead>Role</TableHead><TableHead className="hidden xl:table-cell">Terdaftar</TableHead><TableHead className="w-14 pr-4 text-right">Aksi</TableHead></TableRow></TableHeader>
                  <TableBody>{users.map((user) => <UserTableRow key={user.id} user={user} isCurrent={user.id === currentUser.id} onRole={updateRole} onStatus={() => setPendingStatusUser(user)} onDelete={() => setPendingDeleteUser(user)} />)}</TableBody>
                </Table>
              </div>
              <div className="grid gap-3 px-4 md:hidden">{users.map((user) => <UserMobileCard key={user.id} user={user} isCurrent={user.id === currentUser.id} onRole={updateRole} onStatus={() => setPendingStatusUser(user)} onDelete={() => setPendingDeleteUser(user)} />)}</div>
              </div>
            </>
          )}
        </CardContent>
        {!loading && users.length > 0 && <DataPagination page={page} totalPages={totalPages} total={total} itemLabel="akun" onPageChange={(nextPage) => { setLoading(true); setErrorMessage(""); setPage(nextPage); }} />}
      </Card>

      <AlertDialog open={Boolean(pendingStatusUser)} onOpenChange={(open) => { if (!open) setPendingStatusUser(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogMedia className={pendingStatusUser?.is_active ? "bg-destructive/10 text-destructive" : "bg-primary/10 text-primary"}>{pendingStatusUser?.is_active ? <Ban /> : <UserCheck />}</AlertDialogMedia>
            <AlertDialogTitle>{pendingStatusUser?.is_active ? "Nonaktifkan akun?" : "Aktifkan akun?"}</AlertDialogTitle>
            <AlertDialogDescription>{pendingStatusUser?.is_active ? `Akun ${pendingStatusUser.name} tidak dapat login dan seluruh sesi serta token aplikasinya akan dicabut.` : `Akun ${pendingStatusUser?.name} dapat kembali login setelah diaktifkan.`}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter><AlertDialogCancel>Batal</AlertDialogCancel><AlertDialogAction variant={pendingStatusUser?.is_active ? "destructive" : "default"} onClick={updateStatus}>{pendingStatusUser?.is_active ? "Nonaktifkan" : "Aktifkan"}</AlertDialogAction></AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={Boolean(pendingDeleteUser)} onOpenChange={(open) => { if (!open && !deletingUser) setPendingDeleteUser(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogMedia className="bg-destructive/10 text-destructive"><Trash2 /></AlertDialogMedia>
            <AlertDialogTitle>Hapus pengguna secara permanen?</AlertDialogTitle>
            <AlertDialogDescription>
              Akun {pendingDeleteUser?.name}, sesi, aplikasi OAuth miliknya, authorization code, dan seluruh token terkait akan dihapus. Tindakan ini tidak dapat dibatalkan.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter><AlertDialogCancel disabled={deletingUser}>Batal</AlertDialogCancel><AlertDialogAction variant="destructive" disabled={deletingUser} onClick={deleteUser}>{deletingUser ? "Menghapus..." : "Hapus permanen"}</AlertDialogAction></AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

function UserIdentity({ user }: { user: User }) {
  const initials = user.name.split(" ").map((part) => part[0]).join("").slice(0, 2).toUpperCase();
  return <div className="flex min-w-0 items-center gap-3"><Avatar className="size-10"><AvatarImage src={user.avatar} alt={user.name} /><AvatarFallback>{initials || <CircleUserRound />}</AvatarFallback></Avatar><div className="min-w-0"><p className="truncate font-medium">{user.name}</p><p className="truncate text-xs text-muted-foreground">{user.email}</p></div></div>;
}

function StatusBadges({ user }: { user: User }) {
  return <><Badge variant={user.is_active ? "outline" : "destructive"} className={user.is_active ? "border-primary/25 bg-primary/5 text-primary" : ""}>{user.is_active ? <UserCheck /> : <Ban />}{user.is_active ? "Aktif" : "Nonaktif"}</Badge><Badge variant={user.email_verified ? "outline" : "secondary"} className={user.email_verified ? "border-sky-500/25 bg-sky-500/5 text-sky-700" : ""}>{user.email_verified ? <BadgeCheck /> : <ShieldAlert />}{user.email_verified ? "Terverifikasi" : "Belum verifikasi"}</Badge></>;
}

function UserActions({ user, isCurrent, onRole, onStatus, onDelete }: { user: User; isCurrent: boolean; onRole: (user: User, role: Role) => void; onStatus: () => void; onDelete: () => void }) {
  return <DropdownMenu><DropdownMenuTrigger asChild><Button variant="ghost" size="icon" aria-label={`Aksi untuk ${user.name}`}><MoreHorizontal /></Button></DropdownMenuTrigger><DropdownMenuContent align="end" className="w-56"><DropdownMenuLabel>Kelola akun</DropdownMenuLabel><DropdownMenuSeparator /><DropdownMenuItem disabled={isCurrent || user.role === "anggota"} onSelect={() => onRole(user, "anggota")}><CircleUserRound />Jadikan anggota</DropdownMenuItem><DropdownMenuItem disabled={isCurrent || user.role === "super_admin"} onSelect={() => onRole(user, "super_admin")}><ShieldCheck />Jadikan super admin</DropdownMenuItem><DropdownMenuSeparator /><DropdownMenuItem variant={user.is_active ? "destructive" : "default"} disabled={isCurrent} onSelect={onStatus}>{user.is_active ? <Ban /> : <UserCheck />}{user.is_active ? "Nonaktifkan akun" : "Aktifkan akun"}</DropdownMenuItem><DropdownMenuItem variant="destructive" disabled={isCurrent} onSelect={onDelete}><Trash2 />Hapus permanen</DropdownMenuItem></DropdownMenuContent></DropdownMenu>;
}

function UserTableRow({ user, isCurrent, onRole, onStatus, onDelete }: { user: User; isCurrent: boolean; onRole: (user: User, role: Role) => void; onStatus: () => void; onDelete: () => void }) {
  return <TableRow><TableCell className="max-w-72 pl-4"><UserIdentity user={user} /></TableCell><TableCell><Badge variant={user.is_active ? "outline" : "destructive"} className={user.is_active ? "border-primary/25 bg-primary/5 text-primary" : ""}>{user.is_active ? <UserCheck /> : <Ban />}{user.is_active ? "Aktif" : "Nonaktif"}</Badge></TableCell><TableCell><Badge variant={user.email_verified ? "outline" : "secondary"} className={user.email_verified ? "border-sky-500/25 bg-sky-500/5 text-sky-700" : ""}>{user.email_verified ? <BadgeCheck /> : <ShieldAlert />}{user.email_verified ? "Terverifikasi" : "Belum verifikasi"}</Badge></TableCell><TableCell><Badge variant={user.role === "super_admin" ? "default" : "secondary"}>{user.role === "super_admin" ? "Super admin" : "Anggota"}</Badge></TableCell><TableCell className="hidden text-sm text-muted-foreground xl:table-cell">{new Date(user.created_at).toLocaleDateString("id-ID", { day: "2-digit", month: "short", year: "numeric" })}</TableCell><TableCell className="pr-4 text-right"><UserActions user={user} isCurrent={isCurrent} onRole={onRole} onStatus={onStatus} onDelete={onDelete} /></TableCell></TableRow>;
}

function UserMobileCard({ user, isCurrent, onRole, onStatus, onDelete }: { user: User; isCurrent: boolean; onRole: (user: User, role: Role) => void; onStatus: () => void; onDelete: () => void }) {
  return <div className="rounded-xl border p-4"><div className="flex items-start justify-between gap-3"><UserIdentity user={user} /><UserActions user={user} isCurrent={isCurrent} onRole={onRole} onStatus={onStatus} onDelete={onDelete} /></div><div className="mt-4 flex flex-wrap gap-2"><StatusBadges user={user} /><Badge variant={user.role === "super_admin" ? "default" : "secondary"}>{user.role === "super_admin" ? "Super admin" : "Anggota"}</Badge></div></div>;
}
