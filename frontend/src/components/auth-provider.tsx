"use client";

import { createContext, useCallback, useContext, useMemo, useState } from "react";

import { APIError, apiFetch, type User } from "@/lib/api";

interface AuthContextValue {
  user: User | null;
  loading: boolean;
  refresh: () => Promise<User | null>;
  setUser: (user: User | null) => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({
  children,
  initialUser,
}: {
  children: React.ReactNode;
  initialUser: User | null;
}) {
  const [user, setUser] = useState<User | null>(initialUser);
  const [loading, setLoading] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const data = await apiFetch<{ user: User | null }>("/api/auth/session");
      setUser(data.user);
      return data.user;
    } catch (error) {
      if (error instanceof APIError && error.status === 401) {
        setUser(null);
        return null;
      }
      throw error;
    } finally {
      setLoading(false);
    }
  }, []);

  const value = useMemo(() => ({ user, loading, refresh, setUser }), [user, loading, refresh]);
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) throw new Error("useAuth must be used inside AuthProvider");
  return context;
}
