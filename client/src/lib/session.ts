import { reactive } from "vue";
import type { Profile, SessionUser } from "./types";
import { apiFetch, postJSON, setAuthLostHandler } from "./http";

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

// Single source of truth for the auth/session state, mirrored from /api/session.
// Auth is owned by this app: /api/auth/register and /api/auth/login set the
// cookie pair, /api/auth/refresh rotates it, /api/auth/logout revokes it.
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
    // noRetry: a 401 here is the normal anonymous case, not an expired session —
    // /api/session answers 200 either way, so only a real error lands here.
    adopt(await apiFetch<SessionResponse>("/api/session", { noRetry: true }));
  } catch {
    clear();
    session.loaded = true;
  }
}

export async function register(email: string, password: string, name: string) {
  adopt(await postJSON<SessionResponse>("/api/auth/register", { email, password, name }, { noRetry: true }));
}

export async function login(email: string, password: string) {
  adopt(await postJSON<SessionResponse>("/api/auth/login", { email, password }, { noRetry: true }));
}

export async function changePassword(current: string, next: string) {
  adopt(await postJSON<SessionResponse>("/api/auth/password", { current, new: next }));
}

// Google sign-in needs a top-level navigation — a consent screen cannot render
// inside a fetch. The server sets the state cookie and redirects on from there.
export function loginWithGoogle(returnTo = currentPath()) {
  window.location.assign(`/api/auth/google?return=${encodeURIComponent(returnTo)}`);
}

export async function logout() {
  try {
    await apiFetch("/api/auth/logout", { method: "POST", noRetry: true });
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
