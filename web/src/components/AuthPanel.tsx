import { useState } from "react";
import {
  Alert,
  Button,
  Checkbox,
  Divider,
  Group,
  PasswordInput,
  Skeleton,
  Stack,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import { useForm } from "@mantine/form";
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

export function AuthPanel({ locale }: { locale: Locale }) {
  const { user, loading, refresh, signOut } = useSession();
  const [mode, setMode] = useState<Mode>("signin");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [showVerify, setShowVerify] = useState(false);
  const [showReset, setShowReset] = useState(false);
  const [verifyToken, setVerifyToken] = useState("");
  const [resetToken, setResetToken] = useState("");
  const [newPassword, setNewPassword] = useState("");

  const form = useForm({
    initialValues: { email: "", password: "", consent: false },
    validate: {
      email: (v) => (/^\S+@\S+$/.test(v) ? null : "Invalid email"),
      password: (v) => (v.length >= 8 ? null : "Min 8 characters"),
      consent: (v) => (mode === "signup" && !v ? "Consent required" : null),
    },
  });

  if (loading) {
    return (
      <Stack gap="sm" aria-live="polite">
        <Skeleton height={16} w={140} />
        <Skeleton height={40} radius="md" />
        <Skeleton height={40} radius="md" />
        <Text span style={{ position: "absolute", left: -9999 }}>
          {t(locale, "common.loading")}
        </Text>
      </Stack>
    );
  }

  if (user) {
    return (
      <Group justify="space-between" wrap="wrap" data-testid="auth-signedin">
        <Text size="sm" c="dimmed">
          {t(locale, "auth.signedInAs")} <Text span fw={700} c="bright">{user.email}</Text>
        </Text>
        <Button variant="default" onClick={() => void signOut()} data-testid="auth-signout">
          {t(locale, "auth.signout")}
        </Button>
      </Group>
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

  const onSubmit = form.onSubmit((values) => {
    if (mode === "signup") {
      void run(async () => {
        const res: RegisterResult = await register(values.email, values.password, values.consent);
        setShowVerify(true);
        if (res.verification_token) setVerifyToken(res.verification_token);
        setNotice(t(locale, "auth.needVerify"));
      });
    } else {
      void run(async () => {
        await login(values.email, values.password);
        await refresh();
      });
    }
  });

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
    if (!form.values.email.trim()) return;
    void run(async () => {
      const res = await requestReset(form.values.email);
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
    <Stack gap="md" maw={520}>
      <Group gap="xs">
        {(["signin", "signup"] as const).map((m) => (
          <Button
            key={m}
            variant={mode === m ? "filled" : "default"}
            size="xs"
            aria-pressed={mode === m}
            onClick={() => setMode(m)}
          >
            {t(locale, m === "signin" ? "auth.tab.signin" : "auth.tab.signup")}
          </Button>
        ))}
      </Group>

      <form onSubmit={onSubmit} noValidate>
        <Stack gap="sm">
          <TextInput
            label={t(locale, "auth.email")}
            type="email"
            required
            autoComplete="email"
            {...form.getInputProps("email")}
          />
          <PasswordInput
            label={t(locale, "auth.password")}
            required
            autoComplete={mode === "signup" ? "new-password" : "current-password"}
            {...form.getInputProps("password")}
          />
          {mode === "signup" && (
            <Checkbox
              label={t(locale, "auth.consent")}
              required
              checked={form.values.consent}
              onChange={(e) => form.setFieldValue("consent", e.currentTarget.checked)}
              error={form.errors.consent}
            />
          )}
          <Button type="submit" loading={busy} fullWidth>
            {mode === "signup" ? t(locale, "auth.signup") : t(locale, "auth.signin")}
          </Button>
        </Stack>
      </form>

      <Button variant="subtle" size="xs" onClick={() => setShowReset((v) => !v)} px={0} justify="flex-start">
        {t(locale, "auth.reset.toggle")}
      </Button>

      {showVerify && (
        <>
          <Divider />
          <form onSubmit={onVerify}>
            <Stack gap="sm">
              <Title order={5}>{t(locale, "auth.verify.heading")}</Title>
              <TextInput
                label={t(locale, "auth.verify.token")}
                required
                value={verifyToken}
                onChange={(e) => setVerifyToken(e.target.value)}
              />
              <Button type="submit" variant="default" loading={busy}>
                {t(locale, "auth.verify.submit")}
              </Button>
            </Stack>
          </form>
        </>
      )}

      {showReset && (
        <>
          <Divider />
          <Stack gap="sm">
            <Title order={5}>{t(locale, "auth.reset.heading")}</Title>
            <Button variant="default" size="xs" loading={busy} onClick={onRequestReset} style={{ alignSelf: "flex-start" }}>
              {t(locale, "auth.reset.request")}
            </Button>
            <form onSubmit={onConfirmReset}>
              <Stack gap="sm">
                <TextInput
                  label={t(locale, "auth.reset.token")}
                  required
                  value={resetToken}
                  onChange={(e) => setResetToken(e.target.value)}
                />
                <PasswordInput
                  label={t(locale, "auth.reset.newpw")}
                  required
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                />
                <Button type="submit" variant="default" loading={busy}>
                  {t(locale, "auth.reset.submit")}
                </Button>
              </Stack>
            </form>
          </Stack>
        </>
      )}

      {error && (
        <Alert color="red" variant="light" role="alert">
          {error}
        </Alert>
      )}
      {notice && (
        <Alert color="teal" variant="light">
          {notice}
        </Alert>
      )}
    </Stack>
  );
}
