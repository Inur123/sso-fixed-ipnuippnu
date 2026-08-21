import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Brand } from "@/components/brand";

export function AuthShell({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
  children: React.ReactNode;
}) {
  return (
    <main className="auth-shell relative isolate min-h-svh w-full overflow-x-clip bg-background lg:bg-muted/35">
      <div className="pointer-events-none absolute inset-0 hidden bg-[radial-gradient(circle_at_10%_5%,color-mix(in_oklab,var(--primary)_16%,transparent),transparent_30%),radial-gradient(circle_at_95%_90%,color-mix(in_oklab,var(--primary)_9%,transparent),transparent_32%)] lg:block" />
      <div className="relative mx-auto flex min-h-svh w-full min-w-0 items-start lg:items-center lg:justify-center lg:p-8">
        <div className="auth-shell-frame grid w-full min-w-0 grid-cols-[minmax(0,1fr)] bg-background lg:min-h-[42rem] lg:max-w-6xl lg:grid-cols-[minmax(0,1.04fr)_minmax(0,.96fr)] lg:overflow-hidden lg:rounded-3xl lg:shadow-2xl lg:shadow-primary/8 lg:ring-1 lg:ring-foreground/10">
          <section className="relative hidden min-w-0 flex-col justify-between overflow-hidden bg-primary p-10 text-primary-foreground lg:flex xl:p-12">
            <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_20%_15%,color-mix(in_oklab,white_16%,transparent),transparent_25%),linear-gradient(145deg,transparent_45%,color-mix(in_oklab,black_10%,transparent))]" />
            <div className="pointer-events-none absolute -right-16 -bottom-20 size-72 rounded-full border border-primary-foreground/10" />
            <Brand inverted className="relative" />
            <div className="relative space-y-6">
              <Badge variant="secondary" className="w-fit border-0 bg-primary-foreground text-primary">
                SSO terpusat & aman
              </Badge>
              <h1 className="max-w-lg text-4xl leading-tight font-semibold tracking-[-0.035em] xl:text-5xl">
                Satu identitas untuk seluruh layanan organisasi.
              </h1>
              <p className="max-w-md text-base leading-7 text-primary-foreground/75">
                Masuk dengan satu akun untuk mengakses semua layanan digital PC IPNU IPPNU Kabupaten Magetan.
              </p>
            </div>
            <p className="relative text-xs text-primary-foreground/60">IPNU IPPNU Magetan ID · Single Sign-On</p>
          </section>
          <div className="auth-shell-form flex w-full min-w-0 max-w-full items-start px-5 pt-[max(1.5rem,env(safe-area-inset-top))] pb-[max(2rem,env(safe-area-inset-bottom))] sm:px-8 sm:py-10 lg:items-center lg:px-10 lg:py-12 xl:px-14">
            <Card className="mx-auto w-full min-w-0 max-w-[30rem] gap-6 bg-transparent py-0 ring-0 shadow-none">
              <CardHeader className="min-w-0 gap-2 px-0">
                <div className="mb-7 lg:hidden"><Brand /></div>
                <CardTitle className="text-2xl leading-tight font-semibold tracking-tight sm:text-3xl">{title}</CardTitle>
                <CardDescription className="max-w-md text-sm leading-6 sm:text-base">{description}</CardDescription>
              </CardHeader>
              <CardContent className="min-w-0 px-0">{children}</CardContent>
            </Card>
          </div>
        </div>
      </div>
    </main>
  );
}
