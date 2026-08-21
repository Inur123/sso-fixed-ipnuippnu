import { PUBLIC_BACKEND_URL } from "@/lib/public-env";

export const API_URL = PUBLIC_BACKEND_URL;

export type Role = "super_admin" | "anggota";

export interface User {
  id: string;
  email: string;
  name: string;
  phone: string;
  bio: string;
  gender: "male" | "female" | "other" | "";
  avatar: string;
  role: Role;
  is_active: boolean;
  email_verified: boolean;
  email_verified_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface OAuthClient {
  client_id: string;
  client_secret?: string;
  secret_available: boolean;
  secret_version: number;
  name: string;
  description: string;
  redirect_uris: string[];
  allowed_scopes: string[];
  owner_id: string;
  access_policy: "assigned_only" | "all_active_users";
  status: "active" | "suspended";
  assignment_count: number;
}

export interface OAuthClientAssignment {
  id: string;
  user_id: string;
  name: string;
  email: string;
  avatar: string;
}

export interface AssignableUser {
  user_id: string;
  name: string;
  email: string;
  avatar: string;
}

export interface ClientAssignmentsResponse {
  client: OAuthClient;
  assignments: OAuthClientAssignment[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}

interface APIErrorBody {
  error?: string;
  message?: string;
  error_description?: string;
}

export class APIError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly code?: string,
    public readonly payload?: APIErrorBody & Record<string, unknown>,
  ) {
    super(message);
    this.name = "APIError";
  }
}

export async function apiFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body && !(init.body instanceof FormData) && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  const response = await fetch(`${API_URL}${path}`, {
    ...init,
    headers,
    credentials: "include",
    cache: "no-store",
  });
  const contentType = response.headers.get("content-type") ?? "";
  const payload = contentType.includes("application/json")
    ? ((await response.json()) as APIErrorBody & Record<string, unknown>)
    : undefined;
  if (!response.ok) {
    throw new APIError(
      payload?.message ?? payload?.error_description ?? "Permintaan tidak dapat diproses.",
      response.status,
      payload?.error,
      payload,
    );
  }
  return payload as T;
}

export function getErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Terjadi kesalahan yang tidak diketahui.";
}
