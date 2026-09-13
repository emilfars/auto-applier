// Account data-rights UI (AUTH-5 / UU PDP No. 27/2022): export a complete copy
// of the user's data, or permanently erase the account. Erasure is gated behind
// typing the account email exactly, because it cannot be undone.

import { useState } from "react";
import { Alert, Button, Divider, Group, Modal, Stack, Text, TextInput, Title } from "@mantine/core";
import { t, type Locale } from "../i18n";
import { useSession } from "../auth/session";
import { deleteAccount, exportAccount } from "../api/account";
import { downloadJSON } from "../api/download";
import { ApiError } from "../api/http";

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
      setError(err instanceof ApiError ? err.message : t(locale, "account.error"));
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
    <Stack gap="md">
      <Divider />
      <Stack gap="xs">
        <Title order={5}>{t(locale, "account.dataRights")}</Title>
        <Text size="xs" c="dimmed">
          {t(locale, "account.dataRights.note")}
        </Text>
      </Stack>

      <Stack gap="xs">
        <Text fw={500}>{t(locale, "account.export.heading")}</Text>
        <Text size="sm" c="dimmed">
          {t(locale, "account.export.description")}
        </Text>
        <Button
          variant="default"
          loading={busy === "export"}
          disabled={busy !== null}
          onClick={onExport}
          data-testid="account-export"
          style={{ alignSelf: "flex-start" }}
        >
          {t(locale, "account.export.action")}
        </Button>
      </Stack>

      <Divider />

      <Stack gap="xs">
        <Text fw={500} c="red">
          {t(locale, "account.delete.heading")}
        </Text>
        <Text size="sm" c="dimmed">
          {t(locale, "account.delete.description")}
        </Text>
        <Button
          color="red"
          variant="light"
          disabled={busy !== null}
          onClick={() => setOpened(true)}
          data-testid="account-delete-open"
          style={{ alignSelf: "flex-start" }}
        >
          {t(locale, "account.delete.action")}
        </Button>
      </Stack>

      {error && (
        <Alert color="red" variant="light" role="alert" data-testid="account-status">
          {error}
        </Alert>
      )}
      {notice && (
        <Alert color="teal" variant="light" data-testid="account-notice">
          {notice}
        </Alert>
      )}

      <Modal opened={opened} onClose={closeModal} title={t(locale, "account.delete.confirmTitle")}>
        <Stack gap="sm">
          <Text size="sm">{t(locale, "account.delete.confirmBody")}</Text>
          <TextInput
            label={t(locale, "account.delete.emailLabel")}
            placeholder={email}
            value={typedEmail}
            onChange={(e) => setTypedEmail(e.currentTarget.value)}
            data-testid="account-delete-email"
          />
          <Group justify="flex-end">
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
