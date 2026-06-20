import { url } from "./config";
import type { Deployment, Environment, Fanout, Instance, OverviewFanout, Summary } from "./types";

const TOKEN_KEY = "edpm_token";

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
    const msg =
      (data as { error?: string })?.error || (typeof data === "string" ? data : res.statusText);
    throw new ApiError(res.status, msg || "request failed");
  }
  return data as T;
}

// Path to the per-instance pass-through (deploy, env detail, SSE log streams).
export function instancePath(instanceID: number, edpPath: string): string {
  return `/api/instances/${instanceID}/edp${edpPath}`;
}

// envPayload prepares an Environment for create/update:
//   - drops server-managed fields. edp decodes created_at/updated_at into Go
//     time.Time, which rejects the empty strings a blank form carries (→ 400);
//     id/status/webhook_token are owned by edp and sending them would clobber.
//   - drops empty credentials so "leave blank to keep" preserves the stored ones
//     on edit (edp keeps a field it doesn't receive) instead of wiping them.
function envPayload(e: Environment): Record<string, unknown> {
  const copy: Record<string, unknown> = { ...e };
  for (const k of ["id", "status", "created_at", "updated_at", "webhook_token"]) delete copy[k];
  for (const k of ["git_token", "registry_password"]) if (!copy[k]) delete copy[k];
  return copy;
}

export const api = {
  // auth
  authStatus: () => request<{ configured: boolean }>("GET", "/api/auth"),
  login: (password: string) =>
    request<{ token: string; first_run: boolean }>("POST", "/api/login", { password }),

  // instance registry
  listInstances: () => request<Instance[]>("GET", "/api/instances"),
  createInstance: (i: Omit<Instance, "id">) => request<Instance>("POST", "/api/instances", i),
  updateInstance: (id: number, i: Instance) => request<Instance>("PUT", `/api/instances/${id}`, i),
  deleteInstance: (id: number) => request<{ status: string }>("DELETE", `/api/instances/${id}`),
  testInstance: (id: number) => request<{ ok: boolean; error?: string }>("POST", `/api/instances/${id}/test`),

  // aggregate (fan-out across all instances)
  overview: () => request<OverviewFanout>("GET", "/api/overview"),
  environments: () => request<Fanout>("GET", "/api/environments"),
  status: () => request<Fanout>("GET", "/api/status"),
  summary: () => request<Summary>("GET", "/api/summary"),

  // per-instance reads + actions (via pass-through)
  getEnv: (instanceID: number, envID: number) =>
    request<Environment>("GET", instancePath(instanceID, `/api/environments/${envID}`)),
  createEnv: (instanceID: number, e: Environment) =>
    request<Environment>("POST", instancePath(instanceID, "/api/environments"), envPayload(e)),
  updateEnv: (instanceID: number, envID: number, e: Environment) =>
    request<Environment>("PUT", instancePath(instanceID, `/api/environments/${envID}`), envPayload(e)),
  deleteEnv: (instanceID: number, envID: number) =>
    request<{ status: string }>("DELETE", instancePath(instanceID, `/api/environments/${envID}`)),
  listDeployments: (instanceID: number, envID: number) =>
    request<Deployment[]>("GET", instancePath(instanceID, `/api/environments/${envID}/deployments`)),
  getDeployment: (instanceID: number, depID: number) =>
    request<Deployment>("GET", instancePath(instanceID, `/api/deployments/${depID}`)),
  deploy: (instanceID: number, envID: number, reason: string) =>
    request<{ deployment_id: number }>(
      "POST",
      instancePath(instanceID, `/api/environments/${envID}/deploy${reason ? `?reason=${encodeURIComponent(reason)}` : ""}`),
    ),
};
