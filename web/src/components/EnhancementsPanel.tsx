import { useEffect, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Grid,
  Group,
  Paper,
  NativeSelect,
  Progress,
  Skeleton,
  Stack,
  Text,
  TextInput,
  Textarea,
  Title,
} from "@mantine/core";
import { useSession } from "../auth/session";
import {
  confirmSubmitted,
  createSnippet,
  deleteSavedFilter,
  deleteSnippet,
  getCompleteness,
  listApplications,
  listSavedFilters,
  listSnippets,
  saveFilter,
  updateApplication,
  type Application,
  type Completeness,
  type SavedFilter,
  type Snippet,
} from "../api/m5";
import { IconAlertCircle } from "@tabler/icons-react";
import { t, type Locale, type TranslationKey } from "../i18n";
import { errorMessage } from "../i18n/errors";
import { StateMarker } from "./StateMarker";

const STATUSES: Application["status"][] = ["form_filled", "submitted", "viewed", "rejected", "interview"];

export function EnhancementsPanel({ locale }: { locale: Locale }) {
  const { user } = useSession();
  const [completeness, setCompleteness] = useState<Completeness | null>(null);
  const [filters, setFilters] = useState<SavedFilter[]>([]);
  const [applications, setApplications] = useState<Application[]>([]);
  const [snippets, setSnippets] = useState<Snippet[]>([]);
  const [filterName, setFilterName] = useState("");
  const [filterSearch, setFilterSearch] = useState("");
  const [snippetName, setSnippetName] = useState("");
  const [snippetBody, setSnippetBody] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [load, setLoad] = useState<"loading" | "ready" | "error">("loading");
  const [attempt, setAttempt] = useState(0);
  const [saving, setSaving] = useState<"filter" | "snippet" | null>(null);

  useEffect(() => {
    if (!user) return;
    let current = true;
    setLoad("loading");
    Promise.all([getCompleteness(), listSavedFilters(), listApplications(), listSnippets()])
      .then(([score, saved, tracked, reusable]) => {
        if (!current) return;
        setCompleteness(score);
        setFilters(saved);
        setApplications(tracked);
        setSnippets(reusable);
        setLoad("ready");
      })
      .catch((err: unknown) => {
        if (!current) return;
        setError(errorMessage(locale, err));
        setLoad("error");
      });
    return () => {
      current = false;
    };
  }, [user, attempt]);

  if (!user) return null;
  if (load === "loading") {
    return (
      <Stack gap="sm" aria-busy="true" aria-live="polite">
        <Text size="sm" c="dimmed">
          {t(locale, "m5.loading")}
        </Text>
        <Skeleton height={72} />
        <Skeleton height={160} />
      </Stack>
    );
  }
  if (load === "error") {
    return (
      <Alert color="red" variant="light" role="alert" icon={<IconAlertCircle aria-hidden />}>
        <Stack gap="xs" align="flex-start">
          <Text size="sm">
            {t(locale, "m5.loadError")} {error}
          </Text>
          <Button
            variant="default"
            onClick={() => {
              setError(null);
              setAttempt((n) => n + 1);
            }}
          >
            {t(locale, "common.retry")}
          </Button>
        </Stack>
      </Alert>
    );
  }

  async function createFilter(event: React.FormEvent) {
    event.preventDefault();
    if (!filterName.trim() || saving) return;
    setSaving("filter");
    setError(null);
    try {
      const filter = await saveFilter(filterName, { q: filterSearch });
      setFilters((current) => [...current, filter]);
      setFilterName("");
    } catch (err) {
      setError(errorMessage(locale, err));
    } finally {
      setSaving(null);
    }
  }

  async function createAnswer(event: React.FormEvent) {
    event.preventDefault();
    if (!snippetName.trim() || !snippetBody.trim() || saving) return;
    setSaving("snippet");
    setError(null);
    try {
      const snippet = await createSnippet(snippetName, snippetBody);
      setSnippets((current) => [...current, snippet]);
      setSnippetName("");
      setSnippetBody("");
    } catch (err) {
      setError(errorMessage(locale, err));
    } finally {
      setSaving(null);
    }
  }

  async function changeStatus(application: Application, status: Application["status"]) {
    try {
      const updated =
        status === "submitted"
          ? await confirmSubmitted(application.id)
          : await updateApplication(application.id, status);
      setApplications((current) => current.map((item) => (item.id === updated.id ? updated : item)));
    } catch (err) {
      setError(errorMessage(locale, err));
    }
  }

  return (
    <Stack gap="md">
      {completeness && (
        <Paper withBorder p="md" bg="var(--mantine-color-default)">
          <Group justify="space-between" mb={8}>
            <Title order={3} size="h5">{t(locale, "m5.completeness")}</Title>
            <StateMarker state={completeness.complete ? "success" : "warning"}>{completeness.score}%</StateMarker>
          </Group>
          <Progress value={completeness.score} color={completeness.complete ? "teal" : "blue"} />
          {completeness.missing.length > 0 && (
            <Text size="sm" c="dimmed" mt="xs">
              {t(locale, "m5.missing", {
                fields: completeness.missing.map((f) => t(locale, `completeness.${f}` as TranslationKey)).join(", "),
              })}
            </Text>
          )}
        </Paper>
      )}

      <Grid>
        <Grid.Col span={{ base: 12, md: 6 }}>
          <Card withBorder h="100%">
            <Title order={3} size="h5" mb="sm">
              {t(locale, "m5.savedFilters")}
            </Title>
            <form onSubmit={createFilter}>
              <Stack gap="xs">
                <TextInput
                  label={t(locale, "m5.filterName")}
                  value={filterName}
                  onChange={(e) => setFilterName(e.target.value)}
                  required
                />
                <TextInput
                  label={t(locale, "m5.filterSearch")}
                  value={filterSearch}
                  onChange={(e) => setFilterSearch(e.target.value)}
                />
                <Button type="submit" variant="default" loading={saving === "filter"} style={{ alignSelf: "flex-start" }}>
                  {t(locale, "m5.saveFilter")}
                </Button>
              </Stack>
            </form>
            <Stack gap="xs" mt="md">
              {filters.length === 0 && (
                <Text size="sm" c="dimmed">
                  {t(locale, "m5.noFilters")}
                </Text>
              )}
              {filters.map((filter) => (
                <Group key={filter.id} justify="space-between" wrap="nowrap">
                  <Text size="sm" truncate>
                    {filter.name}
                  </Text>
                  <Button
                    color="red"
                    variant="subtle"
                    style={{ flex: "none" }}
                    aria-label={t(locale, "m5.removeItem", { name: filter.name })}
                    onClick={() =>
                      void deleteSavedFilter(filter.id)
                        .then(() => setFilters((c) => c.filter((i) => i.id !== filter.id)))
                        .catch((err: unknown) =>
                          setError(errorMessage(locale, err)),
                        )
                    }
                  >
                    {t(locale, "m5.remove")}
                  </Button>
                </Group>
              ))}
            </Stack>
          </Card>
        </Grid.Col>

        <Grid.Col span={{ base: 12, md: 6 }}>
          <Card withBorder h="100%">
            <Title order={3} size="h5" mb="sm">
              {t(locale, "m5.tracker")}
            </Title>
            {applications.length === 0 ? (
              <Text size="sm" c="dimmed">
                {t(locale, "m5.noApplications")}
              </Text>
            ) : (
              <Stack gap="sm">
                {applications.map((application) => (
                  <Group key={application.id} justify="space-between" wrap="nowrap">
                    <Text size="sm" fw={500} truncate>
                      {application.job_title || application.job_key}
                    </Text>
                    <NativeSelect
                      w={190}
                      style={{ flex: "none" }}
                      value={application.status}
                      onChange={(e) => void changeStatus(application, e.currentTarget.value as Application["status"])}
                      data={STATUSES.map((status) => ({ value: status, label: t(locale, `status.${status}`) }))}
                      aria-label={t(locale, "m5.statusFor", { job: application.job_title || application.job_key || "" })}
                    />
                  </Group>
                ))}
              </Stack>
            )}
          </Card>
        </Grid.Col>
      </Grid>

      <Card withBorder>
        <Title order={3} size="h5" mb="sm">
          {t(locale, "m5.snippets")}
        </Title>
        <form onSubmit={createAnswer}>
          <Stack gap="xs">
            <TextInput
              label={t(locale, "m5.snippetName")}
              value={snippetName}
              onChange={(e) => setSnippetName(e.target.value)}
            />
            <Textarea
              rows={3}
              label={t(locale, "m5.snippetBodyLabel")}
              description={t(locale, "m5.snippetBody")}
              value={snippetBody}
              onChange={(e) => setSnippetBody(e.target.value)}
            />
            <Button type="submit" variant="default" loading={saving === "snippet"} style={{ alignSelf: "flex-start" }}>
              {t(locale, "m5.saveSnippet")}
            </Button>
          </Stack>
        </form>
        <Stack gap="xs" mt="md">
          {snippets.length === 0 && (
            <Text size="sm" c="dimmed">
              {t(locale, "m5.noSnippets")}
            </Text>
          )}
          {snippets.map((snippet) => (
            <Group key={snippet.id} justify="space-between" wrap="nowrap">
              <Text size="sm" lineClamp={2} style={{ flex: 1 }}>
                <Text span fw={600}>
                  {snippet.name}
                </Text>
                : {snippet.body}
              </Text>
              <Button
                color="red"
                variant="subtle"
                style={{ flex: "none" }}
                aria-label={t(locale, "m5.removeItem", { name: snippet.name })}
                onClick={() =>
                  void deleteSnippet(snippet.id)
                    .then(() => setSnippets((c) => c.filter((i) => i.id !== snippet.id)))
                    .catch((err: unknown) => setError(errorMessage(locale, err)))
                }
              >
                {t(locale, "m5.remove")}
              </Button>
            </Group>
          ))}
        </Stack>
      </Card>

      {error && (
        <Alert color="red" variant="light" role="alert" icon={<IconAlertCircle aria-hidden />}>
          {error}
        </Alert>
      )}
    </Stack>
  );
}
