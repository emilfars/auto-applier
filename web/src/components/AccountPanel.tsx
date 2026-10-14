// Account data-rights UI (AUTH-5 / UU PDP No. 27/2022): export a complete copy
// of the user's data, or permanently erase the account. Erasure is gated behind
// typing the account email exactly, because it cannot be undone.

import { useState } from "react";
import { Alert, Button, Card, Group, Modal, Stack, Text, TextInput, ThemeIcon, Title } from "@mantine/core";
import { IconAlertCircle, IconCircleCheck, IconDownload, IconTrash } from "@tabler/icons-react";
import { t, type Locale } from "../i18n";
import { useSession } from "../auth/session";
import { deleteAccount, exportAccount } from "../api/account";
import { downloadJSON } from "../api/download";
import { errorMessage } from "../i18n/errors";

function exportFilename(): string {
  const stamp = new Date().toISOString().replace(/[:.]/g, "-");
  return `auto-applier-account-export-${stamp}.json`;
}

export function AccountPanel({ locale }: { locale: Locale }) {
  const { user, refresh } = useSession();
  const [busy, setBusy] = useState<"export" | "delete" | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [opened, setOpened] = useState(false);
  const [typedEmail, setTypedEmail] = useState("");

  const email = user?.email ?? "";
  const canDelete = email !== "" && typedEmail.trim() === email;

  async function run(kind: "export" | "delete", fn: () => Promise<void>) {
    setBusy(kind);
    setError(null);
    setNotice(null);
    try {
      await fn();
    } catch (err) {
      setError(errorMessage(locale, err));
    } finally {
      setBusy(null);
    }
  }

  const onExport = () =>
    void run("export", async () => {
      const data = await exportAccount();
      downloadJSON(exportFilename(), data);
      setNotice(t(locale, "account.export.done"));
    });

  const closeModal = () => {
    setOpened(false);
    setTypedEmail("");
  };

  const onConfirmDelete = () =>
    void run("delete", async () => {
      await deleteAccount();
      closeModal();
      await refresh();
    });

  return (
    <Stack gap="lg">
      <Stack gap={4} mt="md">
        <Title order={2}>{t(locale, "account.dataRights")}</Title>
        <Text size="sm" c="dimmed">
          {t(locale, "account.dataRights.note")}
        </Text>
      </Stack>

      <Card>
        <Group justify="space-between" align="center" gap="md" wrap="wrap">
          <Group gap="md" wrap="nowrap" align="flex-start" style={{ flex: "1 1 280px" }}>
            <ThemeIcon size={40} radius="md" variant="light" color="blue" aria-hidden>
              <IconDownload size={20} />
            </ThemeIcon>
            <div>
              <Text fw={600}>{t(locale, "account.export.heading")}</Text>
              <Text size="sm" c="dimmed">
                {t(locale, "account.export.description")}
              </Text>
            </div>
          </Group>
          <Button
            variant="default"
            loading={busy === "export"}
            disabled={busy !== null}
            onClick={onExport}
            data-testid="account-export"
          >
            {t(locale, "account.export.action")}
          </Button>
        </Group>
      </Card>

      <Card style={{ borderColor: "var(--brand-danger)" }}>
        <Group justify="space-between" align="center" gap="md" wrap="wrap">
          <Group gap="md" wrap="nowrap" align="flex-start" style={{ flex: "1 1 280px" }}>
            <ThemeIcon size={40} radius="md" variant="light" color="red" aria-hidden>
              <IconTrash size={20} />
            </ThemeIcon>
            <div>
              <Text fw={600} c="red">
                {t(locale, "account.delete.heading")}
              </Text>
              <Text size="sm" c="dimmed">
                {t(locale, "account.delete.description")}
              </Text>
            </div>
          </Group>
          <Button
            color="red"
            variant="light"
            disabled={busy !== null}
            onClick={() => setOpened(true)}
            data-testid="account-delete-open"
          >
            {t(locale, "account.delete.action")}
          </Button>
        </Group>
      </Card>

      {error && (
        <Alert
          color="red"
          variant="light"
          role="alert"
          icon={<IconAlertCircle aria-hidden />}
          data-testid="account-status"
        >
          {error}
        </Alert>
      )}
      {notice && (
        <Alert
          color="teal"
          variant="light"
          role="status"
          icon={<IconCircleCheck aria-hidden />}
          data-testid="account-notice"
        >
          {notice}
        </Alert>
      )}

      <Modal
        opened={opened}
        onClose={closeModal}
        title={<Text fw={650}>{t(locale, "account.delete.confirmTitle")}</Text>}
      >
        <Stack gap="sm">
          <Text size="sm">{t(locale, "account.delete.confirmBody")}</Text>
          <TextInput
            label={t(locale, "account.delete.emailLabel")}
            placeholder={email}
            value={typedEmail}
            onChange={(e) => setTypedEmail(e.currentTarget.value)}
            data-testid="account-delete-email"
          />
          <Group justify="flex-end" mt="sm">
            <Button variant="default" onClick={closeModal} data-testid="account-delete-cancel">
              {t(locale, "account.delete.cancel")}
            </Button>
            <Button
              color="red"
              disabled={!canDelete}
              loading={busy === "delete"}
              onClick={onConfirmDelete}
              data-testid="account-delete-confirm"
            >
              {t(locale, "account.delete.confirm")}
            </Button>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  );
}
