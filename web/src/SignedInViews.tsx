import { Stack, Title } from "@mantine/core";
import { t, type Locale } from "./i18n";
import { CvPanel } from "./components/CvPanel";
import { ProfilePanel } from "./components/ProfilePanel";
import { EnhancementsPanel } from "./components/EnhancementsPanel";

/** The CV-and-profile and applications views. Both stay mounted; `view` only toggles visibility. */
export default function SignedInViews({
  locale,
  view,
  profileRefresh,
  onParsed,
  onConfirmedChange,
}: {
  locale: Locale;
  view: string;
  profileRefresh: number;
  onParsed: () => void;
  onConfirmedChange: (confirmed: boolean) => void;
}) {
  return (
    <>
      <section hidden={view !== "profile"}>
        <Stack gap="xl" maw={860}>
          <Stack id="cv" gap="md">
            <Title order={2} size="h3">
              {t(locale, "cv.heading")}
            </Title>
            <CvPanel locale={locale} onParsed={onParsed} />
          </Stack>
          <Stack id="profile" gap="md">
            <Title order={2} size="h3">
              {t(locale, "profile.heading")}
            </Title>
            <ProfilePanel key={profileRefresh} locale={locale} onConfirmedChange={onConfirmedChange} />
          </Stack>
        </Stack>
      </section>

      <section hidden={view !== "applications"}>
        <Stack id="enhancements" gap="md">
          <Title order={2} size="h3">
            {t(locale, "m5.heading")}
          </Title>
          <EnhancementsPanel locale={locale} />
        </Stack>
      </section>
    </>
  );
}
