import { lazy, Suspense, useEffect, useState } from "react";
import {
  AppShell,
  Avatar,
  Box,
  Button,
  Container,
  Group,
  SegmentedControl,
  Stack,
  Text,
  Title,
  UnstyledButton,
} from "@mantine/core";
import { useMediaQuery } from "@mantine/hooks";
import { IconSend, IconBriefcase, IconFileCv, IconListCheck, IconLogin2, IconUserCircle } from "@tabler/icons-react";
import { t, locales, DEFAULT_LOCALE, type Locale, type TranslationKey } from "./i18n";
import { JobFeed } from "./components/JobFeed";
import { AuthPanel } from "./components/AuthPanel";
import { AccountPanel } from "./components/AccountPanel";
import { PageHeader } from "./components/PageHeader";
import { useSession } from "./auth/session";
import { ThemeToggle } from "./theme";

// Signed-in screens load on demand so a visitor browsing jobs downloads less.
const SignedInViews = lazy(() => import("./SignedInViews"));

const VIEWS = [
  { id: "feed", label: "nav.feed", icon: IconBriefcase, signedIn: false },
  { id: "profile", label: "nav.profile", icon: IconFileCv, signedIn: true },
  { id: "applications", label: "nav.applications", icon: IconListCheck, signedIn: true },
  { id: "account", label: "nav.account", icon: IconUserCircle, signedIn: false },
] as const satisfies ReadonlyArray<{ id: string; label: TranslationKey; icon: unknown; signedIn: boolean }>;

type ViewId = (typeof VIEWS)[number]["id"];

function useHashView(allowed: readonly ViewId[]): ViewId {
  const read = () => window.location.hash.slice(1);
  const [hash, setHash] = useState(read);
  useEffect(() => {
    const onChange = () => {
      setHash(read());
      document.documentElement.scrollTop = 0;
    };
    window.addEventListener("hashchange", onChange);
    return () => window.removeEventListener("hashchange", onChange);
  }, []);
  return allowed.find((id) => id === hash) ?? "feed";
}

function BrandMark({ locale }: { locale: Locale }) {
  return (
    <Group
      gap="sm"
      wrap="nowrap"
      renderRoot={(props) => <a href="#feed" {...props} />}
      style={{ textDecoration: "none", color: "inherit" }}
    >
      <span className="app-brand-mark" aria-hidden="true">
        <IconSend size={18} stroke={2} />
      </span>
      <Text fw={700} size="md" style={{ letterSpacing: "-0.01em" }}>
        {t(locale, "app.title")}
      </Text>
    </Group>
  );
}

