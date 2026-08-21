"use client";

import {
  Suspense,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
import { useRouter, useSearchParams } from "next/navigation";
import {
  AlertCircle,
  AppWindow,
  Check,
  ChevronRight,
  CircleUserRoundIcon,
  ShieldCheck,
  X,
} from "lucide-react";

import { useAuth } from "@/components/auth-provider";
import { Brand } from "@/components/brand";
import { Alert, AlertDescription } from "@/components/ui/alert";
import {
  AvatarBadge,
} from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import { UserAvatar } from "@/components/user-avatar";
import { APIError, apiFetch, getErrorMessage } from "@/lib/api";

interface ClientInfo {
  client_id: string;
  name: string;
  description: string;
  allowed_scopes: string[];
  consent_required: boolean;
  select_account: boolean;
}

function AuthorizeContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { user, loading: authLoading } = useAuth();
  const [client, setClient] = useState<ClientInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [approving, setApproving] = useState(false);
  const [error, setError] = useState("");

  const request = useMemo(
    () => ({
      client_id: searchParams.get("client_id") ?? "",
      redirect_uri: searchParams.get("redirect_uri") ?? "",
      response_type: searchParams.get("response_type") ?? "",
      scope: searchParams.get("scope") ?? "",
      state: searchParams.get("state") ?? "",
      code_challenge: searchParams.get("code_challenge") ?? "",
      code_challenge_method: searchParams.get("code_challenge_method") ?? "",
      nonce: searchParams.get("nonce") ?? "",
      prompt: searchParams.get("prompt") ?? "",
    }),
    [searchParams],
  );
  const complete =
    Object.entries(request).every(
      ([key, value]) => key === "nonce" || key === "prompt" || Boolean(value),
    ) &&
    request.response_type === "code" &&
    request.code_challenge_method === "S256" &&
    (!request.scope.split(" ").includes("openid") || Boolean(request.nonce));

  const submitAuthorization = useCallback(
    async (consentApproved: boolean) => {
      setApproving(true);
      setError("");
      try {
        const data = await apiFetch<{ redirect_url: string }>(
          "/oauth/authorize",
          {
            method: "POST",
            body: JSON.stringify({
              ...request,
              consent_approved: consentApproved,
            }),
          },
        );
        window.location.assign(data.redirect_url);
      } catch (approvalError) {
        if (
          approvalError instanceof APIError &&
          typeof approvalError.payload?.redirect_url === "string"
        ) {
          window.location.assign(approvalError.payload.redirect_url);
          return;
        }
        if (
          approvalError instanceof APIError &&
          approvalError.code === "consent_required"
        ) {
          setClient((current) =>
            current
              ? {
                  ...current,
                  consent_required: true,
                }
              : current,
          );
        }
        setError(getErrorMessage(approvalError));
        setApproving(false);
      }
    },
    [request],
  );

  useEffect(() => {
    if (authLoading) return;
    if (!user) {
      const callback = `${window.location.pathname}${window.location.search}`;
      router.replace(`/login?callbackUrl=${encodeURIComponent(callback)}`);
      return;
    }
    if (!complete) {
      Promise.resolve().then(() => {
        setError(
          "Permintaan OAuth tidak lengkap. response_type=code, state, dan PKCE S256 wajib digunakan.",
        );
        setLoading(false);
      });
      return;
    }
    let active = true;
    const query = new URLSearchParams({
      client_id: request.client_id,
      redirect_uri: request.redirect_uri,
      scope: request.scope,
      prompt: request.prompt,
    });
    apiFetch<ClientInfo>(`/api/oauth/authorization-context?${query.toString()}`)
      .then((data) => {
        if (active) setClient(data);
      })
      .catch((loadError) => {
        if (active) setError(getErrorMessage(loadError));
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [
    authLoading,
    complete,
    request.client_id,
    request.redirect_uri,
    request.scope,
    request.prompt,
    router,
    user,
  ]);

  function approve() {
    void submitAuthorization(Boolean(client?.consent_required));
  }

  function cancel() {
    if (!client) return;
    const target = new URL(request.redirect_uri);
    target.searchParams.set("error", "access_denied");
    target.searchParams.set("state", request.state);
    window.location.assign(target.toString());
  }

  async function useAnotherAccount() {
    setApproving(true);
    setError("");
    try {
      await apiFetch<{ message: string }>("/api/auth/logout", { method: "POST" });
      const callback = `${window.location.pathname}${window.location.search}`;
      router.replace(`/login?callbackUrl=${encodeURIComponent(callback)}`);
    } catch (logoutError) {
      setError(getErrorMessage(logoutError));
      setApproving(false);
    }
  }

  if (authLoading || loading || !user)
    return (
      <main className="flex min-h-svh items-center justify-center bg-muted/30 p-4">
        <Card className="w-full max-w-lg">
          <CardHeader>
            <Skeleton className="h-8 w-36" />
            <Skeleton className="h-5 w-64" />
          </CardHeader>
          <CardContent>
            <Skeleton className="h-40 w-full" />
          </CardContent>
        </Card>
      </main>
    );

  if (client && !client.consent_required) {
    return (
      <main className="flex min-h-svh items-center justify-center bg-muted/30 p-4 sm:p-8">
        <Card className="w-full max-w-lg overflow-hidden border-t-4 border-t-primary shadow-xl shadow-primary/5">
          <CardHeader className="space-y-6 pb-4">
            <Brand />
            <div>
              <CardTitle className="text-2xl tracking-tight">Pilih akun</CardTitle>
              <CardDescription className="mt-2 leading-6">
                Pilih akun untuk melanjutkan ke{" "}
                <span className="font-medium text-foreground">{client.name}</span>.
              </CardDescription>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            {error && (
              <Alert variant="destructive">
                <AlertCircle />
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}

            <div className="overflow-hidden rounded-2xl border bg-background shadow-sm">
              <button
                type="button"
                className="group flex w-full items-center gap-4 p-5 text-left transition-colors hover:bg-primary/[0.04] focus-visible:bg-primary/[0.04] focus-visible:outline-none disabled:pointer-events-none disabled:opacity-60"
                onClick={approve}
                disabled={approving}
              >
                <UserAvatar
                  size="lg"
                  className="size-12 ring-4 ring-primary/10"
                  name={user.name}
                  src={user.avatar}
                  fallbackClassName="bg-primary/10 font-semibold text-primary"
                >
                  <AvatarBadge className="size-4">
                    <Check className="size-2.5" />
                  </AvatarBadge>
                </UserAvatar>
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-semibold">{user.name}</span>
                  <span className="mt-1 block truncate text-sm text-muted-foreground">
                    {user.email}
                  </span>
                </span>
                <ChevronRight className="size-5 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5 group-hover:text-primary" />
              </button>

              <Separator />

              <button
                type="button"
                className="group flex w-full items-center gap-4 p-5 text-left transition-colors hover:bg-muted/60 focus-visible:bg-muted/60 focus-visible:outline-none disabled:pointer-events-none disabled:opacity-60"
                onClick={useAnotherAccount}
                disabled={approving}
              >
                <span className="flex size-12 shrink-0 items-center justify-center rounded-full bg-muted text-primary">
                  <CircleUserRoundIcon className="size-5" />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block font-semibold text-primary">
                    Gunakan akun lain
                  </span>
                  <span className="mt-1 block text-sm text-muted-foreground">
                    Masuk dengan akun IPNU IPPNU ID yang berbeda
                  </span>
                </span>
                <ChevronRight className="size-5 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5 group-hover:text-primary" />
              </button>
            </div>

            <Button
              variant="ghost"
              className="w-full text-muted-foreground"
              onClick={cancel}
              disabled={approving}
            >
              <X />
              Batal
            </Button>
          </CardContent>
        </Card>
      </main>
    );
  }

  return (
    <main className="flex min-h-svh items-center justify-center bg-muted/30 p-4 sm:p-8">
      <Card className="w-full max-w-lg shadow-xl shadow-primary/5">
        <CardHeader className="space-y-5">
          <Brand />
          <div>
            <CardTitle className="text-xl">
              {client && !client.consent_required
                ? "Pilih akun"
                : "Izinkan akses aplikasi"}
            </CardTitle>
            <CardDescription className="mt-2">
              {client && !client.consent_required
                ? `Pilih akun untuk melanjutkan ke ${client.name}.`
                : "Tinjau identitas aplikasi dan scope sebelum melanjutkan."}
            </CardDescription>
          </div>
        </CardHeader>
        <CardContent className="space-y-5">
          {error && (
            <Alert variant="destructive">
              <AlertCircle />
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}
          {client && (
            <>
              <div className="flex items-center gap-3 rounded-xl border bg-muted/30 p-4">
                <span className="flex size-11 items-center justify-center rounded-xl bg-primary/10 text-primary">
                  <AppWindow />
                </span>
                <div>
                  <p className="font-semibold">{client.name}</p>
                  <p className="text-sm text-muted-foreground">
                    {client.description || "Aplikasi terdaftar IPNU IPPNU ID"}
                  </p>
                </div>
              </div>
              {client.consent_required && (
                <div>
                  <p className="mb-3 text-sm font-medium">Aplikasi meminta:</p>
                  <div className="space-y-2">
                    {request.scope
                      .split(" ")
                      .filter(Boolean)
                      .map((scope) => (
                        <div
                          key={scope}
                          className="flex items-center gap-3 rounded-lg border p-3 text-sm"
                        >
                          <Check className="size-4 text-primary" />
                          <span className="flex-1">
                            {scope === "email"
                              ? "Alamat email"
                              : scope === "profile"
                                ? "Profil dasar"
                                : scope === "openid"
                                  ? "Identitas OpenID"
                                  : scope}
                          </span>
                          <Badge variant="secondary">{scope}</Badge>
                        </div>
                      ))}
                  </div>
                </div>
              )}
            </>
          )}
          <Separator />
          <div className="flex items-center gap-3">
            <UserAvatar name={user.name} src={user.avatar} />
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium">
                Masuk sebagai {user.name}
              </p>
              <p className="truncate text-xs text-muted-foreground">{user.email}</p>
            </div>
            <ShieldCheck className="hidden size-5 shrink-0 text-primary sm:block" />
            <Button
              variant="ghost"
              size="sm"
              onClick={useAnotherAccount}
              disabled={approving}
            >
              <CircleUserRoundIcon />
              Ganti akun
            </Button>
          </div>
        </CardContent>
        <CardFooter className="grid grid-cols-2 gap-3">
          <Button
            variant="outline"
            onClick={cancel}
            disabled={!client || approving}
          >
            <X />
            {client?.consent_required ? "Tolak" : "Batal"}
          </Button>
          <Button onClick={approve} disabled={!client || approving}>
            {approving ? <Spinner /> : <Check />}
            {approving
              ? "Memproses..."
              : client && !client.consent_required
                ? "Lanjut"
                : "Izinkan"}
          </Button>
        </CardFooter>
      </Card>
    </main>
  );
}

export default function AuthorizePage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-svh items-center justify-center">
          <Spinner className="size-6" />
        </div>
      }
    >
      <AuthorizeContent />
    </Suspense>
  );
}
