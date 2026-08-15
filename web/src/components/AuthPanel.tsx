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
    return <p className="panel__status">{t(locale, "common.loading")}</p>;
  }

  if (user) {
    return (
      <div className="auth auth--signedin">
        <p>
          {t(locale, "auth.signedInAs")} <strong>{user.email}</strong>
        </p>
        <button type="button" className="btn" onClick={() => void signOut()}>
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

  const onRequestReset = () =>
    void run(async () => {
      const res = await requestReset(email);
      if (res.reset_token) setResetToken(res.reset_token);
      setNotice(t(locale, "auth.reset.sent"));
    });

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
    <div className="auth">
      <div className="auth__tabs" role="tablist">
        <button
          type="button"
          role="tab"
          aria-selected={mode === "signin"}
          className={mode === "signin" ? "tab tab--active" : "tab"}
          onClick={() => setMode("signin")}
        >
          {t(locale, "auth.tab.signin")}
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={mode === "signup"}
          className={mode === "signup" ? "tab tab--active" : "tab"}
          onClick={() => setMode("signup")}
        >
          {t(locale, "auth.tab.signup")}
        </button>
      </div>

      <form className="auth__form" onSubmit={onSubmit}>
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
          <label className="auth__consent">
            <input
              type="checkbox"
              required
              checked={consent}
              onChange={(e) => setConsent(e.target.checked)}
            />
            {t(locale, "auth.consent")}
          </label>
        )}
        <button type="submit" className="btn btn--primary" disabled={busy}>
          {mode === "signup" ? t(locale, "auth.signup") : t(locale, "auth.signin")}
        </button>
      </form>

      <button
        type="button"
        className="linklike"
        onClick={() => setShowReset((v) => !v)}
      >
        {t(locale, "auth.reset.toggle")}
      </button>

      {showVerify && (
        <form className="auth__form auth__verify" onSubmit={onVerify}>
          <h3>{t(locale, "auth.verify.heading")}</h3>
          <label>
            {t(locale, "auth.verify.token")}
            <input
              type="text"
              required
              value={verifyToken}
              onChange={(e) => setVerifyToken(e.target.value)}
            />
          </label>
          <button type="submit" className="btn" disabled={busy}>
            {t(locale, "auth.verify.submit")}
          </button>
        </form>
      )}

      {showReset && (
        <div className="auth__reset">
          <h3>{t(locale, "auth.reset.heading")}</h3>
          <button type="button" className="btn" disabled={busy} onClick={onRequestReset}>
            {t(locale, "auth.reset.request")}
          </button>
          <form className="auth__form" onSubmit={onConfirmReset}>
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
            <button type="submit" className="btn" disabled={busy}>
              {t(locale, "auth.reset.submit")}
            </button>
          </form>
        </div>
      )}

      {error && (
        <p className="panel__status panel__status--error" role="alert">
          {error}
        </p>
      )}
      {notice && <p className="panel__status">{notice}</p>}
    </div>
  );
}
