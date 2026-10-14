import { useEffect, useState } from "react";
import {
  Alert,
  Box,
  Button,
  Card,
  Divider,
  FileInput,
  Group,
  Skeleton,
  Stack,
  Text,
  ThemeIcon,
  Title,
} from "@mantine/core";
import { IconAlertCircle, IconCircleCheck, IconFileText, IconUpload } from "@tabler/icons-react";
import { t, type Locale } from "../i18n";
import { errorMessage } from "../i18n/errors";
import { StateMarker } from "./StateMarker";
import { Illustration } from "./Illustration";
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
    <Card padding={0}>
      <Stack gap="md" p="lg">
        <div>
          <Title order={2}>{t(locale, "cv.heading")}</Title>
          <Text size="sm" c="dimmed" mt={4}>
            {t(locale, "cv.reviewNote")}
          </Text>
        </div>
        <form onSubmit={onUpload}>
          <Group align="end" gap="sm" wrap="wrap">
            <FileInput
              label={t(locale, "cv.selectFile")}
              placeholder={t(locale, "cv.filePlaceholder")}
              leftSection={<IconFileText size={18} aria-hidden />}
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

      <Divider />

      <Stack gap={0} p="lg" pt="md">
        <Text size="sm" fw={600} c="dimmed" mb="sm">
          {t(locale, "cv.list.heading")}
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
            <Skeleton height={56} radius="md" />
          </Stack>
        ) : files.length === 0 ? (
          <Group gap="md" wrap="nowrap">
            <Box visibleFrom="xs" style={{ flex: "none" }}>
              <Illustration name="documents" width={120} />
            </Box>
            <Text size="sm" c="dimmed">
              {t(locale, "cv.list.empty")}
            </Text>
          </Group>
        ) : (
          <Stack gap="xs" className="stagger">
            {files.map((f, i) => (
              <Group
                key={f.id}
                justify="space-between"
                wrap="wrap"
                gap="sm"
                p="sm"
                data-testid="cv-item"
                className="job-card"
                style={
                  {
                    "--i": i,
                    border: "1px solid var(--mantine-color-default-border)",
                    borderRadius: "var(--mantine-radius-md)",
                  } as React.CSSProperties
                }
              >
                <Group gap="sm" wrap="nowrap" style={{ minWidth: 0, flex: "1 1 220px" }}>
                  <ThemeIcon size={40} radius="md" variant="light" color="blue" aria-hidden>
                    <IconFileText size={20} />
                  </ThemeIcon>
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
                </Group>
                <Group gap="xs">
                  {!f.is_primary && (
                    <Button variant="subtle" color="gray" loading={busy} onClick={() => onPrimary(f.id)}>
                      {t(locale, "cv.setPrimary")}
                    </Button>
                  )}
                  <Button variant="light" loading={busy} onClick={() => onParse(f.id)}>
                    {t(locale, "cv.parse")}
                  </Button>
                </Group>
              </Group>
            ))}
          </Stack>
        )}
      </Stack>
    </Card>
  );
}
