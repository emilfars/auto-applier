import { useState } from "react";
import { t, type Locale } from "../i18n";
import { useSession } from "../auth/session";
import {
  confirmReset,
  login,
  register,
  requestReset,
  verify,
  type RegisterResult,
} from "../api/auth";
import { ApiError } from "../api/http";

type Mode = "signin" | "signup";

/**
 * AuthPanel drives the seeker account flow (AUTH-1/3): sign up (with required
 * consent), email verification, sign in, and password reset. Google OAuth is
 * deferred. When the backend runs in local demo mode it returns the
 * verification token, which we surface so the whole flow works without an email
 * service.
 */
export function AuthPanel({ locale }: { locale: Locale }) {
  const { user, loading, refresh, signOut } = useSession();
  const [mode, setMode] = useState<Mode>("signin");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [consent, setConsent] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [verifyToken, setVerifyToken] = useState("");
  const [showVerify, setShowVerify] = useState(false);
  const [showReset, setShowReset] = useState(false);
  const [resetToken, setResetToken] = useState("");
  const [newPassword, setNewPassword] = useState("");

  if (loading) {
    return (
      <div className="panel__status grid animate-pulse gap-3" aria-live="polite">
        <span className="h-4 w-36 rounded bg-brand-surface-2" />
        <span className="h-10 rounded-xl bg-brand-surface-2" />
        <span className="h-10 rounded-xl bg-brand-surface-2" />
        <span className="sr-only">{t(locale, "common.loading")}</span>
      </div>
    );
  }

  if (user) {
    return (
      <div className="auth auth--signedin flex flex-wrap items-center justify-between gap-4">
        <p className="m-0 text-sm text-brand-muted">
          {t(locale, "auth.signedInAs")} <strong>{user.email}</strong>
        </p>
        <button
          type="button"
          className="btn rounded-xl border border-brand-border bg-brand-surface-2 px-4 py-2 font-semibold text-brand-text transition hover:border-brand-primary disabled:cursor-not-allowed disabled:opacity-60"
          onClick={() => void signOut()}
        >
          {t(locale, "auth.signout")}
        </button>
      </div>
    );
  }

  async function run(fn: () => Promise<void>) {
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await fn();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t(locale, "auth.error"));
    } finally {
      setBusy(false);
    }
  }

  const onSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (mode === "signup") {
      void run(async () => {
        const res: RegisterResult = await register(email, password, consent);
        setShowVerify(true);
        if (res.verification_token) {
          setVerifyToken(res.verification_token);
        }
        setNotice(t(locale, "auth.needVerify"));
      });
    } else {
      void run(async () => {
        await login(email, password);
        await refresh();
      });
    }
  };

  const onVerify = (e: React.FormEvent) => {
    e.preventDefault();
    void run(async () => {
      await verify(verifyToken);
      setShowVerify(false);
      setNotice(t(locale, "auth.verify.done"));
      setMode("signin");
    });
  };

  const onRequestReset = () => {
    if (!email.trim()) return;
    void run(async () => {
      const res = await requestReset(email);
      if (res.reset_token) setResetToken(res.reset_token);
      setNotice(t(locale, "auth.reset.sent"));
    });
  };

  const onConfirmReset = (e: React.FormEvent) => {
    e.preventDefault();
    void run(async () => {
      await confirmReset(resetToken, newPassword);
      setShowReset(false);
      setNotice(t(locale, "auth.reset.done"));
      setMode("signin");
    });
  };

  return (
    <div className="auth max-w-xl">
      <div className="auth__tabs grid grid-cols-2 gap-2">
        {(["signin", "signup"] as const).map((m) => (
          <button
            key={m}
            type="button"
            aria-pressed={mode === m}
            className={
              mode === m
                ? "tab tab--active rounded-xl border border-brand-primary bg-brand-primary-soft px-4 py-2 font-semibold text-brand-text transition"
                : "tab rounded-xl border border-brand-border bg-brand-surface-2 px-4 py-2 font-semibold text-brand-muted transition hover:border-brand-primary"
            }
            onClick={() => setMode(m)}
          >
            {t(locale, m === "signin" ? "auth.tab.signin" : "auth.tab.signup")}
          </button>
        ))}
      </div>

      <form
        className="auth__form grid gap-4 [&>label]:grid [&>label]:gap-2 [&>label]:text-sm [&>label]:font-medium [&>label]:text-brand-muted"
        onSubmit={onSubmit}
      >
        <label>
          {t(locale, "auth.email")}
          <input
            type="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            autoComplete="email"
          />
        </label>
        <label>
          {t(locale, "auth.password")}
          <input
            type="password"
            required
            minLength={8}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete={mode === "signup" ? "new-password" : "current-password"}
          />
        </label>
        {mode === "signup" && (
          <label className="auth__consent !flex !grid-cols-none items-start gap-2 text-xs font-normal">
            <input
              type="checkbox"
              required
              checked={consent}
              onChange={(e) => setConsent(e.target.checked)}
            />
            {t(locale, "auth.consent")}
          </label>
        )}
        <button
          type="submit"
          className="btn btn--primary rounded-xl border border-brand-primary-strong bg-brand-primary-strong px-4 py-2.5 font-semibold text-brand-on-primary transition hover:bg-brand-primary disabled:cursor-not-allowed disabled:opacity-60"
          disabled={busy}
        >
          {mode === "signup" ? t(locale, "auth.signup") : t(locale, "auth.signin")}
        </button>
      </form>

      <button
        type="button"
        className="linklike mt-1 inline-flex text-sm font-semibold text-brand-accent-light hover:underline"
        onClick={() => setShowReset((v) => !v)}
      >
        {t(locale, "auth.reset.toggle")}
      </button>

      {showVerify && (
        <form
          className="auth__form auth__verify mt-5 grid gap-4 border-t border-brand-border pt-5 [&>label]:grid [&>label]:gap-2 [&>label]:text-sm [&>label]:font-medium [&>label]:text-brand-muted"
          onSubmit={onVerify}
        >
          <h3 className="m-0 text-base font-bold text-brand-text">{t(locale, "auth.verify.heading")}</h3>
          <label>
            {t(locale, "auth.verify.token")}
            <input
              type="text"
              required
              value={verifyToken}
              onChange={(e) => setVerifyToken(e.target.value)}
            />
          </label>
          <button
            type="submit"
            className="btn rounded-xl border border-brand-border bg-brand-surface-2 px-4 py-2 font-semibold text-brand-text transition hover:border-brand-primary disabled:cursor-not-allowed disabled:opacity-60"
            disabled={busy}
          >
            {t(locale, "auth.verify.submit")}
          </button>
        </form>
      )}

      {showReset && (
        <div className="auth__reset mt-5 grid gap-4 border-t border-brand-border pt-5">
          <h3 className="m-0 text-base font-bold text-brand-text">{t(locale, "auth.reset.heading")}</h3>
          <button
            type="button"
            className="btn w-fit rounded-xl border border-brand-border bg-brand-surface-2 px-4 py-2 font-semibold text-brand-text transition hover:border-brand-primary disabled:cursor-not-allowed disabled:opacity-60"
            disabled={busy}
            onClick={onRequestReset}
          >
            {t(locale, "auth.reset.request")}
          </button>
            <form
              className="auth__form grid gap-4 [&>label]:grid [&>label]:gap-2 [&>label]:text-sm [&>label]:font-medium [&>label]:text-brand-muted"
              onSubmit={onConfirmReset}
            >
            <label>
              {t(locale, "auth.reset.token")}
              <input
                type="text"
                required
                value={resetToken}
                onChange={(e) => setResetToken(e.target.value)}
              />
            </label>
            <label>
              {t(locale, "auth.reset.newpw")}
              <input
                type="password"
                required
                minLength={8}
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
              />
            </label>
            <button
              type="submit"
              className="btn rounded-xl border border-brand-border bg-brand-surface-2 px-4 py-2 font-semibold text-brand-text transition hover:border-brand-primary disabled:cursor-not-allowed disabled:opacity-60"
              disabled={busy}
            >
              {t(locale, "auth.reset.submit")}
            </button>
          </form>
        </div>
      )}

      {error && (
          <p className="panel__status panel__status--error mt-4 rounded-xl border border-brand-danger bg-brand-danger-soft px-4 py-3 text-sm text-brand-danger" role="alert">
          {error}
        </p>
      )}
      {notice && <p className="panel__status mt-4 rounded-xl border border-brand-success bg-brand-success-soft px-4 py-3 text-sm text-brand-success">{notice}</p>}
    </div>
  );
}