export default function App() {
  const [locale, setLocale] = useState<Locale>(DEFAULT_LOCALE);
  const { user } = useSession();
  const [profileRefresh, setProfileRefresh] = useState(0);
  // null until the profile has loaded. Open & Fill is not refused on a guess:
  // the server's arm check decides while this is unknown.
  const [profileConfirmed, setProfileConfirmed] = useState<boolean | null>(null);
  const userId = user?.id;
  useEffect(() => setProfileConfirmed(null), [userId]);
  useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);
  const mobile = useMediaQuery("(max-width: 48em)", false, { getInitialValueInEffect: false });

  const views = VIEWS.filter((v) => user || !v.signedIn);
  const view = useHashView(views.map((v) => v.id));

  const canFill = !!user && profileConfirmed !== false;
  const fillReason: "needLogin" | "needProfile" | undefined = !user
    ? "needLogin"
    : profileConfirmed === false
      ? "needProfile"
      : undefined;

  const nav = (stacked: boolean) =>
    views.map(({ id, label, icon: Icon }) => (
      <UnstyledButton
        key={id}
        component="a"
        href={`#${id}`}
        aria-current={view === id ? "page" : undefined}
        className="app-nav-item"
        data-stacked={stacked || undefined}
      >
        <Icon size={stacked ? 22 : 20} stroke={1.75} aria-hidden />
        <span>{t(locale, label)}</span>
      </UnstyledButton>
    ));

  const preferences = (
    <Group gap="xs" wrap="nowrap">
      <SegmentedControl
        aria-label={t(locale, "app.language")}
        value={locale}
        onChange={(v) => setLocale(v as Locale)}
        data={locales.map((l) => ({ value: l, label: l.slice(0, 2).toUpperCase() }))}
        size="sm"
        radius="md"
        styles={{ label: { minHeight: 36, minWidth: 40, display: "grid", placeItems: "center" } }}
      />
      <ThemeToggle locale={locale} />
    </Group>
  );

  return (
    <AppShell
      header={{ height: 56, collapsed: !mobile }}
      navbar={{ width: 248, breakpoint: "48em", collapsed: { mobile: true } }}
      footer={{ height: 64, collapsed: !mobile }}
    >
      <AppShell.Header px="md">
        <Group h="100%" justify="space-between" wrap="nowrap">
          <BrandMark locale={locale} />
          {mobile && preferences}
        </Group>
      </AppShell.Header>

      <AppShell.Navbar p="md" className="app-sidebar">
        {!mobile && (
          <>
            <Box px={4} py={6} mb="lg">
              <BrandMark locale={locale} />
            </Box>
            <Stack component="nav" aria-label={t(locale, "nav.label")} gap={4} style={{ flex: 1 }}>
              {nav(false)}
            </Stack>
            <Stack gap="md" pt="md" style={{ borderTop: "1px solid var(--mantine-color-default-border)" }}>
              {user ? (
                <Group gap="sm" wrap="nowrap" px={4}>
                  <Avatar size={32} radius="xl" color="blue" variant="light">
                    {user.email.slice(0, 1).toUpperCase()}
                  </Avatar>
                  <Text size="sm" truncate style={{ minWidth: 0 }}>
                    {user.email}
                  </Text>
                </Group>
              ) : (
                <Button
                  component="a"
                  href="#account"
                  variant="light"
                  leftSection={<IconLogin2 size={18} aria-hidden />}
                  fullWidth
                >
                  {t(locale, "auth.tab.signin")}
                </Button>
              )}
              {preferences}
            </Stack>
          </>
        )}
      </AppShell.Navbar>

      <AppShell.Main>
        <Container size={1200} px={{ base: "md", sm: "xl" }} py={{ base: "lg", sm: "xl" }}>
          <section hidden={view !== "feed"}>
            <PageHeader
              title={t(locale, "feed.heading")}
              description={t(locale, "app.tagline")}
              illustration="jobs"
              overlap
            />
            <JobFeed locale={locale} canFill={canFill} fillReason={fillReason} signedIn={!!user} />
          </section>

          {user && (
            <Suspense fallback={null}>
              <SignedInViews
                locale={locale}
                view={view}
                profileRefresh={profileRefresh}
                onParsed={() => setProfileRefresh((n) => n + 1)}
                onConfirmedChange={setProfileConfirmed}
              />
            </Suspense>
          )}

          <section hidden={view !== "account"}>
            <Box id="account" maw={user ? 760 : 960} mx="auto">
              {user ? (
                <PageHeader
                  title={t(locale, "auth.heading")}
                  description={t(locale, "page.account.description")}
                  illustration="secure"
                />
              ) : (
                <Title order={1} mb="lg" style={{ letterSpacing: "-0.03em" }}>
                  {t(locale, "auth.heading")}
                </Title>
              )}
              <Stack gap="lg">
                <AuthPanel locale={locale} />
                {user && <AccountPanel locale={locale} />}
              </Stack>
            </Box>
          </section>
        </Container>
      </AppShell.Main>

      <AppShell.Footer>
        <Group component="nav" aria-label={t(locale, "nav.label")} h="100%" gap={0} wrap="nowrap">
          {mobile && nav(true)}
        </Group>
      </AppShell.Footer>
    </AppShell>
  );
}
