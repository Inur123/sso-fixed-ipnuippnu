"use client";

import Link from "next/link";
import {
  ArrowRight,
  CheckCircle2,
  Clock3,
  Fingerprint,
  Globe,
  LogIn,
  ShieldCheck,
  Sparkles,
  UserRound,
  Users,
} from "lucide-react";

import { useAuth } from "@/components/auth-provider";
import { Brand } from "@/components/brand";
import { Button } from "@/components/ui/button";
import { PUBLIC_ORGANIZATION_NAME } from "@/lib/public-env";

const benefits = [
  {
    icon: Fingerprint,
    title: "Satu akun untuk semua",
    description:
      "Cukup satu akun SSO untuk mengakses seluruh layanan digital milik IPNU IPPNU Magetan — tanpa perlu daftar ulang di setiap aplikasi.",
  },
  {
    icon: ShieldCheck,
    title: "Aman & terverifikasi",
    description:
      "Akun Anda dilindungi dengan verifikasi email dan pengelolaan sesi yang aman. Data pribadi hanya digunakan sesuai kebutuhan layanan.",
  },
  {
    icon: Globe,
    title: "Akses layanan dengan mudah",
    description:
      "Masuk ke berbagai aplikasi organisasi cukup dengan satu kali login. Praktis, cepat, dan tanpa ribet.",
  },
  {
    icon: Users,
    title: "Untuk seluruh anggota",
    description:
      "SSO ini dirancang khusus untuk anggota dan pengurus IPNU IPPNU Kabupaten Magetan agar terhubung dalam ekosistem digital organisasi.",
  },
];

const connectedServices = [
  {
    icon: ShieldCheck,
    name: "Laci",
    title: "Layanan Cerdas Administrasi",
    description:
      "Kelola kebutuhan administrasi organisasi dengan akses yang terhubung ke satu akun SSO.",
  },
  {
    icon: UserRound,
    name: "Sistem Anggota",
    title: "Sistem data dan keanggotaan",
    description:
      "Akses layanan keanggotaan dan informasi anggota dalam ekosistem digital organisasi.",
  },
];

