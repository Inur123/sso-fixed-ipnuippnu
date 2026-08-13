"use client";

import { Suspense, useEffect, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { AlertCircle, AppWindow, Check, ShieldCheck, X } from "lucide-react";

import { useAuth } from "@/components/auth-provider";
import { Brand } from "@/components/brand";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
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
import { APIError, apiFetch, getErrorMessage } from "@/lib/api";

interface ClientInfo {
  client_id: string;
  name: string;
  description: string;
  allowed_scopes: string[];
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
    }),
    [searchParams],
  );
  const complete =
    Object.entries(request).every(
      ([key, value]) => key === "nonce" || Boolean(value),
    ) &&
    request.response_type === "code" &&
    request.code_challenge_method === "S256" &&
    (!request.scope.split(" ").includes("openid") || Boolean(request.nonce));

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
    });
    apiFetch<ClientInfo>(`/api/oauth/client-info?${query.toString()}`)
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
    router,
    user,
  ]);

  async function approve() {
    setApproving(true);
    setError("");
    try {
      const data = await apiFetch<{ redirect_url: string }>(
        "/oauth/authorize",
        { method: "POST", body: JSON.stringify(request) },
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
      setError(getErrorMessage(approvalError));
      setApproving(false);
    }
  }

  function cancel() {
    if (!client) return;
    const target = new URL(request.redirect_uri);
    target.searchParams.set("error", "access_denied");
    target.searchParams.set("state", request.state);
    window.location.assign(target.toString());
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

  const initials = user.name
    .split(" ")
    .map((part) => part[0])
    .join("")
    .slice(0, 2)
    .toUpperCase();
  return (
    <main className="flex min-h-svh items-center justify-center bg-muted/30 p-4 sm:p-8">
      <Card className="w-full max-w-lg shadow-xl shadow-primary/5">
        <CardHeader className="space-y-5">
          <Brand />
          <div>
            <CardTitle className="text-xl">Izinkan akses aplikasi</CardTitle>
            <CardDescription className="mt-2">
              Tinjau identitas aplikasi dan scope sebelum melanjutkan.
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
            </>
          )}
          <Separator />
          <div className="flex items-center gap-3">
            <Avatar>
              <AvatarFallback>{initials}</AvatarFallback>
            </Avatar>
            <div>
              <p className="text-sm font-medium">Masuk sebagai {user.name}</p>
              <p className="text-xs text-muted-foreground">{user.email}</p>
            </div>
            <ShieldCheck className="ml-auto size-5 text-primary" />
          </div>
        </CardContent>
        <CardFooter className="grid grid-cols-2 gap-3">
          <Button
            variant="outline"
            onClick={cancel}
            disabled={!client || approving}
          >
            <X />
            Tolak
          </Button>
          <Button onClick={approve} disabled={!client || approving}>
            {approving ? <Spinner /> : <Check />}
            {approving ? "Mengizinkan..." : "Izinkan"}
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
