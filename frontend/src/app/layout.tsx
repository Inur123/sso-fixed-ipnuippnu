import type { Metadata } from "next";
import { cookies } from "next/headers";
import "./globals.css";
import { AuthProvider } from "@/components/auth-provider";
import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { PUBLIC_APP_DESCRIPTION, PUBLIC_APP_NAME, PUBLIC_BACKEND_URL } from "@/lib/public-env";
import { BACKEND_SESSION_COOKIE_NAME } from "@/lib/server-env";
import type { User } from "@/lib/api";

export const metadata: Metadata = {
  title: {
    default: PUBLIC_APP_NAME,
    template: `%s | ${PUBLIC_APP_NAME}`,
  },
  description: PUBLIC_APP_DESCRIPTION,
  applicationName: PUBLIC_APP_NAME,
  keywords: ["IPNU", "IPPNU", "SSO", "OAuth 2.0", "OpenID Connect"],
  icons: {
    icon: "/images/logo-sso.png",
    shortcut: "/images/logo-sso.png",
    apple: "/images/logo-sso.png",
  },
};

async function getInitialUser(): Promise<User | null> {
  const cookieStore = await cookies();
  const sessionCookie = cookieStore.get(BACKEND_SESSION_COOKIE_NAME);
  if (!sessionCookie?.value) return null;

  try {
    const response = await fetch(`${PUBLIC_BACKEND_URL}/api/auth/session`, {
      headers: {
        Cookie: `${BACKEND_SESSION_COOKIE_NAME}=${encodeURIComponent(sessionCookie.value)}`,
      },
      cache: "no-store",
      signal: AbortSignal.timeout(5000),
    });
    if (!response.ok) return null;
    const data = (await response.json()) as { user?: User | null };
    return data.user ?? null;
  } catch {
    return null;
  }
}

export default async function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const initialUser = await getInitialUser();

  return (
    <html lang="id" className="font-sans" suppressHydrationWarning>
      <body>
        <AuthProvider initialUser={initialUser}>
          <TooltipProvider>{children}</TooltipProvider>
        </AuthProvider>
        <Toaster position="top-right" />
      </body>
    </html>
  );
}
