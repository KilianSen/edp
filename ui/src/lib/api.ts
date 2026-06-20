import { url } from "./config";
import type { Deployment, Environment, HookRun, OverviewItem, StatusItem, TimedHook } from "./types";

const TOKEN_KEY = "edp_token";

let token: string | null = localStorage.getItem(TOKEN_KEY);
let onUnauthorized: (() => void) | null = null;

export function getToken(): string | null {
  return token;
}
export function setToken(t: string): void {
  token = t;
  localStorage.setItem(TOKEN_KEY, t);
}
export function clearToken(): void {
  token = null;
  localStorage.removeItem(TOKEN_KEY);
}
// The auth provider registers a handler so a 401 anywhere bounces to login.
export function setUnauthorizedHandler(fn: () => void): void {
  onUnauthorized = fn;
}

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

function authHeaders(extra?: HeadersInit): HeadersInit {
  const h: Record<string, string> = {};
  if (token) h["Authorization"] = "Bearer " + token;
  return { ...h, ...(extra as Record<string, string>) };
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const init: RequestInit = { method, headers: authHeaders() };
  if (body !== undefined) {
    init.headers = authHeaders({ "Content-Type": "application/json" });
    init.body = JSON.stringify(body);
  }
  const res = await fetch(url(path), init);
  if (res.status === 401 && onUnauthorized) {
    onUnauthorized();
    throw new ApiError(401, "unauthorized");
  }
  const text = await res.text();
  let data: unknown = null;
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = text;
    }
  }
  if (!res.ok) {
    const msg = (data as { error?: string })?.error || (typeof data === "string" ? data : res.statusText);
    throw new ApiError(res.status, msg || "request failed");
  }
  return data as T;
}

// reqList wraps request for endpoints that return a JSON array, coalescing a
// null/undefined body to [] — Go marshals an empty slice as null, which would
// otherwise crash callers doing .map/.length on the result.
async function reqList<T>(method: string, path: string): Promise<T[]> {
  const data = await request<T[] | null>(method, path);
  return data ?? [];
}

// ---- auth ----
export const api = {
  authStatus: () => request<{ configured: boolean }>("GET", "/api/auth"),
  login: (password: string) =>
    request<{ token: string; first_run: boolean }>("POST", "/api/login", { password }),

  // ---- environments ----
  listEnvs: () => request<Environment[]>("GET", "/api/environments"),
  getEnv: (id: number) => request<Environment>("GET", `/api/environments/${id}`),
  createEnv: (e: Environment) => request<Environment>("POST", "/api/environments", e),
  updateEnv: (id: number, e: Environment) => request<Environment>("PUT", `/api/environments/${id}`, e),
  deleteEnv: (id: number) => request<{ status: string }>("DELETE", `/api/environments/${id}`),
  deploy: (id: number, reason: string) =>
    request<{ deployment_id: number }>(
      "POST",
      `/api/environments/${id}/deploy${reason ? `?reason=${encodeURIComponent(reason)}` : ""}`,
    ),
  rotateToken: (id: number) =>
    request<{ webhook_token: string }>("POST", `/api/environments/${id}/rotate-token`),

  // ---- deployments ----
  // Go marshals an empty slice as null, so list endpoints coalesce to [] to keep
  // callers' .map/.length/.find safe even against an older backend.
  listDeployments: (id: number) => reqList<Deployment>("GET", `/api/environments/${id}/deployments`),
  getDeployment: (id: number) => request<Deployment>("GET", `/api/deployments/${id}`),

  // ---- status ----
  overview: () => reqList<OverviewItem>("GET", "/api/overview"),
  status: () => reqList<StatusItem>("GET", "/api/status"),

  // ---- timed hooks ----
  listHooks: (envID: number) => reqList<TimedHook>("GET", `/api/environments/${envID}/hooks`),
  createHook: (envID: number, h: TimedHook) =>
    request<TimedHook>("POST", `/api/environments/${envID}/hooks`, h),
  getHook: (id: number) => request<TimedHook>("GET", `/api/hooks/${id}`),
  updateHook: (id: number, h: TimedHook) => request<TimedHook>("PUT", `/api/hooks/${id}`, h),
  deleteHook: (id: number) => request<{ status: string }>("DELETE", `/api/hooks/${id}`),
  runHook: (id: number) => request<{ run_id: number }>("POST", `/api/hooks/${id}/run`),
  listHookRuns: (id: number) => reqList<HookRun>("GET", `/api/hooks/${id}/runs`),
  getHookRun: (id: number) => request<HookRun>("GET", `/api/hook-runs/${id}`),

  // ---- import ----
  importBundle: (bundle: string) =>
    request<{ imported: number; names: string[] }>("POST", "/api/import", JSON.parse(bundle)),
};

// Export download URLs carry no auth header (anchor navigation), so they are
// fetched as blobs with the token instead. Returns an object URL the caller frees.
export async function downloadExport(path: string): Promise<{ href: string; revoke: () => void }> {
  const res = await fetch(url(path), { headers: authHeaders() });
  if (!res.ok) throw new ApiError(res.status, "export failed");
  const blob = await res.blob();
  const href = URL.createObjectURL(blob);
  return { href, revoke: () => URL.revokeObjectURL(href) };
}
