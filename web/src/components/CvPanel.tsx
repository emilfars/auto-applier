import { useEffect, useState } from "react";
import { t, type Locale } from "../i18n";
import { useSession } from "../auth/session";
import { listCVs, parseCV, uploadCV, type CVFile } from "../api/cv";
import { ApiError } from "../api/http";

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

/**
 * CvPanel lets a signed-in user upload a CV (stored encrypted at rest) and
 * parse it into their profile. Parsing is an assist — the profile is left
 * unconfirmed afterwards so the user reviews it before confirming (CV-1/2/5).
 * `onParsed` lets the parent refresh the profile view.
 */
export function CvPanel({
  locale,
  onParsed,
}: {
  locale: Locale;
  onParsed?: () => void;
}) {
  const { user } = useSession();
  const [files, setFiles] = useState<CVFile[]>([]);
  const [selected, setSelected] = useState<File | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  useEffect(() => {
    if (!user) {
      setFiles([]);
      return;
    }
    const controller = new AbortController();
    listCVs(controller.signal)
      .then(setFiles)
      .catch((err: unknown) => {
        if (err instanceof DOMException && err.name === "AbortError") return;
        setError(t(locale, "auth.error"));
      });
    return () => controller.abort();
  }, [user, locale]);

  if (!user) {
    return <p className="panel__status">{t(locale, "profile.loginRequired")}</p>;
  }

  async function run(fn: () => Promise<void>) {
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await fn();
    } catch (err) {
      if (err instanceof ApiError && err.status === 503) {
        setError(t(locale, "cv.parseUnavailable"));
      } else {
        setError(err instanceof ApiError ? err.message : t(locale, "auth.error"));
      }
    } finally {
      setBusy(false);
    }
  }

  const onUpload = (e: React.FormEvent) => {
    e.preventDefault();
    if (!selected) return;
    void run(async () => {
      const rec = await uploadCV(selected);
      setFiles((prev) => [rec, ...prev]);
      setSelected(null);
      setNotice(t(locale, "cv.uploaded"));
    });
  };

  const onParse = (id: string) =>
    void run(async () => {
      await parseCV(id);
      setNotice(t(locale, "cv.parsed"));
      onParsed?.();
    });

  return (
    <div className="cv">
      <form className="cv__upload" onSubmit={onUpload}>
        <label>
          {t(locale, "cv.selectFile")}
          <input
            type="file"
            accept=".pdf,.docx,application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document"
            onChange={(e) => setSelected(e.target.files?.[0] ?? null)}
          />
        </label>
        <button type="submit" className="btn btn--primary" disabled={busy || !selected}>
          {busy ? t(locale, "cv.uploading") : t(locale, "cv.upload")}
        </button>
      </form>
      <p className="cv__note">{t(locale, "cv.reviewNote")}</p>

      {files.length === 0 ? (
        <p className="panel__status">{t(locale, "cv.list.empty")}</p>
      ) : (
        <ul className="cv__list">
          {files.map((f) => (
            <li key={f.id} className="cv__item">
              <span>
                {f.filename} <span className="cv__note">({formatSize(f.size_bytes)})</span>
              </span>
              <button type="button" className="btn" disabled={busy} onClick={() => onParse(f.id)}>
                {t(locale, "cv.parse")}
              </button>
            </li>
          ))}
        </ul>
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
