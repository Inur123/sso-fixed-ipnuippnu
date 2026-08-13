"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { AppWindow, BadgeCheck, ChevronDown, KeyRound, LayoutDashboard, LogOut, ScrollText, Shield, UserRound, UsersRound } from "lucide-react";
import { toast } from "sonner";

import { useAuth } from "@/components/auth-provider";
import { Brand } from "@/components/brand";
import { DashboardLoadingSkeleton } from "@/components/dashboard-loading-skeleton";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Sidebar, SidebarContent, SidebarFooter, SidebarGroup, SidebarGroupContent, SidebarGroupLabel, SidebarHeader, SidebarInset, SidebarMenu, SidebarMenuButton, SidebarMenuItem, SidebarProvider, SidebarRail, SidebarTrigger } from "@/components/ui/sidebar";
import { apiFetch, getErrorMessage } from "@/lib/api";

const memberNavigation = [
  { title: "Profil", href: "/dashboard/profil", icon: UserRound },
  { title: "Keamanan", href: "/dashboard/keamanan", icon: KeyRound },
  { title: "Sesi aplikasi", href: "/dashboard/sesi", icon: Shield },
  { title: "Aplikasi OAuth", href: "/dashboard/aplikasi", icon: AppWindow },
];
const adminNavigation = [
  { title: "Pengguna", href: "/dashboard/pengguna", icon: UsersRound },
  { title: "Audit log", href: "/dashboard/audit-log", icon: ScrollText },
];

export function DashboardShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { user, loading, setUser } = useAuth();
  const logoutStarted = useRef(false);
  const [loggingOut, setLoggingOut] = useState(false);

  useEffect(() => {
    if (!loading && !user && !logoutStarted.current) {
      router.replace(`/login?callbackUrl=${encodeURIComponent(pathname)}`);
    }
  }, [loading, pathname, router, user]);

  if (!loading && !user) return null;

  async function logout() {
    if (logoutStarted.current) return;
    logoutStarted.current = true;
    setLoggingOut(true);

    try {
      await apiFetch("/api/auth/logout", { method: "POST" });
      toast.success("Anda berhasil keluar.");
    } catch (error) {
      toast.error(getErrorMessage(error));
    } finally {
      setUser(null);
      router.replace("/login");
      setLoggingOut(false);
    }
  }

  const navigation = user?.role === "super_admin" ? [...memberNavigation, ...adminNavigation] : memberNavigation;
  const initials = user?.name.split(" ").map((part) => part[0]).join("").slice(0, 2).toUpperCase() ?? "";

  return (
    <SidebarProvider>
      <Sidebar collapsible="icon" className="border-r">
        <SidebarHeader className="h-20 justify-center px-3 group-data-[collapsible=icon]:px-1.5">
          <Brand className="group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:gap-0 [&>img]:group-data-[collapsible=icon]:size-9 [&>span:last-child]:group-data-[collapsible=icon]:hidden" />
        </SidebarHeader>
        <SidebarContent>
          <SidebarGroup>
            <SidebarGroupLabel>Portal identitas</SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                {navigation.map(({ title, href, icon: Icon }) => (
                  <SidebarMenuItem key={href}>
                    <SidebarMenuButton asChild isActive={pathname.startsWith(href)} tooltip={title}>
                      <Link href={href}><Icon /><span>{title}</span></Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                ))}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        </SidebarContent>
        <SidebarFooter className="p-3">
          <SidebarMenu><SidebarMenuItem>
            <SidebarMenuButton asChild tooltip="Kembali ke beranda"><Link href="/"><LayoutDashboard /><span>Beranda</span></Link></SidebarMenuButton>
          </SidebarMenuItem></SidebarMenu>
        </SidebarFooter>
        <SidebarRail />
      </Sidebar>
      <SidebarInset>
        <header className="sticky top-0 z-20 flex h-16 items-center justify-between border-b bg-background/90 px-4 backdrop-blur-xl sm:px-6 lg:px-8">
          <div className="flex items-center gap-3"><SidebarTrigger /><div className="hidden h-5 w-px bg-border sm:block" /><p className="hidden text-sm text-muted-foreground sm:block">Pusat kendali IPNU IPPNU ID</p></div>
          {user ? (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" className="group h-auto gap-3 px-2 py-1.5" aria-label="Buka menu akun dan logout">
                  <Avatar className="size-8"><AvatarImage src={user.avatar} alt={user.name} /><AvatarFallback>{initials}</AvatarFallback></Avatar>
                  <span className="hidden text-left sm:grid"><span className="max-w-36 truncate text-sm font-medium">{user.name}</span><span className="text-xs text-muted-foreground">{user.role === "super_admin" ? "Super admin" : "Anggota"}</span></span>
                  <ChevronDown className="size-4 text-muted-foreground transition-transform group-data-[state=open]:rotate-180" aria-hidden="true" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-60">
                <DropdownMenuLabel className="space-y-2">
                  <span className="block truncate text-sm text-foreground">{user.email}</span>
                  <span className="flex flex-wrap gap-1.5"><Badge variant="secondary">{user.role === "super_admin" ? "Super admin" : "Anggota"}</Badge>{user.email_verified && <Badge variant="outline" className="text-primary"><BadgeCheck />Terverifikasi</Badge>}</span>
                </DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem variant="destructive" disabled={loggingOut} onSelect={logout}><LogOut />{loggingOut ? "Sedang keluar..." : "Keluar"}</DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          ) : (
            <span className="text-xs text-muted-foreground" aria-live="polite">Memuat akun...</span>
          )}
        </header>
        <main className="flex-1 bg-muted/20 p-4 sm:p-6 lg:px-8 lg:py-6" aria-busy={loading || !user}>
          <div className="mx-auto w-full max-w-7xl">
            {loading || !user ? <DashboardLoadingSkeleton pathname={pathname} /> : children}
          </div>
        </main>
      </SidebarInset>
    </SidebarProvider>
  );
}
