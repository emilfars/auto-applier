import { useEffect, useState } from "react";
import {
  ActionIcon,
  Alert,
  Button,
  Card,
  Grid,
  Group,
  NativeSelect,
  RingProgress,
  SimpleGrid,
  Skeleton,
  Stack,
  Table,
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
import { IconAlertCircle, IconBookmark, IconListCheck, IconMessage2, IconTrash } from "@tabler/icons-react";
import { Illustration } from "./Illustration";
import { t, type Locale, type TranslationKey } from "../i18n";
import { errorMessage } from "../i18n/errors";

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
  const [ring, setRing] = useState(0);
  // Starts the ring at zero so it visibly fills once data arrives.
  useEffect(() => setRing(completeness?.score ?? 0), [completeness]);

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
      <Stack gap="lg" aria-busy="true" aria-live="polite">
        <Text size="sm" c="dimmed">
          {t(locale, "m5.loading")}
        </Text>
        <Skeleton height={120} radius="lg" />
        <Skeleton height={220} radius="lg" />
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

  const removeButton = (name: string, onClick: () => void) => (
    <ActionIcon
      variant="subtle"
      color="red"
      size={44}
      aria-label={t(locale, "m5.removeItem", { name })}
      title={t(locale, "m5.remove")}
      onClick={onClick}
      style={{ flex: "none" }}
    >
      <IconTrash size={18} aria-hidden />
    </ActionIcon>
  );

  const stats = [
    { label: t(locale, "nav.applications"), value: applications.length, icon: IconListCheck },
    { label: t(locale, "m5.savedFilters"), value: filters.length, icon: IconBookmark },
    { label: t(locale, "m5.snippets"), value: snippets.length, icon: IconMessage2 },
  ];

  return (
    <Stack gap="lg" className="stagger">
      <SimpleGrid cols={{ base: 1, xs: 3 }} spacing="lg" style={{ "--i": 0 } as React.CSSProperties}>
        {stats.map(({ label, value, icon: Icon }) => (
          <Card key={label} className="stat-tile">
            <Group gap="md" wrap="nowrap">
              <span className="icon-tile" aria-hidden="true">
                <Icon size={22} />
              </span>
              <div style={{ minWidth: 0 }}>
                <Text fw={700} style={{ fontSize: "1.75rem", lineHeight: 1.1 }}>
                  {value}
                </Text>
                <Text size="sm" c="dimmed" truncate>
                  {label}
                </Text>
              </div>
            </Group>
          </Card>
        ))}
      </SimpleGrid>

      {completeness && (
        <Card style={{ "--i": 1 } as React.CSSProperties}>
          <Group gap="lg" wrap="nowrap" align="center">
            <RingProgress
              size={96}
              thickness={9}
              roundCaps
              transitionDuration={900}
              sections={[{ value: ring, color: completeness.complete ? "teal" : "blue" }]}
              label={
                <Text ta="center" fw={700} size="sm">
                  {completeness.score}%
                </Text>
              }
            />
            <Stack gap={6} style={{ minWidth: 0 }}>
              <Title order={2} size="h3">
                {t(locale, "m5.completeness")}
              </Title>
              {completeness.missing.length > 0 && (
                <Text size="sm" c="dimmed">
                  {t(locale, "m5.missing", {
                    fields: completeness.missing
                      .map((f) => t(locale, `completeness.${f}` as TranslationKey))
                      .join(", "),
                  })}
                </Text>
              )}
            </Stack>
          </Group>
        </Card>
      )}

      <Card padding={0} style={{ "--i": 2 } as React.CSSProperties}>
        <Title order={2} size="h3" p="lg" pb="sm">
          {t(locale, "m5.tracker")}
        </Title>
        {applications.length === 0 ? (
          <Stack align="center" gap="xs" px="lg" pb="xl" ta="center">
            <Illustration name="tracker" width={160} />
            <Text size="sm" c="dimmed">
              {t(locale, "m5.noApplications")}
            </Text>
          </Stack>
        ) : (
          <Table verticalSpacing="sm" horizontalSpacing="lg" highlightOnHover>
            <Table.Tbody>
              {applications.map((application) => (
                <Table.Tr key={application.id}>
                  <Table.Td>
                    <Text size="sm" fw={500} lineClamp={2}>
                      {application.job_title || application.job_key}
                    </Text>
                    {application.company && (
                      <Text size="xs" c="dimmed">
                        {application.company}
                      </Text>
                    )}
                  </Table.Td>
                  <Table.Td w={1}>
                    <NativeSelect
                      w={{ base: 150, sm: 220 }}
                      value={application.status}
                      onChange={(e) => void changeStatus(application, e.currentTarget.value as Application["status"])}
                      data={STATUSES.map((status) => ({ value: status, label: t(locale, `status.${status}`) }))}
                      aria-label={t(locale, "m5.statusFor", {
                        job: application.job_title || application.job_key || "",
                      })}
                    />
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        )}
      </Card>

      <Grid gutter="lg" style={{ "--i": 3 } as React.CSSProperties}>
        <Grid.Col span={{ base: 12, md: 6 }}>
          <Card h="100%">
            <Title order={2} size="h3" mb="md">
              {t(locale, "m5.savedFilters")}
            </Title>
            <form onSubmit={createFilter}>
              <Stack gap="sm">
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
                <Button type="submit" variant="light" loading={saving === "filter"} style={{ alignSelf: "flex-start" }}>
                  {t(locale, "m5.saveFilter")}
                </Button>
              </Stack>
            </form>
            <Stack gap={4} mt="lg">
              {filters.length === 0 && (
                <Text size="sm" c="dimmed">
                  {t(locale, "m5.noFilters")}
                </Text>
              )}
              {filters.map((filter) => (
                <Group key={filter.id} justify="space-between" wrap="nowrap" gap="xs">
                  <Text size="sm" fw={500} truncate>
                    {filter.name}
                  </Text>
                  {removeButton(
                    filter.name,
                    () =>
                      void deleteSavedFilter(filter.id)
                        .then(() => setFilters((c) => c.filter((i) => i.id !== filter.id)))
                        .catch((err: unknown) => setError(errorMessage(locale, err))),
                  )}
                </Group>
              ))}
            </Stack>
          </Card>
        </Grid.Col>

        <Grid.Col span={{ base: 12, md: 6 }}>
          <Card h="100%">
            <Title order={2} size="h3" mb="md">
              {t(locale, "m5.snippets")}
            </Title>
            <form onSubmit={createAnswer}>
              <Stack gap="sm">
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
                <Button
                  type="submit"
                  variant="light"
                  loading={saving === "snippet"}
                  style={{ alignSelf: "flex-start" }}
                >
                  {t(locale, "m5.saveSnippet")}
                </Button>
              </Stack>
            </form>
            <Stack gap={4} mt="lg">
              {snippets.length === 0 && (
                <Text size="sm" c="dimmed">
                  {t(locale, "m5.noSnippets")}
                </Text>
              )}
              {snippets.map((snippet) => (
                <Group key={snippet.id} justify="space-between" wrap="nowrap" gap="xs">
                  <Text size="sm" lineClamp={2} style={{ flex: 1 }}>
                    <Text span fw={600}>
                      {snippet.name}
                    </Text>
                    : {snippet.body}
                  </Text>
                  {removeButton(
                    snippet.name,
                    () =>
                      void deleteSnippet(snippet.id)
                        .then(() => setSnippets((c) => c.filter((i) => i.id !== snippet.id)))
                        .catch((err: unknown) => setError(errorMessage(locale, err))),
                  )}
                </Group>
              ))}
            </Stack>
          </Card>
        </Grid.Col>
      </Grid>

      {error && (
        <Alert color="red" variant="light" role="alert" icon={<IconAlertCircle aria-hidden />}>
          {error}
        </Alert>
      )}
    </Stack>
  );
}
