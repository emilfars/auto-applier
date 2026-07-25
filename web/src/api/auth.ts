// Typed client for the auth API (AUTH-1/3/5). Sessions are held in an HttpOnly
// cookie set by the backend at login; the browser never sees the token, so the
// client just calls the endpoints and relies on the cookie for subsequent
// requests. Google OAuth is deferred (not surfaced here).

import { apiFetch } from "./http";

export interface User {
  id: string;
  email: string;
  verified: boolean;
}

export interface RegisterResult {
  id: string;
  email: string;
  verified: boolean;
  consent: boolean;
  /** Present only when the backend runs with AUTH_DEV_EXPOSE_TOKENS (local demos). */
  verification_token?: string;
}

/** Register a new account. Consent is required (UU PDP) and enforced server-side. */
export function register(email: string, password: string): Promise<RegisterResult> {
  return apiFetch<RegisterResult>("/auth/register", {
    body: { email, password, consent: true },
  });
}

/** Verify an account with the token from the verification email (or dev response). */
export function verify(token: string): Promise<{ status: string }> {
  return apiFetch<{ status: string }>("/auth/verify", { body: { token } });
}

/** Log in; on success the backend sets the session cookie. */
export function login(email: string, password: string): Promise<{ token: string }> {
  return apiFetch<{ token: string }>("/auth/login", { body: { email, password } });
}

/** Log out; clears the session cookie server-side. */
export function logout(): Promise<{ status: string }> {
  return apiFetch<{ status: string }>("/auth/logout", { method: "POST" });
}

/** Return the current user, or null when not authenticated/verified. */
export async function me(signal?: AbortSignal): Promise<User | null> {
  try {
    return await apiFetch<User>("/auth/me", { signal });
  } catch {
    return null;
  }
}

/** Request a password reset. The token is emailed (or returned in dev mode). */
export function requestReset(email: string): Promise<{ status: string; reset_token?: string }> {
  return apiFetch<{ status: string; reset_token?: string }>("/auth/password-reset/request", {
    body: { email },
  });
}

/** Complete a password reset with the token and a new password. */
export function confirmReset(token: string, newPassword: string): Promise<unknown> {
  return apiFetch<unknown>("/auth/password-reset/confirm", {
    body: { token, new_password: newPassword },
  });
}
