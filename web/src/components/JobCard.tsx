import { useState } from "react";
import { Badge, Button, Card, Group, Paper, Stack, Text, Title } from "@mantine/core";
import { IconExternalLink } from "@tabler/icons-react";
import { t, type Locale, type TranslationKey } from "../i18n";
import { errorMessage } from "../i18n/errors";
import { hasStatedSalary, type JobCard as Job } from "../api/feed";
import {
  openFillDeps,
  requestOpenAndFill,
  type OpenFillResult,
  type OpenFillStatus,
} from "../api/openfill";
import { recordApplication, setJobState } from "../api/m5";
import { EMPLOYMENT_TYPES } from "../api/profile";
import { StateMarker } from "./StateMarker";

const NOTICE_KEY: Record<OpenFillStatus, TranslationKey> = {
  armed: "feed.openFill.armed",
  noExtension: "feed.openFill.noExtension",
  needLogin: "feed.openFill.needLogin",
  needProfile: "feed.openFill.needProfile",
  error: "feed.openFill.error",
};

/** Localized employment type, or the source's own wording when it is not one of ours. */
export function employmentLabel(locale: Locale, value: string): string {
  return (EMPLOYMENT_TYPES as readonly string[]).includes(value)
    ? t(locale, `employment.${value}` as TranslationKey)
    : value.replace(/_/g, " ");
}

export function JobCard({
  job,
  locale,
  canFill = false,
  fillReason,
  signedIn = false,
  onDismiss,
  runOpenFill = (url) => requestOpenAndFill(url, openFillDeps),
}: {
  job: Job;
  locale: Locale;
  canFill?: boolean;
  fillReason?: "needLogin" | "needProfile";
  signedIn?: boolean;
  onDismiss?: () => void;
  runOpenFill?: (url: string) => Promise<OpenFillResult>;
}) {
  const stated = hasStatedSalary(job.salary);
  const estimated = !stated && !!job.salary.estimated;
  const tags = job.requirement_tags?.length ? job.requirement_tags : job.requirements;
  const [notice, setNotice] = useState<OpenFillStatus | null>(null);
  const [busy, setBusy] = useState(false);
  const [applied, setApplied] = useState(job.already_applied ?? false);
  const [actionError, setActionError] = useState<string | null>(null);

  async function onOpenFill() {
    if (!canFill) {
      setNotice(fillReason ?? "needLogin");
      return;
    }
    setBusy(true);
    try {
      const result = await runOpenFill(job.source_url);
      setNotice(result.status);
      if (result.status === "armed" && signedIn && job.dedup_key && !applied) {
        void recordApplication(job.dedup_key).catch(() => undefined);
      }
    } catch {
      setNotice("error");
    } finally {
      setBusy(false);
    }
  }

  async function changeState(action: "dismiss" | "applied") {
    if (!job.dedup_key) return;
    setBusy(true);
    setActionError(null);
    try {
      await setJobState(job.dedup_key, action);
      if (action === "applied") setApplied(true);
      if (action === "dismiss") onDismiss?.();
    } catch (err) {
      setActionError(errorMessage(locale, err));
    } finally {
      setBusy(false);
    }
  }

  const meta = [
    job.location,
    job.employment_type && employmentLabel(locale, job.employment_type),
    job.seniority,
    job.years_experience != null && `${job.years_experience} ${t(locale, "feed.experience")}`,
  ].filter(Boolean);

  return (
    <Card withBorder padding="md" component="article" style={{ display: "flex", flexDirection: "column", height: "100%" }}>
      <Stack gap={6}>
        <Title order={3} size="h5" lineClamp={2}>
          {job.title}
        </Title>
        <Text size="sm">
          {job.company}{" "}
          <Text span c="dimmed">
            {t(locale, "feed.source", { source: job.source })}
          </Text>
        </Text>

        {stated || estimated ? (
          <div>
            <Text fw={700} style={{ overflowWrap: "anywhere" }}>
              {job.salary.label}
            </Text>
            <StateMarker state={stated ? "success" : "warning"}>
              {t(locale, stated ? "feed.salary.stated" : "feed.salary.estimated")}
            </StateMarker>
          </div>
        ) : (
          <Text c="dimmed">{t(locale, "feed.salary.undisclosed")}</Text>
        )}

        <Text size="sm" c="dimmed">
          {meta.join(" · ")}
        </Text>
        {job.remote && <StateMarker state="neutral">{t(locale, "feed.remote")}</StateMarker>}

        {job.match_score != null && (
          <Text size="sm" fw={600}>
            {t(locale, "feed.matchScore", { score: job.match_score })}
          </Text>
        )}

        {tags.length > 0 && (
          <Group gap={6} wrap="wrap" aria-label={t(locale, "feed.requirements")}>
            {tags.slice(0, 6).map((req) => (
              <Badge key={req} variant="default" radius="sm" tt="none" fw={500}>
                {req}
              </Badge>
            ))}
          </Group>
        )}
      </Stack>

      <Stack gap="xs" mt="auto" pt="md">
        <Group gap="xs" wrap="wrap">
          <Button loading={busy} onClick={onOpenFill} data-testid="job-fill-button">
            {t(locale, "feed.openFill")}
          </Button>
          <Button
            component="a"
            href={job.source_url}
            target="_blank"
            rel="noreferrer noopener"
            variant="default"
            rightSection={<IconExternalLink size={16} aria-hidden />}
          >
            {t(locale, "feed.viewApply")}
          </Button>
          {signedIn && (
            <>
              <Button variant="default" loading={busy} disabled={applied} onClick={() => void changeState("applied")}>
                {applied ? t(locale, "feed.applied") : t(locale, "feed.markApplied")}
              </Button>
              <Button variant="subtle" loading={busy} onClick={() => void changeState("dismiss")}>
                {t(locale, "feed.dismiss")}
              </Button>
            </>
          )}
        </Group>
        {actionError && (
          <Paper role="alert" p="xs" bg="var(--mantine-color-red-light)">
            <StateMarker state="danger">{actionError}</StateMarker>
          </Paper>
        )}
        {notice && (
          <Paper
            role="status"
            aria-live="polite"
            p="xs"
            data-testid="job-fill-notice"
            data-status={notice}
            bg={`var(--mantine-color-${notice === "armed" ? "teal" : notice === "error" ? "red" : "yellow"}-light)`}
          >
            <StateMarker state={notice === "armed" ? "success" : notice === "error" ? "danger" : "warning"}>
              {t(locale, NOTICE_KEY[notice])}
            </StateMarker>
          </Paper>
        )}
      </Stack>
    </Card>
  );
}