export default function Home() {
  const { user, loading } = useAuth();
  const portalHref = user ? "/dashboard/profil" : "/login";

  return (
    <main className="min-h-svh bg-[#f8faf8] text-foreground">
      {/* ── Header ── */}
      <header className="border-b bg-background/90 backdrop-blur">
        <div className="mx-auto flex h-18 max-w-6xl items-center justify-between px-4 sm:px-6 lg:px-8">
          <Brand />
          <nav className="flex items-center gap-2" aria-label="Navigasi utama">
            <span className="inline-flex h-9 w-28 shrink-0" aria-busy={loading}>
              <Button className="w-full" asChild>
                <Link href={portalHref}>
                  {user ? "Buka portal" : "Masuk"}
                  <ArrowRight />
                </Link>
              </Button>
            </span>
          </nav>
        </div>
      </header>

      {/* ── Hero ── */}
      <section className="border-b bg-background">
        <div className="mx-auto flex max-w-5xl flex-col items-center px-4 py-20 text-center sm:px-6 sm:py-28 lg:px-8 lg:py-32">
          <div className="inline-flex items-center gap-2 rounded-full border bg-[#f8faf8] px-3 py-1.5 text-xs font-medium text-muted-foreground">
            <CheckCircle2 className="size-3.5 text-primary" />
            {PUBLIC_ORGANIZATION_NAME}
          </div>

          <h1 className="mt-7 max-w-4xl text-4xl leading-[1.08] font-semibold tracking-[-0.045em] sm:text-6xl lg:text-7xl">
            Satu akun untuk semua layanan {PUBLIC_ORGANIZATION_NAME}.
          </h1>

          <p className="mt-6 max-w-2xl text-base leading-7 text-muted-foreground sm:text-lg">
            Single Sign-On (SSO) resmi yang memudahkan anggota dan pengurus mengakses
            seluruh layanan digital organisasi hanya dengan satu akun.
          </p>

          <div
            className="mt-9 flex min-h-[6.25rem] w-full max-w-sm flex-col justify-center gap-3 sm:min-h-11 sm:w-auto sm:max-w-none sm:flex-row"
            aria-busy={loading}
          >
            <Button size="lg" className="px-6" asChild>
              <Link href={portalHref}>
                {user ? "Kelola akun" : "Masuk ke akun"}
                <LogIn />
              </Link>
            </Button>
            {!user && (
              <Button size="lg" variant="outline" className="px-6" asChild>
                <Link href="/register">
                  <Sparkles />
                  Daftar sekarang
                </Link>
              </Button>
            )}
          </div>
        </div>
      </section>

      {/* ── Layanan terhubung ── */}
      <section className="border-b bg-[#f8faf8]">
        <div className="mx-auto max-w-6xl px-4 py-16 sm:px-6 sm:py-20 lg:px-8">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <p className="text-xs font-semibold tracking-[0.16em] text-primary uppercase">
                Ekosistem layanan
              </p>
              <h2 className="mt-3 text-3xl font-semibold tracking-[-0.035em] sm:text-4xl">
                Layanan yang terhubung dengan SSO.
              </h2>
            </div>
            <p className="max-w-md text-sm leading-6 text-muted-foreground sm:text-right">
              Satu akun untuk layanan digital organisasi yang sedang kami siapkan.
            </p>
          </div>

          <div className="mt-10 grid gap-5 md:grid-cols-2">
            {connectedServices.map(({ icon: Icon, name, title, description }) => (
              <article
                key={name}
                className="group relative overflow-hidden rounded-2xl border bg-background p-6 shadow-sm transition-shadow hover:shadow-md sm:p-7"
              >
                <div className="absolute inset-x-0 top-0 h-1 bg-primary/15 transition-colors group-hover:bg-primary/30" />
                <div className="flex items-start justify-between gap-5">
                  <span className="flex size-12 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
                    <Icon className="size-6" />
                  </span>
                  <span className="inline-flex items-center gap-1.5 rounded-full border border-primary/20 bg-primary/5 px-2.5 py-1 text-xs font-medium text-primary">
                    <Clock3 className="size-3.5" />
                    Coming soon
                  </span>
                </div>
                <h3 className="mt-6 text-xl font-semibold tracking-[-0.02em]">{name}</h3>
                <p className="mt-1 text-sm font-medium text-primary">{title}</p>
                <p className="mt-3 max-w-lg text-sm leading-6 text-muted-foreground">
                  {description}
                </p>
              </article>
            ))}
          </div>
        </div>
      </section>

      {/* ── Keuntungan ── */}
      <section className="mx-auto max-w-6xl px-4 py-16 sm:px-6 sm:py-20 lg:px-8">
        <div className="text-center">
          <p className="text-xs font-semibold tracking-[0.16em] text-primary uppercase">
            Kenapa SSO?
          </p>
          <h2 className="mt-3 text-3xl font-semibold tracking-[-0.035em] sm:text-4xl">
            Kemudahan akses untuk anggota organisasi.
          </h2>
          <p className="mx-auto mt-4 max-w-2xl text-sm leading-6 text-muted-foreground">
            Dengan satu akun SSO, Anda bisa langsung terhubung ke semua aplikasi dan
            layanan digital {PUBLIC_ORGANIZATION_NAME}.
          </p>
        </div>

        <div className="mt-12 grid gap-6 sm:grid-cols-2 lg:gap-8">
          {benefits.map(({ icon: Icon, title, description }) => (
            <article
              key={title}
              className="rounded-2xl border bg-background p-6 shadow-sm transition-shadow hover:shadow-md sm:p-8"
            >
              <span className="flex size-11 items-center justify-center rounded-xl bg-primary/10 text-primary">
                <Icon className="size-5" />
              </span>
              <h3 className="mt-5 text-lg font-semibold">{title}</h3>
              <p className="mt-2 text-sm leading-6 text-muted-foreground">
                {description}
              </p>
            </article>
          ))}
        </div>
      </section>

      {/* ── CTA Bottom ── */}
      <section className="border-y bg-background">
        <div className="mx-auto flex max-w-4xl flex-col items-center gap-6 px-4 py-16 text-center sm:px-6 sm:py-20 lg:px-8">
          <h2 className="text-2xl font-semibold tracking-[-0.035em] sm:text-3xl">
            Siap bergabung?
          </h2>
          <p className="max-w-xl text-sm leading-6 text-muted-foreground">
            Buat akun SSO Anda sekarang dan nikmati kemudahan mengakses seluruh layanan
            digital {PUBLIC_ORGANIZATION_NAME} dengan satu kali login.
          </p>
          <div className="flex flex-col gap-3 sm:flex-row">
            <Button size="lg" className="px-6" asChild>
              <Link href={user ? "/dashboard/profil" : "/register"}>
                {user ? "Buka portal" : "Daftar akun"}
                <ArrowRight />
              </Link>
            </Button>
            {!user && (
              <Button size="lg" variant="outline" className="px-6" asChild>
                <Link href="/login">Sudah punya akun? Masuk</Link>
              </Button>
            )}
          </div>
        </div>
      </section>

      {/* ── Footer ── */}
      <footer className="bg-background">
        <div className="mx-auto flex max-w-6xl flex-col gap-4 px-4 py-8 sm:flex-row sm:items-center sm:justify-between sm:px-6 lg:px-8">
          <Brand />
          <p className="text-xs text-muted-foreground">
            Pusat identitas dan akses resmi {PUBLIC_ORGANIZATION_NAME}.
          </p>
        </div>
      </footer>
    </main>
  );
}
