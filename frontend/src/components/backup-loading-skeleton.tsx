import { CalendarClock, Plus, RefreshCw } from "lucide-react";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

export function BackupContentSkeleton() {
  return (
    <div role="status" aria-label="Memuat jadwal dan riwayat backup" aria-busy="true">
      <span className="sr-only">Memuat data backup…</span>
      <div className="min-w-0 space-y-6" aria-hidden="true">
        <section className="grid gap-5 rounded-xl border bg-card p-5 text-sm sm:grid-cols-2 lg:grid-cols-3">
          <div className="min-w-0 space-y-1.5">
            <p className="flex items-center gap-2 font-medium"><CalendarClock className="size-4 text-primary" />Minggu, 02.00 WIB</p>
            <Skeleton className="h-5 w-56 max-w-full motion-reduce:animate-none" />
          </div>
          <div className="min-w-0 space-y-1.5">
            <p className="font-medium">Maksimal 10 backup</p>
            <Skeleton className="h-5 w-64 max-w-full motion-reduce:animate-none" />
          </div>
          <div className="min-w-0 space-y-1.5">
            <p className="font-medium">Backup terakhir berhasil</p>
            <Skeleton className="h-5 w-44 max-w-full motion-reduce:animate-none" />
          </div>
        </section>

        <Card className="min-w-0 overflow-hidden">
          <CardHeader className="flex flex-row items-start justify-between gap-3 border-b pb-5">
            <div><CardTitle>Riwayat backup</CardTitle><CardDescription className="mt-1">File lama dihapus hanya setelah backup baru berhasil diverifikasi.</CardDescription></div>
            <Button variant="outline" size="icon" disabled tabIndex={-1} aria-label="Perbarui riwayat backup"><RefreshCw /></Button>
          </CardHeader>
          <CardContent className="p-0">
            <Table>
              <TableHeader><TableRow><TableHead className="pl-5">Waktu & sumber</TableHead><TableHead>Status</TableHead><TableHead>Ukuran arsip R2</TableHead><TableHead className="pr-5 text-right">File</TableHead></TableRow></TableHeader>
              <TableBody>
                {[0, 1, 2, 3].map((row) => (
                  <TableRow key={row}>
                    <TableCell className="py-4 pl-5 align-top"><Skeleton className="h-5 w-44 motion-reduce:animate-none" /><Skeleton className="mt-1 h-4 w-12 motion-reduce:animate-none" /></TableCell>
                    <TableCell className="min-w-48 py-4 align-top"><Skeleton className="h-6 w-20 rounded-full motion-reduce:animate-none" /></TableCell>
                    <TableCell className="py-4 align-top"><Skeleton className="h-5 w-16 motion-reduce:animate-none" /></TableCell>
                    <TableCell className="py-4 pr-5 align-top"><Skeleton className="ml-auto h-8 w-28 motion-reduce:animate-none" /><Skeleton className="mt-2 ml-auto h-4 w-32 motion-reduce:animate-none" /></TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

export function BackupPageSkeleton() {
  return (
    <div className="min-w-0 space-y-6">
      <PageHeader title="Backup database" description="Backup manual dan otomatis dengan unduhan SQL yang siap diimpor ke PostgreSQL."
        action={<Button disabled tabIndex={-1} className="disabled:opacity-100"><Plus />Buat backup</Button>} />
      <BackupContentSkeleton />
    </div>
  );
}
