// Shared fetch helper for the backend API. All calls send credentials so the
// HttpOnly session cookie set at login rides along, and non-2xx responses are
// turned into a typed ApiError carrying the server's message.

export class ApiError extends Error {
  readonly status: number;
  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

interface RequestOptions {
  method?: string;
  /** JSON body; serialized automatically. Omit for GET. */
  body?: unknown;
  /** Raw body (e.g. FormData) used instead of a JSON body. */
  rawBody?: BodyInit;
  signal?: AbortSignal;
}

/**
 * apiFetch issues an API request with credentials and JSON handling. It parses
 * a JSON response when present, and throws ApiError on a non-ok status using
 * the server's `error` field when available.
 */
export async function apiFetch<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = { Accept: "application/json" };
  let body: BodyInit | undefined;
  if (opts.rawBody !== undefined) {
    body = opts.rawBody;
  } else if (opts.body !== undefined) {
    headers["Content-Type"] = "application/json";
    body = JSON.stringify(opts.body);
  }

  const res = await fetch(path, {
    method: opts.method ?? (body ? "POST" : "GET"),
    headers,
    body,
    credentials: "include",
    signal: opts.signal,
  });

  const text = await res.text();
  const data = text ? (JSON.parse(text) as unknown) : null;
  if (!res.ok) {
    const msg =
      (data && typeof data === "object" && "error" in data
        ? String((data as { error: unknown }).error)
        : null) ?? `request failed: ${res.status}`;
    throw new ApiError(res.status, msg);
  }
  return data as T;
}
