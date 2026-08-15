import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import { logout, me, type User } from "../api/auth";
import { clearExtensionState } from "../api/openfill";

interface SessionValue {
  user: User | null;
  loading: boolean;
  /** Re-fetch the current user (call after login/verify). */
  refresh: () => Promise<void>;
  /** Log out and clear the local user. */
  signOut: () => Promise<void>;
}

const SessionContext = createContext<SessionValue | undefined>(undefined);

/** Provides the current authenticated user to the app, backed by /auth/me. */
export function SessionProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    setUser(await me());
  }, []);

  const signOut = useCallback(async () => {
    await logout();
    await clearExtensionState();
    setUser(null);
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    me(controller.signal)
      .then((u) => setUser(u))
      .finally(() => setLoading(false));
    return () => controller.abort();
  }, []);

  return (
    <SessionContext.Provider value={{ user, loading, refresh, signOut }}>
      {children}
    </SessionContext.Provider>
  );
}

/** Access the current session. Must be used within a SessionProvider. */
export function useSession(): SessionValue {
  const ctx = useContext(SessionContext);
  if (!ctx) {
    throw new Error("useSession must be used within a SessionProvider");
  }
  return ctx;
}
