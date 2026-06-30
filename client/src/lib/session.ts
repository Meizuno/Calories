import { reactive } from "vue";
import type { Profile } from "./types";

interface SessionState {
  loaded: boolean;
  authenticated: boolean;
  profile: Profile | null;
}

// Single source of truth for the auth/session state, mirrored from /api/session.
// Login/logout are owned by our own backend (/api/login redirects to the central
// auth; /api/logout revokes + clears cookies) — the client just calls them.
export const session = reactive<SessionState>({
  loaded: false,
  authenticated: false,
  profile: null,
});

export async function loadSession(force = false) {
  if (session.loaded && !force) return;
  try {
    const res = await fetch("/api/session");
    const d = await res.json();
    session.authenticated = !!d.authenticated;
    session.profile = d.profile || null;
  } catch {
    session.authenticated = false;
    session.profile = null;
  } finally {
    session.loaded = true;
  }
}

function returnHere() {
  return encodeURIComponent(window.location.pathname + window.location.search);
}

// Login needs a top-level navigation (the OAuth flow can't run in a fetch); the
// server turns /api/login into a redirect to the central auth with our return URL.
export function login() {
  window.location.assign(`/api/login?return=${returnHere()}`);
}
export const redirectToLogin = login;

export async function logout() {
  try {
    await fetch("/api/logout", { method: "POST" });
  } catch {
    // ignore network errors — still drop local state below
  }
  session.authenticated = false;
  session.profile = null;
  window.location.assign("/");
}
