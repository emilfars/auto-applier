import { useState } from "react";
import { Badge, Box, Button, Card, Divider, Group, Paper, Stack, Text, Title } from "@mantine/core";
import { IconBriefcase, IconClock, IconExternalLink, IconMapPin, IconTarget } from "@tabler/icons-react";
import { t, type Locale, type TranslationKey } from "../i18n";
import { errorMessage } from "../i18n/errors";
import { hasStatedSalary, type JobCard as Job } from "../api/feed";
import { openFillDeps, requestOpenAndFill, type OpenFillResult, type OpenFillStatus } from "../api/openfill";
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

function initials(company: string): string {
  const words = company.trim().split(/\s+/).filter(Boolean);
  return (
    words
      .slice(0, 2)
      .map((w) => w[0])
      .join("")
      .toUpperCase() || "?"
  );
}

function tone(company: string): number {
  let hash = 0;
  for (const ch of company) hash = (hash * 31 + ch.charCodeAt(0)) >>> 0;
  return hash % 6;
}

const NEW_WINDOW_MS = 3 * 24 * 60 * 60 * 1000;

function Meta({ icon: Icon, children }: { icon: typeof IconMapPin; children: React.ReactNode }) {
  return (
    <Group gap={6} wrap="nowrap" style={{ minWidth: 0 }}>
      <Icon size={16} stroke={1.75} aria-hidden style={{ flex: "none", color: "var(--mantine-color-dimmed)" }} />
      <Text size="sm" c="dimmed" truncate>
        {children}
      </Text>
    </Group>
  );
}

export function JobCard({
  job,
  locale,
  canFill = false,
  fillReason,
  signedIn = false,
  onDismiss,
  index = 0,
  runOpenFill = (url) => requestOpenAndFill(url, openFillDeps),
}: {
  job: Job;
  locale: Locale;
  canFill?: boolean;
  fillReason?: "needLogin" | "needProfile";
  signedIn?: boolean;
  onDismiss?: () => void;
  /** Position in the list, used to stagger the entrance animation. */
  index?: number;
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

  const posted = job.posted_at ? Date.parse(job.posted_at) : NaN;
  const isNew = Date.now() - posted < NEW_WINDOW_MS;
  const noticeState = notice === "armed" ? "success" : notice === "error" ? "danger" : "warning";
  const noticeColor = notice === "armed" ? "teal" : notice === "error" ? "red" : "yellow";

  return (
    <Card component="article" className="job-card" padding={0} style={{ "--i": index } as React.CSSProperties}>
      <Group align="flex-start" wrap="nowrap" gap="md" p={{ base: "md", sm: "lg" }}>
        <Box className="company-avatar" data-tone={tone(job.company)} visibleFrom="xs" aria-hidden>
          {initials(job.company)}
        </Box>

        <Stack gap="sm" style={{ flex: 1, minWidth: 0 }}>
          <Group justify="space-between" align="flex-start" gap="sm" wrap="wrap">
            <Stack gap={2} style={{ flex: "1 1 260px", minWidth: 0 }}>
              <Group gap="xs" wrap="nowrap" align="flex-start">
                <Title order={3} lineClamp={2}>
                  {job.title}
                </Title>
                {isNew && (
                  <Badge
                    variant="light"
                    color="teal"
                    size="sm"
                    leftSection={<span className="live-dot" aria-hidden />}
                    style={{ flex: "none" }}
                    mt={2}
                  >
                    {t(locale, "feed.new")}
                  </Badge>
                )}
              </Group>
              <Group gap={6} wrap="wrap">
                <Text size="sm" fw={500}>
                  {job.company}
                </Text>
                <Text size="sm" c="dimmed">
                  {t(locale, "feed.source", { source: job.source })}
                </Text>
              </Group>
            </Stack>

            <Stack gap={4} align="flex-start" style={{ flex: "0 1 auto", minWidth: 0 }}>
              {stated || estimated ? (
                <>
                  <Text fw={700} style={{ overflowWrap: "anywhere" }}>
                    {job.salary.label}
                  </Text>
                  <StateMarker state={stated ? "success" : "warning"}>
                    {t(locale, stated ? "feed.salary.stated" : "feed.salary.estimated")}
                  </StateMarker>
                </>
              ) : (
                <Text size="sm" c="dimmed">
                  {t(locale, "feed.salary.undisclosed")}
                </Text>
              )}
            </Stack>
          </Group>

          <Group gap="lg" wrap="wrap" style={{ rowGap: 6 }}>
            {job.location && <Meta icon={IconMapPin}>{job.location}</Meta>}
            {job.employment_type && <Meta icon={IconBriefcase}>{employmentLabel(locale, job.employment_type)}</Meta>}
            {(job.seniority || job.years_experience != null) && (
              <Meta icon={IconClock}>
                {[
                  job.seniority,
                  job.years_experience != null && `${job.years_experience} ${t(locale, "feed.experience")}`,
                ]
                  .filter(Boolean)
                  .join(" · ")}
              </Meta>
            )}
          </Group>

          {(job.remote || job.match_score != null || tags.length > 0) && (
            <Group gap={6} wrap="wrap" aria-label={tags.length > 0 ? t(locale, "feed.requirements") : undefined}>
              {job.remote && (
                <Badge variant="light" color="blue">
                  {t(locale, "feed.remote")}
                </Badge>
              )}
              {job.match_score != null && (
                <Badge variant="light" color="teal" leftSection={<IconTarget size={12} aria-hidden />}>
                  {t(locale, "feed.matchScore", { score: job.match_score })}
                </Badge>
              )}
              {tags.slice(0, 6).map((req) => (
                <Badge key={req} variant="default">
                  {req}
                </Badge>
              ))}
            </Group>
          )}
        </Stack>
      </Group>

      <Divider />

      <Box px={{ base: "md", sm: "lg" }} py="sm">
        <Group justify="space-between" gap="xs" wrap="wrap">
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
          </Group>
          {signedIn && (
            <Group gap="xs" wrap="wrap">
              <Button variant="subtle" color="gray" loading={busy} onClick={() => void changeState("dismiss")}>
                {t(locale, "feed.dismiss")}
              </Button>
              <Button variant="light" loading={busy} disabled={applied} onClick={() => void changeState("applied")}>
                {applied ? t(locale, "feed.applied") : t(locale, "feed.markApplied")}
              </Button>
            </Group>
          )}
        </Group>

        {actionError && (
          <Paper role="alert" mt="sm" p="sm" radius="md" bg="var(--mantine-color-red-light)">
            <StateMarker state="danger">{actionError}</StateMarker>
          </Paper>
        )}
        {notice && (
          <Paper
            role="status"
            aria-live="polite"
            mt="sm"
            p="sm"
            radius="md"
            data-testid="job-fill-notice"
            data-status={notice}
            bg={`var(--mantine-color-${noticeColor}-light)`}
          >
            <StateMarker state={noticeState}>{t(locale, NOTICE_KEY[notice])}</StateMarker>
          </Paper>
        )}
      </Box>
    </Card>
  );
}
