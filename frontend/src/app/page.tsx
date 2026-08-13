"use client";

import Link from "next/link";
import {
  ArrowRight,
  BookOpen,
  CheckCircle2,
  KeyRound,
  Link2,
  ShieldCheck,
} from "lucide-react";

import { useAuth } from "@/components/auth-provider";
import { Brand } from "@/components/brand";
import { Button } from "@/components/ui/button";
import { PUBLIC_DOCUMENTATION_URL } from "@/lib/public-env";

const integrationSteps = [
  {
    number: "01",
    title: "Daftarkan aplikasi",
    description: "Tambahkan nama aplikasi dan Redirect URI dari portal.",
  },
  {
    number: "02",
    title: "Simpan kredensial",
    description: "Gunakan Client ID dan Client Secret hanya di server aplikasi.",
  },
  {
    number: "03",
    title: "Mulai alur SSO",
    description: "Hubungkan pengguna dengan Authorization Code dan PKCE S256.",
  },
];

const foundations = [
  {
    icon: ShieldCheck,
    title: "Akun terverifikasi",
    description: "Setiap akun diaktifkan melalui kode OTP email.",
  },
  {
    icon: Link2,
    title: "Callback presisi",
    description: "Redirect URI harus sama persis dengan alamat yang terdaftar.",
  },
  {
    icon: KeyRound,
    title: "Kredensial terkendali",
    description: "Client secret dapat di-regenerate dan akses lama langsung dicabut.",
  },
];

export default function Home() {
  const { user, loading } = useAuth();
  const portalHref = user ? "/dashboard/profil" : "/login";

  return (
    <main className="min-h-svh bg-[#f8faf8] text-foreground">
      <header className="border-b bg-background/90 backdrop-blur">
        <div className="mx-auto flex h-18 max-w-6xl items-center justify-between px-4 sm:px-6 lg:px-8">
          <Brand />
          <nav className="flex items-center gap-2" aria-label="Navigasi utama">
            <Button variant="ghost" className="hidden sm:inline-flex" asChild>
              <a href={PUBLIC_DOCUMENTATION_URL}><BookOpen />Dokumentasi</a>
            </Button>
            <span className="inline-flex h-9 w-28 shrink-0" aria-busy={loading}>
              <Button className="w-full" asChild>
                <Link href={portalHref}>{user ? "Buka portal" : "Masuk"}<ArrowRight /></Link>
              </Button>
            </span>
          </nav>
        </div>
      </header>

      <section className="border-b bg-background">
        <div className="mx-auto flex max-w-5xl flex-col items-center px-4 py-20 text-center sm:px-6 sm:py-28 lg:px-8 lg:py-32">
          <div className="inline-flex items-center gap-2 rounded-full border bg-[#f8faf8] px-3 py-1.5 text-xs font-medium text-muted-foreground">
            <CheckCircle2 className="size-3.5 text-primary" />
            Single Sign-On IPNU IPPNU
          </div>
          <h1 className="mt-7 max-w-4xl text-4xl leading-[1.08] font-semibold tracking-[-0.045em] sm:text-6xl lg:text-7xl">
            Satu akun untuk seluruh layanan organisasi.
          </h1>
          <p className="mt-6 max-w-2xl text-base leading-7 text-muted-foreground sm:text-lg">
            Masuk sekali, kelola aplikasi OAuth, dan kendalikan izin akun dari portal identitas IPNU IPPNU yang sederhana dan aman.
          </p>
          <div className="mt-9 flex min-h-[6.25rem] w-full max-w-sm flex-col justify-center gap-3 sm:min-h-11 sm:w-auto sm:max-w-none sm:flex-row" aria-busy={loading}>
            <Button size="lg" className="px-6" asChild>
              <Link href={portalHref}>{user ? "Kelola akun" : "Masuk ke portal"}<ArrowRight /></Link>
            </Button>
            {!user && (
              <Button size="lg" variant="outline" className="px-6" asChild>
                <Link href="/register">Buat akun anggota</Link>
              </Button>
            )}
          </div>
        </div>
      </section>

      <section className="mx-auto max-w-6xl px-4 py-16 sm:px-6 sm:py-20 lg:px-8">
        <div className="grid gap-10 lg:grid-cols-[.72fr_1.28fr] lg:items-start lg:gap-16">
          <div>
            <p className="text-xs font-semibold tracking-[0.16em] text-primary uppercase">Integrasi singkat</p>
            <h2 className="mt-3 text-3xl font-semibold tracking-[-0.035em] sm:text-4xl">
              Hubungkan aplikasi dalam tiga langkah.
            </h2>
            <p className="mt-4 text-sm leading-6 text-muted-foreground">
              Setiap anggota dapat membuat aplikasi sendiri. Data dan kredensial hanya dapat dikelola oleh pemilik aplikasi.
            </p>
            <Button variant="outline" className="mt-7" asChild>
              <a href={PUBLIC_DOCUMENTATION_URL}><BookOpen />Baca dokumentasi</a>
            </Button>
          </div>

          <ol className="overflow-hidden rounded-2xl border bg-background shadow-sm">
            {integrationSteps.map((step) => (
              <li key={step.number} className="grid gap-3 border-b p-5 last:border-b-0 sm:grid-cols-[3rem_1fr] sm:p-6">
                <span className="font-mono text-sm font-medium text-primary">{step.number}</span>
                <div>
                  <h3 className="font-semibold">{step.title}</h3>
                  <p className="mt-1 text-sm leading-6 text-muted-foreground">{step.description}</p>
                </div>
              </li>
            ))}
          </ol>
        </div>
      </section>

      <section className="border-y bg-background">
        <div className="mx-auto grid max-w-6xl divide-y px-4 sm:grid-cols-3 sm:divide-x sm:divide-y-0 sm:px-6 lg:px-8">
          {foundations.map(({ icon: Icon, title, description }) => (
            <article key={title} className="py-8 sm:px-7 sm:first:pl-0 sm:last:pr-0">
              <span className="flex size-10 items-center justify-center rounded-xl bg-primary/10 text-primary">
                <Icon className="size-5" />
              </span>
              <h3 className="mt-5 font-semibold">{title}</h3>
              <p className="mt-2 text-sm leading-6 text-muted-foreground">{description}</p>
            </article>
          ))}
        </div>
      </section>

      <footer className="bg-background">
        <div className="mx-auto flex max-w-6xl flex-col gap-4 px-4 py-8 sm:flex-row sm:items-center sm:justify-between sm:px-6 lg:px-8">
          <Brand />
          <p className="text-xs text-muted-foreground">Pusat identitas dan akses layanan IPNU IPPNU.</p>
        </div>
      </footer>
    </main>
  );
}
