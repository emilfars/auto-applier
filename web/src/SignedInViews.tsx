import { Stack } from "@mantine/core";
import { t, type Locale } from "./i18n";
import { CvPanel } from "./components/CvPanel";
import { ProfilePanel } from "./components/ProfilePanel";
import { EnhancementsPanel } from "./components/EnhancementsPanel";
import { PageHeader } from "./components/PageHeader";

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
        <PageHeader
          title={t(locale, "page.profile.title")}
          description={t(locale, "page.profile.description")}
          illustration="documents"
        />
        <Stack gap="xl">
          <div id="cv">
            <CvPanel locale={locale} onParsed={onParsed} />
          </div>
          <div id="profile">
            <ProfilePanel key={profileRefresh} locale={locale} onConfirmedChange={onConfirmedChange} />
          </div>
        </Stack>
      </section>

      <section hidden={view !== "applications"}>
        <PageHeader
          title={t(locale, "nav.applications")}
          description={t(locale, "page.applications.description")}
          illustration="tracker"
        />
        <div id="enhancements">
          <EnhancementsPanel locale={locale} />
        </div>
      </section>
    </>
  );
}
