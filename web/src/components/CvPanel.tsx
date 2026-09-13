import { useEffect, useState } from "react";
import { Alert, Badge, Button, FileInput, Group, Paper, Stack, Text } from "@mantine/core";
import { IconUpload, IconWand, IconStar } from "@tabler/icons-react";
import { t, type Locale } from "../i18n";
import { useSession } from "../auth/session";
import { listCVs, parseCV, updateCVVersion, uploadCV, type CVFile } from "../api/cv";
import { ApiError } from "../api/http";

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export function CvPanel({ locale, onParsed }: { locale: Locale; onParsed?: () => void }) {
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
  }, [user]);

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
      if (err instanceof ApiError && err.status === 503) setError(t(locale, "cv.parseUnavailable"));
      else setError(err instanceof ApiError ? err.message : t(locale, "auth.error"));
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

  const onPrimary = (id: string) =>
    void run(async () => {
      await updateCVVersion(id, { is_primary: true });
      setFiles((current) => current.map((file) => ({ ...file, is_primary: file.id === id })));
      setNotice(t(locale, "cv.primarySet"));
    });

  return (
    <Stack gap="md">
      <form onSubmit={onUpload}>
        <Group align="end" gap="sm" wrap="wrap">
          <FileInput
            label={t(locale, "cv.selectFile")}
            placeholder="PDF or DOCX"
            accept=".pdf,.docx,application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document"
            value={selected}
            onChange={setSelected}
            clearable
            style={{ flex: 1, minWidth: 220 }}
          />
          <Button
            type="submit"
            leftSection={<IconUpload size={16} />}
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

      {files.length === 0 ? (
        <Paper withBorder p="md" radius="md" bg="var(--mantine-color-default-hover)">
          <Text size="sm" c="dimmed" ta="center">
            {t(locale, "cv.list.empty")}
          </Text>
        </Paper>
      ) : (
        <Stack gap="xs">
          {files.map((f) => (
            <Paper key={f.id} withBorder p="sm" radius="md" data-testid="cv-item">
              <Group justify="space-between" wrap="wrap" gap="xs">
                <Group gap="xs" wrap="nowrap" style={{ minWidth: 0 }}>
                  <Text size="sm" fw={500} truncate>
                    {f.label || f.filename}
                  </Text>
                  <Text size="xs" c="dimmed">
                    ({formatSize(f.size_bytes)})
                  </Text>
                  {f.is_primary && (
                    <Badge color="teal" size="xs" leftSection={<IconStar size={10} />}>
                      {t(locale, "cv.primary")}
                    </Badge>
                  )}
                </Group>
                <Group gap="xs">
                  <Button
                    size="xs"
                    variant="default"
                    leftSection={<IconWand size={14} />}
                    loading={busy}
                    onClick={() => onParse(f.id)}
                  >
                    {t(locale, "cv.parse")}
                  </Button>
                  {!f.is_primary && (
                    <Button size="xs" variant="light" loading={busy} onClick={() => onPrimary(f.id)}>
                      {t(locale, "cv.setPrimary")}
                    </Button>
                  )}
                </Group>
              </Group>
            </Paper>
          ))}
        </Stack>
      )}

      {error && <Alert color="red" variant="light" role="alert">{error}</Alert>}
      {notice && <Alert color="teal" variant="light">{notice}</Alert>}
    </Stack>
  );
}
