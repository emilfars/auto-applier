import { useState } from "react";
import {
  Alert,
  Anchor,
  Avatar,
  Button,
  Card,
  Checkbox,
  Divider,
  Grid,
  Group,
  Paper,
  PasswordInput,
  Skeleton,
  Stack,
  Text,
  TextInput,
  ThemeIcon,
  Title,
  VisuallyHidden,
} from "@mantine/core";
import { useForm } from "@mantine/form";
import { IconAlertCircle, IconCheck, IconCircleCheck, IconLogout } from "@tabler/icons-react";
import { Illustration } from "./Illustration";
import { t, type Locale, type TranslationKey } from "../i18n";
import { errorMessage } from "../i18n/errors";
import { useSession } from "../auth/session";
import { confirmReset, login, register, requestReset, verify, type RegisterResult } from "../api/auth";
import { ApiError } from "../api/http";

type StatusKeys = Partial<Record<number, TranslationKey>>;

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
      email: (v) => (/^\S+@\S+$/.test(v) ? null : t(locale, "validation.email")),
      password: (v) => (v.length >= 8 ? null : t(locale, "validation.passwordMin")),
      consent: (v) => (mode === "signup" && !v ? t(locale, "validation.consent") : null),
    },
  });

  if (loading) {
    return (
      <Card>
        <Stack gap="sm" aria-live="polite">
          <Skeleton height={40} radius="md" />
          <Skeleton height={40} radius="md" />
          <Skeleton height={40} radius="md" />
          <VisuallyHidden>{t(locale, "common.loading")}</VisuallyHidden>
        </Stack>
      </Card>
    );
  }

  if (user) {
    return (
      <Card data-testid="auth-signedin">
        <Group justify="space-between" wrap="wrap" gap="md">
          <Group gap="md" wrap="nowrap" style={{ minWidth: 0 }}>
            <Avatar size={44} radius="xl" color="blue" variant="light" aria-hidden>
              {user.email.slice(0, 1).toUpperCase()}
            </Avatar>
            <div style={{ minWidth: 0 }}>
              <Text size="sm" c="dimmed">
                {t(locale, "auth.signedInAs")}
              </Text>
              <Text fw={600} truncate>
                {user.email}
              </Text>
            </div>
          </Group>
          <Button
            variant="default"
            leftSection={<IconLogout size={16} aria-hidden />}
            onClick={() => {
              setError(null);
              void signOut().catch((err: unknown) => setError(errorMessage(locale, err)));
            }}
            data-testid="auth-signout"
          >
            {t(locale, "auth.signout")}
          </Button>
        </Group>
        {error && (
          <Alert color="red" variant="light" role="alert" icon={<IconAlertCircle aria-hidden />} mt="md">
            {error}
          </Alert>
        )}
      </Card>
    );
  }

  async function run(fn: () => Promise<void>, byStatus: StatusKeys = {}) {
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await fn();
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) setShowVerify(true);
      setError(errorMessage(locale, err, byStatus));
    } finally {
      setBusy(false);
    }
  }

  const onSubmit = form.onSubmit((values) => {
    if (mode === "signup") {
      void run(
        async () => {
          const res: RegisterResult = await register(values.email, values.password, values.consent);
          setShowVerify(true);
          if (res.verification_token) setVerifyToken(res.verification_token);
          setNotice(t(locale, "auth.needVerify"));
        },
        { 409: "auth.error.emailTaken" },
      );
    } else {
      void run(
        async () => {
          await login(values.email, values.password);
          await refresh();
        },
        { 401: "auth.error.credentials", 403: "auth.error.unverified" },
      );
    }
  });

  const onVerify = (e: React.FormEvent) => {
    e.preventDefault();
    void run(
      async () => {
        await verify(verifyToken);
        setShowVerify(false);
        setNotice(t(locale, "auth.verify.done"));
        setMode("signin");
      },
      { 400: "auth.error.verifyToken" },
    );
  };

  const onRequestReset = () => {
    if (form.validateField("email").hasError) return;
    void run(async () => {
      const res = await requestReset(form.values.email);
      if (res.reset_token) setResetToken(res.reset_token);
      setNotice(t(locale, "auth.reset.sent"));
    });
  };

  const onConfirmReset = (e: React.FormEvent) => {
    e.preventDefault();
    void run(
      async () => {
        await confirmReset(resetToken, newPassword);
        setShowReset(false);
        setNotice(t(locale, "auth.reset.done"));
        setMode("signin");
      },
      { 400: "auth.error.resetToken" },
    );
  };

  return (
    <Card padding={0} shadow="lg">
      <Grid gutter={0}>
        <Grid.Col span={{ base: 12, md: 6 }} className="page-hero" style={{ border: 0, borderRadius: 0 }}>
          <Stack gap="lg" p={{ base: "lg", sm: 40 }} h="100%" justify="center">
            <Illustration name="secure" width={200} />
            <Title order={2} style={{ fontSize: "1.6rem", letterSpacing: "-0.02em" }}>
              {t(locale, "auth.promo.title")}
            </Title>
            <Stack gap="sm">
              {(["auth.promo.1", "auth.promo.2", "auth.promo.3"] as const).map((key) => (
                <Group key={key} gap="sm" wrap="nowrap" align="flex-start">
                  <ThemeIcon size={24} radius="xl" color="teal" variant="light" aria-hidden style={{ flex: "none" }}>
                    <IconCheck size={14} stroke={3} />
                  </ThemeIcon>
                  <Text size="sm">{t(locale, key)}</Text>
                </Group>
              ))}
            </Stack>
          </Stack>
        </Grid.Col>
        <Grid.Col span={{ base: 12, md: 6 }}>
          <Stack gap="lg" p={{ base: "lg", sm: 40 }}>
            <Paper p={4} radius="md" bg="var(--mantine-color-default-hover)">
              <Group gap={4} grow wrap="nowrap">
                {(["signin", "signup"] as const).map((m) => (
                  <Button
                    key={m}
                    variant={mode === m ? "default" : "subtle"}
                    color={mode === m ? undefined : "gray"}
                    aria-pressed={mode === m}
                    data-testid={`auth-mode-${m}`}
                    onClick={() => setMode(m)}
                  >
                    {t(locale, m === "signin" ? "auth.tab.signin" : "auth.tab.signup")}
                  </Button>
                ))}
              </Group>
            </Paper>

            <form onSubmit={onSubmit} noValidate>
              <Stack gap="md">
                <TextInput
                  label={t(locale, "auth.email")}
                  type="email"
                  required
                  autoComplete="email"
                  data-testid="auth-email"
                  {...form.getInputProps("email")}
                />
                <PasswordInput
                  label={t(locale, "auth.password")}
                  required
                  autoComplete={mode === "signup" ? "new-password" : "current-password"}
                  data-testid="auth-password"
                  {...form.getInputProps("password")}
                />
                {mode === "signup" && (
                  <Checkbox
                    label={t(locale, "auth.consent")}
                    required
                    data-testid="auth-consent"
                    checked={form.values.consent}
                    onChange={(e) => form.setFieldValue("consent", e.currentTarget.checked)}
                    error={form.errors.consent}
                  />
                )}
                <Button type="submit" size="md" loading={busy} fullWidth data-testid="auth-submit">
                  {mode === "signup" ? t(locale, "auth.signup") : t(locale, "auth.signin")}
                </Button>
              </Stack>
            </form>

            <Anchor
              component="button"
              type="button"
              size="sm"
              onClick={() => setShowReset((v) => !v)}
              aria-expanded={showReset}
              style={{ alignSelf: "center", minHeight: 44 }}
            >
              {t(locale, "auth.reset.toggle")}
            </Anchor>

            {showVerify && (
              <>
                <Divider />
                <form onSubmit={onVerify}>
                  <Stack gap="sm">
                    <Title order={2} size="h4">
                      {t(locale, "auth.verify.heading")}
                    </Title>
                    <TextInput
                      label={t(locale, "auth.verify.token")}
                      required
                      value={verifyToken}
                      onChange={(e) => setVerifyToken(e.target.value)}
                      data-testid="auth-verify-token"
                    />
                    <Button type="submit" variant="default" loading={busy} data-testid="auth-verify-submit">
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
                  <Title order={2} size="h4">
                    {t(locale, "auth.reset.heading")}
                  </Title>
                  <Button variant="default" loading={busy} onClick={onRequestReset} style={{ alignSelf: "flex-start" }}>
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
        </Grid.Col>
      </Grid>
    </Card>
  );
}
