export type Status = "created" | "photo_ok" | "liveness_started" | "approved" | "review" | "rejected";
export type FinalStatus = "approved" | "review" | "rejected";

// Open-ended so an unknown code from a newer server gets fallback copy.
export type Reason = string;

export interface Thresholds {
  approveLiveness: number;
  approveSimilarity: number;
  reviewLiveness: number;
  reviewSimilarity: number;
}

export interface Verification {
  id: string;
  status: Status;
  createdAt: string;
  expiresAt: string;
  livenessAttempts: number;
  maxLivenessAttempts: number;
  livenessConfidence?: number;
  similarity?: number;
  reasons: Reason[];
  thresholds: Thresholds;
  photoUrl?: string;
  referenceUrl?: string;
}

export interface Created {
  id: string;
  status: Status;
  expiresAt: string;
  uploadUrl: string;
  uploadHeaders: Record<string, string>;
  uploadExpiresAt: string;
}

export interface Session {
  sessionId: string;
  attempt: number;
  maxAttempts: number;
}

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly reasons: Reason[];

  constructor(status: number, code: string, message: string, reasons: Reason[] = []) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.reasons = reasons;
  }
}

async function request<T>(method: "GET" | "POST", path: string): Promise<T> {
  let res: Response;
  try {
    res = await fetch(path, { method, headers: { Accept: "application/json" } });
  } catch {
    throw new ApiError(0, "network", "Can't reach the server");
  }
  // Non-JSON (e.g. the dev proxy while Go is down) means we never reached the API.
  const body: unknown = await res.json().catch(() => null);
  if (body === null) throw new ApiError(res.status, "network", "Can't reach the server");
  if (!res.ok) {
    const e = body as { error?: string; code?: string; reasons?: Reason[] };
    throw new ApiError(res.status, e.code ?? "internal", e.error ?? res.statusText, e.reasons ?? []);
  }
  return body as T;
}

const base = "/api/verifications";

export const api = {
  create: () => request<Created>("POST", base),
  get: (id: string) => request<Verification>("GET", `${base}/${id}`),
  checkPhoto: (id: string) => request<{ id: string; status: Status }>("POST", `${base}/${id}/photo`),
  startLiveness: (id: string) => request<Session>("POST", `${base}/${id}/liveness-session`),
  complete: (id: string) => request<Verification>("POST", `${base}/${id}/complete`),

  async upload(created: Created, photo: Blob): Promise<void> {
    let res: Response;
    try {
      res = await fetch(created.uploadUrl, { method: "PUT", headers: created.uploadHeaders, body: photo });
    } catch {
      // CORS rejections land here too.
      throw new ApiError(0, "upload_failed", "Upload failed");
    }
    if (!res.ok) throw new ApiError(res.status, "upload_failed", `Upload failed (${res.status})`);
  },
};

export function isFinal(status: Status): status is FinalStatus {
  return status === "approved" || status === "review" || status === "rejected";
}

export function isValidId(id: string): boolean {
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(id);
}
