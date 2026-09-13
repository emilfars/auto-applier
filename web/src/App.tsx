import { useState } from "react";
import {
  Alert,
  AppShell,
  Burger,
  Container,
  Divider,
  Group,
  NavLink,
  Paper,
  Select,
  Stack,
  Text,
  Title,
} from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import {
  IconBriefcase,
  IconFileCv,
  IconShieldCheck,
  IconTools,
  IconUser,
  IconUsers,
} from "@tabler/icons-react";
import { t, locales, DEFAULT_LOCALE, type Locale } from "./i18n";
import { JobFeed } from "./components/JobFeed";
import { AuthPanel } from "./components/AuthPanel";
import { AccountPanel } from "./components/AccountPanel";
import { ProfilePanel } from "./components/ProfilePanel";
import { CvPanel } from "./components/CvPanel";
import { useSession } from "./auth/session";
import { ThemeToggle } from "./theme";
import { EnhancementsPanel } from "./components/EnhancementsPanel";

export default function App() {
  const [locale, setLocale] = useState<Locale>(DEFAULT_LOCALE);
  const { user } = useSession();
  const [profileRefresh, setProfileRefresh] = useState(0);
  const [profileConfirmed, setProfileConfirmed] = useState(false);
  const [opened, { toggle, close }] = useDisclosure();

  const canFill = !!user && profileConfirmed;
  const fillReason: "needLogin" | "needProfile" | undefined = !user
    ? "needLogin"
    : !profileConfirmed
      ? "needProfile"
      : undefined;

  return (
    <AppShell
      header={{ height: 64 }}
      navbar={{ width: 260, breakpoint: "sm", collapsed: { mobile: !opened } }}
      padding="md"
    >
      <AppShell.Header>
        <Group h="100%" px="md" justify="space-between" wrap="nowrap">
          <Group gap="sm">
            <Burger opened={opened} onClick={toggle} hiddenFrom="sm" size="sm" />
            <Title order={3} fw={900} style={{ letterSpacing: -0.5 }}>
              {t(locale, "app.title")}
            </Title>
          </Group>
          <Group gap="xs" wrap="nowrap">
            <Select
              aria-label={t(locale, "app.language")}
              value={locale}
              onChange={(v) => setLocale((v as Locale) ?? DEFAULT_LOCALE)}
              data={locales.map((l) => ({ value: l, label: l }))}
              w={96}
              size="xs"
            />
            <ThemeToggle locale={locale} />
          </Group>
        </Group>
      </AppShell.Header>

      <AppShell.Navbar p="md">
        <Stack gap="xs">
          <NavLink
            href="#feed"
            label={t(locale, "nav.feed")}
            leftSection={<IconBriefcase size={16} />}
            onClick={close}
          />
          <NavLink
            href="#account"
            label={t(locale, "nav.account")}
            leftSection={<IconUser size={16} />}
            onClick={close}
          />
          {user && (
            <>
              <NavLink
                href="#cv"
                label={t(locale, "cv.heading")}
                leftSection={<IconFileCv size={16} />}
                onClick={close}
              />
              <NavLink
                href="#profile"
                label={t(locale, "nav.profile")}
                leftSection={<IconUsers size={16} />}
                onClick={close}
              />
              <NavLink
                href="#enhancements"
                label={t(locale, "nav.tools")}
                leftSection={<IconTools size={16} />}
                onClick={close}
              />
            </>
          )}
        </Stack>
      </AppShell.Navbar>

      <AppShell.Main>
        <Container size="xl" px={0}>
          <Stack gap="lg">
            <Stack gap="xs">
              <Text c="dimmed" size="sm" maw={600}>
                {t(locale, "app.tagline")}
              </Text>
              <Alert
                color="teal"
                variant="light"
                icon={<IconShieldCheck size={16} />}
                title={t(locale, "app.safety")}
                p="sm"
              >
                {/* visual safety reassurance already in title */}
              </Alert>
            </Stack>

            <Paper id="account" withBorder shadow="sm" p="lg" radius="lg">
              <Title order={4} mb="md">
                {t(locale, "auth.heading")}
              </Title>
              <AuthPanel locale={locale} />
              {user && (
                <>
                  <Divider my="md" />
                  <AccountPanel locale={locale} />
                </>
              )}
            </Paper>

            {user && (
              <>
                <Paper id="cv" withBorder shadow="sm" p="lg" radius="lg">
                  <Title order={4} mb="md">
                    {t(locale, "cv.heading")}
                  </Title>
                  <CvPanel locale={locale} onParsed={() => setProfileRefresh((n) => n + 1)} />
                </Paper>

                <Paper id="profile" withBorder shadow="sm" p="lg" radius="lg">
                  <Title order={4} mb="md">
                    {t(locale, "profile.heading")}
                  </Title>
                  <ProfilePanel
                    key={profileRefresh}
                    locale={locale}
                    onConfirmedChange={setProfileConfirmed}
                  />
                </Paper>

                <Paper id="enhancements" withBorder shadow="sm" p="lg" radius="lg">
                  <Title order={4} mb="md">
                    {t(locale, "m5.heading")}
                  </Title>
                  <EnhancementsPanel locale={locale} />
                </Paper>
              </>
            )}

            <JobFeed locale={locale} canFill={canFill} fillReason={fillReason} signedIn={!!user} />
          </Stack>
        </Container>
      </AppShell.Main>
    </AppShell>
  );
}
