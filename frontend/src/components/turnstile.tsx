"use client";

import Script from "next/script";
import { useCallback, useEffect, useRef, useState } from "react";
import { AlertCircle } from "lucide-react";

import { Skeleton } from "@/components/ui/skeleton";
import { PUBLIC_TURNSTILE_SITE_KEY } from "@/lib/public-env";

type TurnstileWidgetID = string;

interface TurnstileRenderOptions {
  sitekey: string;
  action: string;
  language: string;
  size: "normal";
  theme: "light";
  retry: "auto";
  callback: (token: string) => void;
  "error-callback": () => boolean;
  "expired-callback": () => void;
  "timeout-callback": () => void;
  "unsupported-callback": () => void;
  "before-interactive-callback": () => void;
}

interface TurnstileAPI {
  render: (
    container: HTMLElement,
    options: TurnstileRenderOptions,
  ) => TurnstileWidgetID;
  remove: (widgetID: TurnstileWidgetID) => void;
  reset: (widgetID: TurnstileWidgetID) => void;
}

declare global {
  interface Window {
    turnstile?: TurnstileAPI;
  }
}

function TurnstileSkeleton() {
  return (
    <div
      className="absolute inset-0 flex h-[65px] w-[300px] items-center justify-between border bg-background px-3.5"
      role="status"
      aria-label="Memuat verifikasi keamanan"
    >
      <div className="flex items-center gap-3">
        <Skeleton className="size-10 shrink-0 rounded-lg" />
        <span className="grid gap-2">
          <Skeleton className="h-3 w-32" />
          <Skeleton className="h-2.5 w-24" />
        </span>
      </div>
      <div className="grid justify-items-center gap-1.5">
        <Skeleton className="size-9 rounded-lg" />
        <Skeleton className="h-2 w-12" />
      </div>
      <span className="sr-only">Turnstile sedang dimuat</span>
    </div>
  );
}

export function Turnstile({
  onTokenChange,
  resetKey,
}: {
  onTokenChange: (token: string) => void;
  resetKey: number;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const widgetIDRef = useRef<TurnstileWidgetID | null>(null);
  const previousResetKeyRef = useRef(resetKey);
  const [widgetRendered, setWidgetRendered] = useState(false);
  const [loadError, setLoadError] = useState("");

  const renderWidget = useCallback(() => {
    if (!containerRef.current || !window.turnstile || widgetIDRef.current) return;

    setLoadError("");
    try {
      widgetIDRef.current = window.turnstile.render(containerRef.current, {
        sitekey: PUBLIC_TURNSTILE_SITE_KEY,
        action: "register",
        language: "id",
        size: "normal",
        theme: "light",
        retry: "auto",
        callback: (token) => {
          setWidgetRendered(true);
          setLoadError("");
          onTokenChange(token);
        },
        "before-interactive-callback": () => {
          setWidgetRendered(true);
        },
        "error-callback": () => {
          onTokenChange("");
          setLoadError("Verifikasi keamanan gagal dimuat. Periksa koneksi Anda.");
          return true;
        },
        "expired-callback": () => {
          onTokenChange("");
          setWidgetRendered(false);
          if (widgetIDRef.current) window.turnstile?.reset(widgetIDRef.current);
        },
        "timeout-callback": () => {
          onTokenChange("");
          setWidgetRendered(false);
          if (widgetIDRef.current) window.turnstile?.reset(widgetIDRef.current);
        },
        "unsupported-callback": () => {
          onTokenChange("");
          setLoadError("Browser ini belum mendukung verifikasi keamanan.");
        },
      });
    } catch {
      setWidgetRendered(false);
      setLoadError("Verifikasi keamanan gagal dimuat. Muat ulang halaman.");
    }
  }, [onTokenChange]);

  useEffect(() => {
    if (previousResetKeyRef.current === resetKey) return;
    previousResetKeyRef.current = resetKey;
    setLoadError("");
    setWidgetRendered(false);
    if (widgetIDRef.current) window.turnstile?.reset(widgetIDRef.current);
  }, [resetKey]);

  useEffect(() => {
    return () => {
      if (widgetIDRef.current) window.turnstile?.remove(widgetIDRef.current);
      widgetIDRef.current = null;
    };
  }, []);

  return (
    <div className="w-[300px] max-w-full space-y-2">
      <div
        className="relative min-h-[65px] w-[300px]"
        aria-busy={!widgetRendered}
      >
        {!widgetRendered && !loadError && <TurnstileSkeleton />}
        <div
          ref={containerRef}
          className={widgetRendered ? "min-h-[65px]" : "invisible min-h-[65px]"}
        />
      </div>
      {loadError && (
        <p className="flex items-start gap-2 text-xs leading-5 text-destructive" role="alert">
          <AlertCircle className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
          {loadError}
        </p>
      )}
      <Script
        id="cloudflare-turnstile"
        src="https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit"
        strategy="afterInteractive"
        onReady={renderWidget}
        onError={() => {
          setWidgetRendered(false);
          setLoadError("Verifikasi keamanan gagal dimuat. Muat ulang halaman.");
        }}
      />
    </div>
  );
}
