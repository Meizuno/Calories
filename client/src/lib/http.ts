// The single place every API call goes through.
//
// Auth is two cookies, both HttpOnly, so this file can never see them: a
// short-lived access JWT sent with every request, and a refresh token scoped to
// /api/auth. When the access token expires the server answers 401; we call
// /api/auth/refresh once, then replay the original request. If the refresh also
// fails the session is genuinely over and onAuthLost() takes over.

type AuthLostHandler = () => void;

let onAuthLost: AuthLostHandler = () => {};

/** Registered by the session store — called when a refresh cannot recover. */
export function setAuthLostHandler(fn: AuthLostHandler) {
  onAuthLost = fn;
}

// One shared refresh across concurrent 401s: a page that fires five requests at
// once must not rotate the refresh token five times. Rotation invalidates the
// previous token, so parallel refreshes would revoke each other and trip the
// server's replay detection, logging the user out for no reason.
let refreshing: Promise<boolean> | null = null;

function refresh(): Promise<boolean> {
  if (!refreshing) {
    refreshing = fetch("/api/auth/refresh", { method: "POST" })
      .then((r) => r.ok)
      .catch(() => false)
      .finally(() => {
        refreshing = null;
      });
  }
  return refreshing;
}

export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
    /** Stable code from the API ("bad_credentials", …), translated for display. */
    readonly code = "unknown",
  ) {
    super(message);
  }
}

interface Options extends RequestInit {
  /** Skip the refresh-and-retry dance (used by the auth endpoints themselves). */
  noRetry?: boolean;
}

/**
 * Fetch an API route and parse the JSON body. Throws ApiError with the server's
 * message on failure, so views can surface it directly.
 */
export async function apiFetch<T>(path: string, opts: Options = {}): Promise<T> {
  const { noRetry, ...init } = opts;
  let res = await fetch(path, init);

  if (res.status === 401 && !noRetry) {
    if (await refresh()) {
      res = await fetch(path, init);
    }
    if (res.status === 401) {
      onAuthLost();
      throw new ApiError(401, "session expired", "session_expired");
    }
  }

  if (!res.ok) {
    // Errors come back as {code, message}; fall back to the raw body for any
    // route that still replies in plain text.
    const body = (await res.text()).trim();
    let code = "unknown";
    let message = body || `Request failed (${res.status})`;
    try {
      const parsed = JSON.parse(body) as { code?: string; message?: string };
      if (parsed.code) code = parsed.code;
      if (parsed.message) message = parsed.message;
    } catch {
      // not JSON — keep the raw text
    }
    throw new ApiError(res.status, message, code);
  }
  // 204 No Content has no body to parse.
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export const JSON_HEADERS = { "Content-Type": "application/json" };

/** POST a JSON body. */
export function postJSON<T>(path: string, body: unknown, opts: Options = {}) {
  return apiFetch<T>(path, { method: "POST", headers: JSON_HEADERS, body: JSON.stringify(body), ...opts });
}
