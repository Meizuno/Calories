import { reactive } from "vue";
import type { Profile, SessionUser } from "./types";
import { apiFetch, postJSON, setAuthLostHandler } from "./http";

const V1 = "/api/v1";

interface SessionState {
  loaded: boolean;
  authenticated: boolean;
  user: SessionUser | null;
  profile: Profile | null;
  /** Whether this deployment has Google sign-in configured. */
  google: boolean;
  /** Whether self-service sign-up is open (ALLOW_REGISTRATION on the server). */
  registration: boolean;
}

interface SessionResponse {
  authenticated: boolean;
  google?: boolean;
  registration?: boolean;
  user?: SessionUser;
  profile?: Profile;
}

// Single source of truth for the auth/session state, mirrored from the API's
// /session. Auth is owned by this app: /auth/register and /auth/login set the
// cookie pair, /auth/refresh rotates it, /auth/logout revokes it.
//
// The session endpoints answer inside a {"session": …} envelope, like every
// other route; `session` below unwraps it.
export const session = reactive<SessionState>({
  loaded: false,
  authenticated: false,
  user: null,
  profile: null,
  google: false,
  registration: false,
});

function adopt(d: SessionResponse) {
  session.authenticated = !!d.authenticated;
  session.user = d.user ?? null;
  session.profile = d.profile ?? null;
  if (typeof d.google === "boolean") session.google = d.google;
  if (typeof d.registration === "boolean") session.registration = d.registration;
  session.loaded = true;
}

function clear() {
  session.authenticated = false;
  session.user = null;
  session.profile = null;
}

export async function loadSession(force = false) {
  if (session.loaded && !force) return;
  try {
    // noRetry: /api/session answers 200 either way and renews the session itself
    // when the access token has expired, so a failure here is a real error.
    adopt((await apiFetch<{ session: SessionResponse }>(`${V1}/session`, { noRetry: true })).session);
  } catch {
    clear();
    session.loaded = true;
  }
}

export async function register(email: string, password: string, name: string) {
  adopt((await postJSON<{ session: SessionResponse }>(`${V1}/auth/register`, { email, password, name }, { noRetry: true })).session);
}

export async function login(email: string, password: string) {
  adopt((await postJSON<{ session: SessionResponse }>(`${V1}/auth/login`, { email, password }, { noRetry: true })).session);
}

export async function changePassword(current: string, next: string) {
  adopt((await postJSON<{ session: SessionResponse }>(`${V1}/auth/password`, { current, new: next })).session);
}

// Google sign-in needs a top-level navigation — a consent screen cannot render
// inside a fetch. The server sets the state cookie and redirects on from there.
export function loginWithGoogle(returnTo = currentPath()) {
  window.location.assign(`${V1}/auth/google?return=${encodeURIComponent(returnTo)}`);
}

export async function logout() {
  try {
    await apiFetch(`${V1}/auth/logout`, { method: "POST", noRetry: true });
  } catch {
    // Ignore network errors — the local state is dropped either way.
  }
  clear();
  window.location.assign("/");
}

/** Send the visitor to the sign-in page, remembering where they were headed. */
export function redirectToLogin(returnTo = currentPath()) {
  const back = returnTo && returnTo !== "/" ? `?return=${encodeURIComponent(returnTo)}` : "";
  window.location.assign(`/login${back}`);
}

function currentPath() {
  return window.location.pathname + window.location.search;
}

// When a refresh cannot revive the session, drop the stale state and ask for a
// fresh sign-in.
setAuthLostHandler(() => {
  clear();
  if (!window.location.pathname.startsWith("/login")) redirectToLogin();
});
