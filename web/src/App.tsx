import { lazy, Suspense, useEffect, useState } from "react";
import { AppShell, Container, Group, SegmentedControl, Stack, Text, Title, UnstyledButton } from "@mantine/core";
import { useMediaQuery } from "@mantine/hooks";
import { IconBriefcase, IconFileCv, IconListCheck, IconUser } from "@tabler/icons-react";
import { t, locales, DEFAULT_LOCALE, type Locale, type TranslationKey } from "./i18n";
import { JobFeed } from "./components/JobFeed";
import { AuthPanel } from "./components/AuthPanel";
import { AccountPanel } from "./components/AccountPanel";
import { useSession } from "./auth/session";
import { ThemeToggle } from "./theme";

// Signed-in screens load on demand so a visitor browsing jobs downloads less.
const SignedInViews = lazy(() => import("./SignedInViews"));

const VIEWS = [
  { id: "feed", label: "nav.feed", icon: IconBriefcase, signedIn: false },
  { id: "profile", label: "nav.profile", icon: IconFileCv, signedIn: true },
  { id: "applications", label: "nav.applications", icon: IconListCheck, signedIn: true },
  { id: "account", label: "nav.account", icon: IconUser, signedIn: false },
] as const satisfies ReadonlyArray<{ id: string; label: TranslationKey; icon: unknown; signedIn: boolean }>;

type ViewId = (typeof VIEWS)[number]["id"];

function useHashView(allowed: readonly ViewId[]): ViewId {
  const read = () => window.location.hash.slice(1);
  const [hash, setHash] = useState(read);
  useEffect(() => {
    const onChange = () => setHash(read());
    window.addEventListener("hashchange", onChange);
    return () => window.removeEventListener("hashchange", onChange);
  }, []);
  return allowed.find((id) => id === hash) ?? "feed";
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
        <Icon size={stacked ? 22 : 18} aria-hidden />
        <span>{t(locale, label)}</span>
      </UnstyledButton>
    ));

  return (
    <AppShell header={{ height: 60 }} footer={{ height: 64, collapsed: !mobile }} padding="md">
      <AppShell.Header>
        <Container size="xl" h="100%">
          <Group h="100%" justify="space-between" wrap="nowrap" gap="sm">
            <Title order={1} size="h4">
              {t(locale, "app.title")}
            </Title>
            {!mobile && (
              <Group component="nav" aria-label={t(locale, "nav.label")} gap={4} wrap="nowrap">
                {nav(false)}
              </Group>
            )}
            <Group gap="xs" wrap="nowrap">
              <SegmentedControl
                aria-label={t(locale, "app.language")}
                value={locale}
                onChange={(v) => setLocale(v as Locale)}
                data={locales.map((l) => ({ value: l, label: l.slice(0, 2).toUpperCase() }))}
                size="sm"
                styles={{ label: { minHeight: 44, minWidth: 44, display: "grid", placeItems: "center" } }}
              />
              <ThemeToggle locale={locale} />
            </Group>
          </Group>
        </Container>
      </AppShell.Header>

      <AppShell.Main>
        <Container size="xl" px={0}>
          <section hidden={view !== "feed"}>
            <Stack gap="lg">
              <Text c="dimmed" size="sm" maw={640}>
                {t(locale, "app.tagline")}
              </Text>
              <JobFeed locale={locale} canFill={canFill} fillReason={fillReason} signedIn={!!user} />
            </Stack>
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
            <Stack id="account" gap="md" maw={560}>
              <Title order={2} size="h3">
                {t(locale, "auth.heading")}
              </Title>
              <AuthPanel locale={locale} />
              {user && <AccountPanel locale={locale} />}
            </Stack>
          </section>
        </Container>
      </AppShell.Main>

      <AppShell.Footer>
        <Group component="nav" aria-label={t(locale, "nav.label")} h="100%" gap={0} grow wrap="nowrap">
          {nav(true)}
        </Group>
      </AppShell.Footer>
    </AppShell>
  );
}
