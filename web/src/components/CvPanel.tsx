import { useEffect, useState } from "react";
import { Alert, Button, FileInput, Group, Paper, Skeleton, Stack, Text } from "@mantine/core";
import { IconAlertCircle, IconCircleCheck, IconUpload } from "@tabler/icons-react";
import { t, type Locale } from "../i18n";
import { errorMessage } from "../i18n/errors";
import { StateMarker } from "./StateMarker";
import { useSession } from "../auth/session";
import { listCVs, parseCV, updateCVVersion, uploadCV, type CVFile } from "../api/cv";

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export function CvPanel({ locale, onParsed }: { locale: Locale; onParsed?: () => void }) {
  const { user } = useSession();
  const [files, setFiles] = useState<CVFile[] | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);
  const [selected, setSelected] = useState<File | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  useEffect(() => {
    if (!user) {
      setFiles(null);
      return;
    }
    setLoadError(null);
    const controller = new AbortController();
    listCVs(controller.signal)
      .then(setFiles)
      .catch((err: unknown) => {
        if (err instanceof DOMException && err.name === "AbortError") return;
        setLoadError(errorMessage(locale, err));
      });
    return () => controller.abort();
  }, [user, attempt]);

  if (!user) {
    return (
      <Text size="sm" c="dimmed">
        {t(locale, "profile.loginRequired")}
      </Text>
    );
  }

  async function run(fn: () => Promise<void>) {
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await fn();
    } catch (err) {
      setError(
        errorMessage(locale, err, {
          413: "cv.error.tooLarge",
          415: "cv.error.type",
          503: "cv.parseUnavailable",
        }),
      );
    } finally {
      setBusy(false);
    }
  }

  const onUpload = (e: React.FormEvent) => {
    e.preventDefault();
    if (!selected) return;
    void run(async () => {
      const rec = await uploadCV(selected);
      setFiles((prev) => [rec, ...(prev ?? [])]);
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

  const onPrimary = (id: string) =>
    void run(async () => {
      await updateCVVersion(id, { is_primary: true });
      setFiles((current) => (current ?? []).map((file) => ({ ...file, is_primary: file.id === id })));
      setNotice(t(locale, "cv.primarySet"));
    });

  return (
    <Stack gap="md">
      <form onSubmit={onUpload}>
        <Group align="end" gap="sm" wrap="wrap">
          <FileInput
            label={t(locale, "cv.selectFile")}
            placeholder={t(locale, "cv.filePlaceholder")}
            accept=".pdf,.docx,application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document"
            value={selected}
            onChange={setSelected}
            clearable
            style={{ flex: 1, minWidth: 220 }}
          />
          <Button
            type="submit"
            leftSection={<IconUpload size={16} aria-hidden />}
            loading={busy}
            disabled={!selected}
            data-testid="cv-upload-button"
          >
            {busy ? t(locale, "cv.uploading") : t(locale, "cv.upload")}
          </Button>
        </Group>
      </form>

      <Text size="xs" c="dimmed">
        {t(locale, "cv.reviewNote")}
      </Text>

      {loadError ? (
        <Alert color="red" variant="light" role="alert" icon={<IconAlertCircle aria-hidden />}>
          <Stack gap="xs" align="flex-start">
            <Text size="sm">
              {t(locale, "cv.loadError")} {loadError}
            </Text>
            <Button variant="default" onClick={() => setAttempt((n) => n + 1)}>
              {t(locale, "common.retry")}
            </Button>
          </Stack>
        </Alert>
      ) : files === null ? (
        <Stack gap="xs" aria-busy="true" aria-live="polite">
          <Text size="sm" c="dimmed">
            {t(locale, "cv.loading")}
          </Text>
          <Skeleton height={52} />
        </Stack>
      ) : files.length === 0 ? (
        <Text size="sm" c="dimmed">
          {t(locale, "cv.list.empty")}
        </Text>
      ) : (
        <Stack gap="xs">
          {files.map((f) => (
            <Paper key={f.id} withBorder p="sm" bg="var(--mantine-color-default)" data-testid="cv-item">
              <Group justify="space-between" wrap="wrap" gap="xs">
                <Stack gap={2} style={{ minWidth: 0 }}>
                  <Group gap={6} wrap="nowrap" style={{ minWidth: 0 }}>
                    <Text size="sm" fw={600} truncate>
                      {f.label || f.filename}
                    </Text>
                    <Text size="xs" c="dimmed" style={{ flex: "none" }}>
                      ({formatSize(f.size_bytes)})
                    </Text>
                  </Group>
                  {f.is_primary && <StateMarker state="success">{t(locale, "cv.primary")}</StateMarker>}
                </Stack>
                <Group gap="xs">
                  <Button variant="default" loading={busy} onClick={() => onParse(f.id)}>
                    {t(locale, "cv.parse")}
                  </Button>
                  {!f.is_primary && (
                    <Button variant="subtle" loading={busy} onClick={() => onPrimary(f.id)}>
                      {t(locale, "cv.setPrimary")}
                    </Button>
                  )}
                </Group>
              </Group>
            </Paper>
          ))}
        </Stack>
      )}

      {error && (
        <Alert color="red" variant="light" role="alert" icon={<IconAlertCircle aria-hidden />}>
          {error}
        </Alert>
      )}
      {notice && (
        <Alert color="teal" variant="light" role="status" icon={<IconCircleCheck aria-hidden />}>
          {notice}
        </Alert>
      )}
    </Stack>
  );
}
