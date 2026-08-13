"use client";

import { useEffect, useState } from "react";
import { AppWindow, Clock3, Link2, ShieldCheck, Trash2 } from "lucide-react";
import { toast } from "sonner";

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogMedia,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiFetch, getErrorMessage } from "@/lib/api";

interface OAuthConnection {
  id: string;
  client_id: string;
  name: string;
  description: string;
  scopes: string[];
  connected_at: string;
  last_used_at: string | null;
  expires_at: string;
}

const dateTimeFormatter = new Intl.DateTimeFormat("id-ID", {
  dateStyle: "medium",
  timeStyle: "short",
});

function formatDateTime(value: string | null) {
  if (!value) return "Belum digunakan";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "—" : dateTimeFormatter.format(date);
}

function ScopeList({ scopes }: { scopes: string[] }) {
  if (scopes.length === 0) return <span className="text-sm text-muted-foreground">Tidak ada scope</span>;

  return (
    <div className="flex flex-wrap gap-1.5">
      {scopes.map((scope) => (
        <Badge key={scope} variant="secondary" className="font-mono font-normal">
          {scope}
        </Badge>
      ))}
    </div>
  );
}

function RevokeConnectionButton({
  connection,
  revoking,
  onRevoke,
}: {
  connection: OAuthConnection;
  revoking: boolean;
  onRevoke: (connection: OAuthConnection) => Promise<void>;
}) {
  return (
    <AlertDialog>
      <AlertDialogTrigger asChild>
        <Button
          variant="destructive"
          size="sm"
          disabled={revoking}
          aria-label={`Cabut akses ${connection.name}`}
        >
          {revoking ? <Spinner /> : <Trash2 />}
          {revoking ? "Mencabut..." : "Cabut akses"}
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogMedia className="bg-destructive/10 text-destructive">
            <Trash2 />
          </AlertDialogMedia>
          <AlertDialogTitle>Cabut akses {connection.name}?</AlertDialogTitle>
          <AlertDialogDescription>
            Aplikasi ini tidak lagi dapat mengakses data akun IPNU IPPNU ID Anda. Untuk menghubungkannya kembali, Anda harus masuk dan memberi izin ulang melalui aplikasi tersebut.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Batal</AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            disabled={revoking}
            onClick={() => void onRevoke(connection)}
          >
            Cabut akses
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

export default function SesiPage() {
  const [connections, setConnections] = useState<OAuthConnection[]>([]);
  const [loading, setLoading] = useState(true);
  const [revokingID, setRevokingID] = useState<string | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    apiFetch<{ connections: OAuthConnection[] }>("/api/connections", { signal: controller.signal })
      .then((data) => setConnections(data.connections ?? []))
      .catch((error) => {
        if (!(error instanceof DOMException && error.name === "AbortError")) {
          toast.error(getErrorMessage(error));
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, []);

  async function revoke(connection: OAuthConnection) {
    setRevokingID(connection.id);
    try {
      await apiFetch<{ message: string }>(`/api/connections/${connection.id}`, { method: "DELETE" });
      setConnections((items) => items.filter((item) => item.id !== connection.id));
      toast.success(`Akses ${connection.name} berhasil dicabut.`);
    } catch (error) {
      toast.error(getErrorMessage(error));
    } finally {
      setRevokingID(null);
    }
  }

  return (
    <div className="space-y-6">
      <div className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">Sesi aplikasi</h1>
        <p className="text-sm text-muted-foreground sm:text-base">
          Pantau dan kelola aplikasi yang terhubung dengan akun IPNU IPPNU ID Anda.
        </p>
      </div>

      <Card>
        <CardHeader className="border-b">
          <div className="flex items-start gap-3">
            <span className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
              <Link2 className="size-5" />
            </span>
            <div className="space-y-1">
              <CardTitle>Aplikasi terhubung</CardTitle>
              <CardDescription>
                Setiap koneksi dibuat saat Anda masuk melalui SSO dan menyetujui akses aplikasi.
              </CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="space-y-3">
              {[1, 2, 3].map((item) => <Skeleton key={item} className="h-24 w-full" />)}
            </div>
          ) : connections.length === 0 ? (
            <Empty className="py-12">
              <EmptyHeader>
                <EmptyMedia variant="icon"><AppWindow /></EmptyMedia>
                <EmptyTitle>Belum ada aplikasi terhubung</EmptyTitle>
                <EmptyDescription>
                  Aplikasi akan muncul di sini setelah Anda menggunakan IPNU IPPNU ID untuk masuk.
                </EmptyDescription>
              </EmptyHeader>
            </Empty>
          ) : (
            <>
              <div className="hidden md:block">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Aplikasi</TableHead>
                      <TableHead>Izin akses</TableHead>
                      <TableHead>Terakhir digunakan</TableHead>
                      <TableHead>Terhubung sejak</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead className="text-right">Aksi</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {connections.map((connection) => (
                      <TableRow key={connection.id}>
                        <TableCell className="max-w-60 whitespace-normal py-4">
                          <div className="flex items-start gap-3">
                            <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                              <AppWindow className="size-4" />
                            </span>
                            <div className="min-w-0">
                              <p className="font-medium text-foreground">{connection.name}</p>
                              <p className="mt-0.5 line-clamp-2 text-xs text-muted-foreground">
                                {connection.description || "Aplikasi SSO terdaftar"}
                              </p>
                            </div>
                          </div>
                        </TableCell>
                        <TableCell className="max-w-56 whitespace-normal py-4"><ScopeList scopes={connection.scopes} /></TableCell>
                        <TableCell className="py-4 text-sm text-muted-foreground">{formatDateTime(connection.last_used_at)}</TableCell>
                        <TableCell className="py-4 text-sm text-muted-foreground">{formatDateTime(connection.connected_at)}</TableCell>
                        <TableCell className="py-4">
                          <Badge className="bg-emerald-600 text-white hover:bg-emerald-600">
                            <ShieldCheck /> Aktif
                          </Badge>
                        </TableCell>
                        <TableCell className="py-4 text-right">
                          <RevokeConnectionButton
                            connection={connection}
                            revoking={revokingID === connection.id}
                            onRevoke={revoke}
                          />
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>

              <div className="grid gap-3 md:hidden">
                {connections.map((connection) => (
                  <div key={connection.id} className="space-y-4 rounded-xl border p-4">
                    <div className="flex items-start justify-between gap-3">
                      <div className="flex min-w-0 items-start gap-3">
                        <span className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-muted text-muted-foreground">
                          <AppWindow className="size-5" />
                        </span>
                        <div className="min-w-0">
                          <p className="font-medium">{connection.name}</p>
                          <p className="mt-0.5 line-clamp-2 text-xs text-muted-foreground">
                            {connection.description || "Aplikasi SSO terdaftar"}
                          </p>
                        </div>
                      </div>
                      <Badge className="bg-emerald-600 text-white hover:bg-emerald-600">
                        <ShieldCheck /> Aktif
                      </Badge>
                    </div>

                    <div className="space-y-2">
                      <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Izin akses</p>
                      <ScopeList scopes={connection.scopes} />
                    </div>

                    <div className="grid gap-3 rounded-lg bg-muted/50 p-3 text-xs sm:grid-cols-2">
                      <div className="flex items-start gap-2">
                        <Clock3 className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
                        <div><p className="text-muted-foreground">Terakhir digunakan</p><p className="mt-0.5 font-medium">{formatDateTime(connection.last_used_at)}</p></div>
                      </div>
                      <div className="flex items-start gap-2">
                        <Link2 className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
                        <div><p className="text-muted-foreground">Terhubung sejak</p><p className="mt-0.5 font-medium">{formatDateTime(connection.connected_at)}</p></div>
                      </div>
                    </div>

                    <RevokeConnectionButton
                      connection={connection}
                      revoking={revokingID === connection.id}
                      onRevoke={revoke}
                    />
                  </div>
                ))}
              </div>
            </>
          )}
        </CardContent>
      </Card>

      <p className="flex items-start gap-2 text-xs text-muted-foreground">
        <ShieldCheck className="mt-0.5 size-3.5 shrink-0 text-primary" />
        Cabut akses menghentikan grant dan token di IPNU IPPNU ID. Aplikasi dapat tetap memiliki sesi lokal sampai aplikasi tersebut melakukan logout. Access token dan refresh token tidak pernah ditampilkan di halaman ini.
      </p>
    </div>
  );
}
