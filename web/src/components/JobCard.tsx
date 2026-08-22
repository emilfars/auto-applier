import { useState } from "react";
import { Alert, Badge, Button, Card, Group, Stack, Text } from "@mantine/core";
import { IconExternalLink, IconWand, IconTrash, IconCheck } from "@tabler/icons-react";
import { t, type Locale, type TranslationKey } from "../i18n";
import { hasStatedSalary, type JobCard as Job } from "../api/feed";
import {
  openFillDeps,
  requestOpenAndFill,
  type OpenFillResult,
  type OpenFillStatus,
} from "../api/openfill";
import { recordApplication, setJobState } from "../api/m5";

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
  const tags = job.requirement_tags?.length ? job.requirement_tags : job.requirements;
  const [notice, setNotice] = useState<OpenFillStatus | null>(null);
  const [busy, setBusy] = useState(false);
  const [applied, setApplied] = useState(job.already_applied ?? false);

  const noticeKey: Record<OpenFillStatus, TranslationKey> = {
    armed: "feed.openFill.armed",
    noExtension: "feed.openFill.noExtension",
    needLogin: "feed.openFill.needLogin",
    needProfile: "feed.openFill.needProfile",
    error: "feed.error",
  };

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
    try {
      await setJobState(job.dedup_key, action);
      if (action === "applied") setApplied(true);
      if (action === "dismiss") onDismiss?.();
    } catch {
      setNotice("error");
    } finally {
      setBusy(false);
    }
  }

  const noticeColor = notice === "armed" ? "teal" : notice === "error" ? "red" : "yellow";

  return (
    <Card withBorder shadow="sm" radius="lg" padding="lg" style={{ display: "flex", flexDirection: "column", height: "100%" }}>
      <Group justify="space-between" align="flex-start" mb="xs" wrap="nowrap">
        <Text fw={700} size="lg" lineClamp={2} style={{ flex: 1, lineHeight: "22px" }}>
          {job.title}
        </Text>
        <Badge variant="light" size="sm" tt="uppercase">
          {job.source}
        </Badge>
      </Group>

      <Text c="blue" fw={600} size="sm" truncate>
        {job.company}
      </Text>

      <Group gap="xs" mt={4} wrap="wrap">
        <Text size="sm" c="dimmed">
          {job.location}
        </Text>
        {job.remote && (
          <Badge color="blue" variant="light" size="xs">
            {t(locale, "feed.remote")}
          </Badge>
        )}
        {job.employment_type && <Badge variant="default" size="xs">{job.employment_type}</Badge>}
        {job.seniority && <Badge variant="default" size="xs">{job.seniority}</Badge>}
        {job.years_experience != null && (
          <Badge variant="default" size="xs">
            {job.years_experience} {t(locale, "feed.experience")}
          </Badge>
        )}
      </Group>

      <Text
        mt="sm"
        fw={stated ? 700 : 500}
        c={stated ? "teal" : job.salary.estimated ? "yellow.7" : "dimmed"}
        fs={!stated && !job.salary.estimated ? "italic" : undefined}
        truncate
      >
        {stated || job.salary.estimated ? job.salary.label : t(locale, "feed.salary.undisclosed")}
      </Text>

      {job.match_score != null && (
        <Text size="sm" fw={600} c="blue">
          {t(locale, "feed.matchScore", { score: job.match_score })}
        </Text>
      )}

      <Group gap={6} mt="sm" wrap="wrap" style={{ minHeight: 56, alignContent: "flex-start" }} aria-label={t(locale, "feed.requirements")}>
        {tags.slice(0, 6).map((req) => (
          <Badge key={req} variant="default" size="sm" radius="sm">
            {req}
          </Badge>
        ))}
      </Group>

      <Stack gap="xs" mt="auto" pt="sm">
        <Group gap="xs" wrap="wrap">
          <Button size="xs" leftSection={<IconWand size={14} />} loading={busy} onClick={onOpenFill}>
            {t(locale, "feed.openFill")}
          </Button>
          <Button
            component="a"
            href={job.source_url}
            target="_blank"
            rel="noreferrer noopener"
            variant="outline"
            size="xs"
            rightSection={<IconExternalLink size={14} />}
          >
            {t(locale, "feed.viewApply")}
          </Button>
          {signedIn && (
            <>
              <Button variant="subtle" color="red" size="xs" leftSection={<IconTrash size={14} />} loading={busy} onClick={() => void changeState("dismiss")}>
                {t(locale, "feed.dismiss")}
              </Button>
              <Button
                variant={applied ? "light" : "default"}
                color={applied ? "teal" : undefined}
                size="xs"
                leftSection={<IconCheck size={14} />}
                loading={busy}
                disabled={applied}
                onClick={() => void changeState("applied")}
              >
                {applied ? t(locale, "feed.applied") : t(locale, "feed.markApplied")}
              </Button>
            </>
          )}
        </Group>
        <Text size="xs" c="dimmed" lh={1.4}>
          {t(locale, "feed.openFill.note")}
        </Text>
        {notice && (
          <Alert color={noticeColor} variant="light" py={8} role="status">
            {t(locale, noticeKey[notice])}
          </Alert>
        )}
      </Stack>
    </Card>
  );
}
