import { Skeleton } from "@/components/ui/skeleton";

function PageHeadingSkeleton({ action = false }: { action?: boolean }) {
  return (
    <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
      <div className="space-y-2">
        <Skeleton className="h-8 w-44 sm:h-9" />
        <Skeleton className="h-4 w-full max-w-md" />
      </div>
      {action && <Skeleton className="h-9 w-40" />}
    </div>
  );
}

function FieldSkeleton({ wide = false }: { wide?: boolean }) {
  return (
    <div className={wide ? "space-y-2 sm:col-span-2" : "space-y-2"}>
      <Skeleton className="h-4 w-24" />
      <Skeleton className="h-10 w-full" />
    </div>
  );
}

function TableRowsSkeleton({ count = 4 }: { count?: number }) {
  return (
    <div className="divide-y px-4">
      {Array.from({ length: count }, (_, index) => (
        <div key={index} className="flex min-h-16 items-center gap-3 py-3">
          <Skeleton className="size-9 shrink-0 rounded-full" />
          <div className="min-w-0 flex-1 space-y-2">
            <Skeleton className="h-3.5 w-36 max-w-full" />
            <Skeleton className="h-3 w-56 max-w-[80%]" />
          </div>
          <Skeleton className="hidden h-6 w-24 sm:block" />
          <Skeleton className="hidden h-6 w-20 lg:block" />
          <Skeleton className="size-8 shrink-0" />
        </div>
      ))}
    </div>
  );
}

function ProfileSkeleton() {
  return (
    <div className="space-y-7">
      <PageHeadingSkeleton />
      <div className="grid gap-6 lg:grid-cols-[320px_minmax(0,1fr)]">
        <div className="rounded-xl border bg-card p-5">
          <div className="flex flex-col items-center">
            <Skeleton className="size-24 rounded-full" />
            <Skeleton className="mt-4 h-5 w-32" />
            <Skeleton className="mt-2 h-4 w-44" />
            <div className="mt-4 flex gap-2"><Skeleton className="h-5 w-20" /><Skeleton className="h-5 w-28" /></div>
            <Skeleton className="mt-6 h-3 w-full max-w-56" />
          </div>
        </div>
        <div className="rounded-xl border bg-card p-5">
          <Skeleton className="h-5 w-40" />
          <Skeleton className="mt-2 h-4 w-72 max-w-full" />
          <div className="mt-6 grid gap-5 sm:grid-cols-2">
            <FieldSkeleton wide />
            <FieldSkeleton />
            <FieldSkeleton />
            <FieldSkeleton wide />
            <div className="space-y-2 sm:col-span-2"><Skeleton className="h-4 w-12" /><Skeleton className="h-20 w-full" /></div>
            <Skeleton className="h-9 w-full sm:w-40" />
          </div>
        </div>
      </div>
    </div>
  );
}

function SecuritySkeleton() {
  return (
    <div className="space-y-7">
      <PageHeadingSkeleton />
      <div className="grid gap-6 xl:grid-cols-[minmax(0,1.15fr)_minmax(320px,.85fr)]">
        <div className="rounded-xl border bg-card p-5">
          <Skeleton className="size-10 rounded-xl" />
          <Skeleton className="mt-4 h-5 w-40" />
          <Skeleton className="mt-2 h-4 w-72 max-w-full" />
          <div className="mt-6 space-y-4"><FieldSkeleton /><FieldSkeleton /><FieldSkeleton /><Skeleton className="h-9 w-full sm:w-48" /></div>
        </div>
        <div className="h-fit rounded-xl border bg-card p-5">
          <div className="flex justify-between"><Skeleton className="size-10 rounded-xl" /><Skeleton className="h-5 w-20" /></div>
          <Skeleton className="mt-4 h-5 w-52" />
          <Skeleton className="mt-2 h-4 w-64 max-w-full" />
          <Skeleton className="mt-6 h-24 w-full" />
          <Skeleton className="mt-4 h-9 w-28" />
        </div>
      </div>
    </div>
  );
}

function SessionsSkeleton() {
  return (
    <div className="space-y-6">
      <PageHeadingSkeleton />
      <div className="overflow-hidden rounded-xl border bg-card">
        <div className="flex items-start gap-3 border-b p-5">
          <Skeleton className="size-10 shrink-0 rounded-xl" />
          <div className="min-w-0 flex-1 space-y-2"><Skeleton className="h-5 w-44" /><Skeleton className="h-4 w-full max-w-lg" /></div>
        </div>
        <TableRowsSkeleton count={3} />
      </div>
      <Skeleton className="h-3 w-full max-w-3xl" />
    </div>
  );
}

export function OAuthClientCardSkeleton() {
  return (
    <div className="rounded-xl border bg-card p-5">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1 space-y-2"><Skeleton className="h-5 w-40 max-w-full" /><Skeleton className="h-4 w-56 max-w-full" /></div>
        <Skeleton className="size-9 shrink-0" />
      </div>
      <div className="mt-7 space-y-5">
        <div className="space-y-2">
          <Skeleton className="h-3 w-20" />
          <div className="flex gap-2"><Skeleton className="h-9 min-w-0 flex-1" /><Skeleton className="size-9 shrink-0" /></div>
        </div>
        <div className="space-y-2">
          <Skeleton className="h-3 w-24" />
          <div className="flex gap-2"><Skeleton className="h-9 min-w-0 flex-1" /><Skeleton className="size-9 shrink-0" /><Skeleton className="size-9 shrink-0" /></div>
          <Skeleton className="h-7 w-40" />
        </div>
        <div className="space-y-2"><Skeleton className="h-3 w-24" /><Skeleton className="h-3 w-[85%]" /><Skeleton className="h-3 w-[70%]" /></div>
      </div>
    </div>
  );
}

