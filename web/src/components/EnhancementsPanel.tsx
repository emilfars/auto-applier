import { useEffect, useState } from "react";
import {
  Alert,
  Badge,
  Button,
  Card,
  Grid,
  Group,
  Paper,
  Progress,
  Select,
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
import { ApiError } from "../api/http";
import { t, type Locale } from "../i18n";

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

  useEffect(() => {
    if (!user) return;
    Promise.all([getCompleteness(), listSavedFilters(), listApplications(), listSnippets()])
      .then(([score, saved, tracked, reusable]) => {
        setCompleteness(score);
        setFilters(saved);
        setApplications(tracked);
        setSnippets(reusable);
      })
      .catch(() => setError(t(locale, "auth.error")));
  }, [user, locale]);

  if (!user) return null;

  async function createFilter(event: React.FormEvent) {
    event.preventDefault();
    if (!filterName.trim()) return;
    try {
      const filter = await saveFilter(filterName, { q: filterSearch });
      setFilters((current) => [...current, filter]);
      setFilterName("");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t(locale, "auth.error"));
    }
  }

  async function createAnswer(event: React.FormEvent) {
    event.preventDefault();
    if (!snippetName.trim() || !snippetBody.trim()) return;
    try {
      const snippet = await createSnippet(snippetName, snippetBody);
      setSnippets((current) => [...current, snippet]);
      setSnippetName("");
      setSnippetBody("");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t(locale, "auth.error"));
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
      setError(err instanceof ApiError ? err.message : t(locale, "auth.error"));
    }
  }

  return (
    <Stack gap="md">
      {completeness && (
        <Paper withBorder p="md" radius="md">
          <Group justify="space-between" mb={8}>
            <Title order={5}>{t(locale, "m5.completeness")}</Title>
            <Badge color={completeness.complete ? "teal" : "yellow"} size="lg">
              {completeness.score}%
            </Badge>
          </Group>
          <Progress value={completeness.score} color={completeness.complete ? "teal" : "blue"} />
          {completeness.missing.length > 0 && (
            <Text size="sm" c="dimmed" mt="xs">
              {t(locale, "m5.missing", { fields: completeness.missing.join(", ") })}
            </Text>
          )}
        </Paper>
      )}

      <Grid>
        <Grid.Col span={{ base: 12, md: 6 }}>
          <Card withBorder shadow="sm" radius="md" h="100%">
            <Title order={5} mb="sm">
              {t(locale, "m5.savedFilters")}
            </Title>
            <form onSubmit={createFilter}>
              <Stack gap="xs">
                <TextInput
                  placeholder={t(locale, "m5.filterName")}
                  aria-label={t(locale, "m5.filterName")}
                  value={filterName}
                  onChange={(e) => setFilterName(e.target.value)}
                  required
                />
                <TextInput
                  placeholder={t(locale, "m5.filterSearch")}
                  aria-label={t(locale, "m5.filterSearch")}
                  value={filterSearch}
                  onChange={(e) => setFilterSearch(e.target.value)}
                />
                <Button type="submit" size="xs">
                  {t(locale, "m5.saveFilter")}
                </Button>
              </Stack>
            </form>
            <Stack gap="xs" mt="md">
              {filters.map((filter) => (
                <Group key={filter.id} justify="space-between" wrap="nowrap">
                  <Text size="sm" truncate>
                    {filter.name}
                  </Text>
                  <Button
                    color="red"
                    variant="subtle"
                    size="compact-xs"
                    onClick={() =>
                      void deleteSavedFilter(filter.id)
                        .then(() => setFilters((c) => c.filter((i) => i.id !== filter.id)))
                        .catch((err: unknown) =>
                          setError(err instanceof ApiError ? err.message : t(locale, "auth.error")),
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
          <Card withBorder shadow="sm" radius="md" h="100%">
            <Title order={5} mb="sm">
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
                    <Select
                      size="xs"
                      w={150}
                      value={application.status}
                      onChange={(v) => void changeStatus(application, v as Application["status"])}
                      data={[
                        { value: "form_filled", label: "form filled" },
                        { value: "submitted", label: "submitted" },
                        { value: "viewed", label: "viewed" },
                        { value: "rejected", label: "rejected" },
                        { value: "interview", label: "interview" },
                      ]}
                      aria-label={application.job_title || application.job_key}
                    />
                  </Group>
                ))}
              </Stack>
            )}
          </Card>
        </Grid.Col>
      </Grid>

      <Card withBorder shadow="sm" radius="md">
        <Title order={5} mb="sm">
          {t(locale, "m5.snippets")}
        </Title>
        <form onSubmit={createAnswer}>
          <Stack gap="xs">
            <TextInput
              placeholder={t(locale, "m5.snippetName")}
              aria-label={t(locale, "m5.snippetName")}
              value={snippetName}
              onChange={(e) => setSnippetName(e.target.value)}
            />
            <Textarea
              rows={3}
              placeholder={t(locale, "m5.snippetBody")}
              aria-label={t(locale, "m5.snippetBody")}
              value={snippetBody}
              onChange={(e) => setSnippetBody(e.target.value)}
            />
            <Button type="submit" size="xs" style={{ alignSelf: "flex-start" }}>
              {t(locale, "m5.saveSnippet")}
            </Button>
          </Stack>
        </form>
        <Stack gap="xs" mt="md">
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
                size="compact-xs"
                onClick={() =>
                  void deleteSnippet(snippet.id)
                    .then(() => setSnippets((c) => c.filter((i) => i.id !== snippet.id)))
                    .catch((err: unknown) => setError(err instanceof ApiError ? err.message : t(locale, "auth.error")))
                }
              >
                {t(locale, "m5.remove")}
              </Button>
            </Group>
          ))}
        </Stack>
      </Card>

      {error && <Alert color="red" role="alert">{error}</Alert>}
    </Stack>
  );
}