function ApplicationsSkeleton() {
  return (
    <div className="space-y-7">
      <PageHeadingSkeleton action />
      <div className="grid gap-4 md:grid-cols-2">
        {[0, 1].map((item) => <OAuthClientCardSkeleton key={item} />)}
      </div>
    </div>
  );
}

function ApplicationDetailSkeleton() {
  return (
    <div className="space-y-7">
      <div className="flex items-start gap-2">
        <Skeleton className="size-9 shrink-0" />
        <div className="min-w-0 flex-1 space-y-2">
          <Skeleton className="h-9 w-64 max-w-full" />
          <Skeleton className="h-4 w-full max-w-md" />
        </div>
      </div>
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1.35fr)_minmax(280px,.65fr)]">
        <div className="rounded-xl border bg-card p-5">
          <Skeleton className="h-5 w-40" />
          <Skeleton className="mt-2 h-4 w-72 max-w-full" />
          <div className="mt-6 grid gap-5 sm:grid-cols-2">
            <FieldSkeleton wide />
            <FieldSkeleton />
            <FieldSkeleton />
            <div className="space-y-2 sm:col-span-2"><Skeleton className="h-4 w-24" /><Skeleton className="h-20 w-full" /></div>
          </div>
        </div>
        <div className="h-fit rounded-xl border bg-card p-5">
          <Skeleton className="size-10 rounded-xl" />
          <Skeleton className="mt-4 h-5 w-36" />
          <Skeleton className="mt-2 h-4 w-full" />
          <Skeleton className="mt-6 h-9 w-20" />
        </div>
      </div>
      <div className="overflow-hidden rounded-xl border bg-card">
        <div className="flex flex-col gap-4 border-b p-5 sm:flex-row sm:items-center sm:justify-between">
          <div className="space-y-2"><Skeleton className="h-5 w-48" /><Skeleton className="h-4 w-72 max-w-full" /></div>
          <Skeleton className="h-10 w-full sm:w-80" />
        </div>
        <TableRowsSkeleton count={3} />
      </div>
    </div>
  );
}

function UsersSkeleton() {
  return (
    <div className="space-y-7">
      <PageHeadingSkeleton />
      <div className="grid gap-4 sm:grid-cols-3">
        {[0, 1, 2].map((item) => <div key={item} className="flex items-center gap-3 rounded-xl border bg-card p-4"><Skeleton className="size-10 shrink-0 rounded-xl" /><div className="space-y-2"><Skeleton className="h-3 w-24" /><Skeleton className="h-6 w-12" /></div></div>)}
      </div>
      <div className="overflow-hidden rounded-xl border bg-card">
        <div className="flex flex-col gap-4 border-b p-5 sm:flex-row sm:items-center sm:justify-between">
          <div className="space-y-2"><Skeleton className="h-5 w-36" /><Skeleton className="h-4 w-80 max-w-full" /></div>
          <Skeleton className="h-10 w-full sm:w-80" />
        </div>
        <TableRowsSkeleton />
      </div>
    </div>
  );
}

function AuditLogSkeleton() {
  return (
    <div className="space-y-7">
      <PageHeadingSkeleton />
      <div className="overflow-hidden rounded-xl border bg-card">
        <div className="space-y-5 border-b p-5">
          <div className="space-y-2"><Skeleton className="h-5 w-40" /><Skeleton className="h-4 w-80 max-w-full" /></div>
          <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_16rem]"><Skeleton className="h-10 w-full" /><Skeleton className="h-10 w-full" /></div>
        </div>
        <TableRowsSkeleton count={5} />
      </div>
    </div>
  );
}

function RouteSkeleton({ pathname }: { pathname: string }) {
  if (pathname.startsWith("/dashboard/keamanan")) return <SecuritySkeleton />;
  if (pathname.startsWith("/dashboard/sesi")) return <SessionsSkeleton />;
  if (/^\/dashboard\/aplikasi\/[^/]+/.test(pathname)) return <ApplicationDetailSkeleton />;
  if (pathname.startsWith("/dashboard/aplikasi")) return <ApplicationsSkeleton />;
  if (pathname.startsWith("/dashboard/pengguna")) return <UsersSkeleton />;
  if (pathname.startsWith("/dashboard/audit-log")) return <AuditLogSkeleton />;
  return <ProfileSkeleton />;
}

export function DashboardLoadingSkeleton({ pathname }: { pathname: string }) {
  return (
    <div className="w-full" role="status" aria-live="polite" aria-label="Memuat konten dashboard">
      <span className="sr-only">Memuat dashboard...</span>
      <div className="w-full" aria-hidden="true">
        <RouteSkeleton pathname={pathname} />
      </div>
    </div>
  );
}
