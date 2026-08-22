import { useEffect, useRef, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Grid,
  Group,
  NativeSelect,
  Paper,
  SimpleGrid,
  Skeleton,
  Stack,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import { IconSearch, IconMapPin } from "@tabler/icons-react";
import { fetchFeed, type FeedQuery, type FeedResponse } from "../api/feed";
import { EMPLOYMENT_TYPES } from "../api/profile";
import { t, type Locale } from "../i18n";
import { JobCard } from "./JobCard";

type Status = "loading" | "ready" | "error";

const PAGE_SIZE = 24;
const SOURCE_IDS = ["kalibrr", "jooble", "glints", "jobstreet", "kemnaker", "company-career"];

function parseNumberFilter(value: string): number | undefined {
  if (!value.trim()) return undefined;
  const parsed = Number(value);
  return Number.isSafeInteger(parsed) && parsed >= 0 ? parsed : undefined;
}

function validNumberFilters(payMin: string, payMax: string, maxYoE: string): boolean {
  const min = parseNumberFilter(payMin);
  const max = parseNumberFilter(payMax);
  return (
    (!payMin.trim() || min != null) &&
    (!payMax.trim() || max != null) &&
    (!maxYoE.trim() || parseNumberFilter(maxYoE) != null) &&
    (min == null || max == null || min <= max)
  );
}

function dateFilterToRFC3339(value: string): string | undefined {
  return /^\d{4}-\d{2}-\d{2}$/.test(value) ? `${value}T00:00:00Z` : undefined;
}

export function JobFeed({
  locale,
  canFill = false,
  fillReason,
  signedIn = false,
}: {
  locale: Locale;
  canFill?: boolean;
  fillReason?: "needLogin" | "needProfile";
  signedIn?: boolean;
}) {
  const [search, setSearch] = useState("");
  const [location, setLocation] = useState("");
  const [remoteOnly, setRemoteOnly] = useState(false);
  const [payMin, setPayMin] = useState("");
  const [payMax, setPayMax] = useState("");
  const [skills, setSkills] = useState("");
  const [maxYoE, setMaxYoE] = useState("");
  const [postedAfter, setPostedAfter] = useState("");
  const [source, setSource] = useState("");
  const [employmentType, setEmploymentType] = useState("");
  const [offset, setOffset] = useState(0);
  const [data, setData] = useState<FeedResponse | null>(null);
  const [status, setStatus] = useState<Status>("loading");
  const debounce = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  const payMinValue = parseNumberFilter(payMin);
  const payMaxValue = parseNumberFilter(payMax);

  const onFilter = (fn: () => void) => {
    fn();
    setOffset(0);
  };

  useEffect(() => {
    const controller = new AbortController();
    if (!validNumberFilters(payMin, payMax, maxYoE)) return;
    const maxYoEValue = parseNumberFilter(maxYoE);
    const postedAfterValue = dateFilterToRFC3339(postedAfter);
    if (postedAfter && !postedAfterValue) return;

    const query: FeedQuery = { limit: PAGE_SIZE, offset };
    if (search.trim()) query.q = search;
    if (location.trim()) query.location = location;
    if (remoteOnly) query.remote = true;
    if (payMinValue != null) query.pay_min = payMinValue;
    if (payMaxValue != null) query.pay_max = payMaxValue;
    if (skills.trim()) query.skills = skills;
    if (maxYoEValue != null) query.max_yoe = maxYoEValue;
    if (postedAfterValue) query.posted_after = postedAfterValue;
    if (source) query.source = source;
    if (employmentType) query.employment_type = employmentType;

    clearTimeout(debounce.current);
    debounce.current = setTimeout(() => {
      setStatus("loading");
      fetchFeed(query, controller.signal)
        .then((res) => {
          setData(res);
          setStatus("ready");
        })
        .catch((err: unknown) => {
          if (err instanceof DOMException && err.name === "AbortError") return;
          setStatus("error");
        });
    }, 250);

    return () => {
      controller.abort();
      clearTimeout(debounce.current);
    };
  }, [
    search,
    location,
    remoteOnly,
    payMin,
    payMax,
    skills,
    maxYoE,
    postedAfter,
    source,
    employmentType,
    offset,
  ]);

  const clearFilters = () => {
    setSearch("");
    setLocation("");
    setRemoteOnly(false);
    setPayMin("");
    setPayMax("");
    setSkills("");
    setMaxYoE("");
    setPostedAfter("");
    setSource("");
    setEmploymentType("");
    setOffset(0);
  };

  const total = data?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const page = Math.floor(offset / PAGE_SIZE) + 1;

  return (
    <Stack gap="md" id="feed">
      <div>
        <Text size="xs" fw={700} tt="uppercase" c="blue" style={{ letterSpacing: "0.18em" }}>
          Auto Applier
        </Text>
        <Title order={2} mt={4}>
          {t(locale, "feed.heading")}
        </Title>
      </div>

      <Grid gutter="lg" align="flex-start">
        <Grid.Col span={{ base: 12, lg: 3 }}>
          <Paper withBorder shadow="sm" p="md" radius="lg" style={{ position: "sticky", top: 80 }}>
            <form
              onSubmit={(e) => {
                e.preventDefault();
                if (validNumberFilters(payMin, payMax, maxYoE)) setOffset(0);
              }}
            >
              <Stack gap="sm">
                <TextInput
                  leftSection={<IconSearch size={16} />}
                  placeholder={t(locale, "feed.search.placeholder")}
                  aria-label={t(locale, "feed.search.placeholder")}
                  type="search"
                  value={search}
                  onChange={(e) => onFilter(() => setSearch(e.target.value))}
                />
                <TextInput
                  leftSection={<IconMapPin size={16} />}
                  placeholder={t(locale, "feed.filter.location")}
                  aria-label={t(locale, "feed.filter.location")}
                  value={location}
                  onChange={(e) => onFilter(() => setLocation(e.target.value))}
                />
                <TextInput
                  label={t(locale, "feed.filter.payMin")}
                  type="number"
                  aria-label={t(locale, "feed.filter.payMin")}
                  value={payMin}
                  onChange={(e) => onFilter(() => setPayMin(e.target.value))}
                />
                <TextInput
                  label={t(locale, "feed.filter.payMax")}
                  type="number"
                  aria-label={t(locale, "feed.filter.payMax")}
                  value={payMax}
                  onChange={(e) => onFilter(() => setPayMax(e.target.value))}
                />
                <TextInput
                  label={t(locale, "feed.filter.skills")}
                  aria-label={t(locale, "feed.filter.skills")}
                  value={skills}
                  onChange={(e) => onFilter(() => setSkills(e.target.value))}
                />
                <TextInput
                  label={t(locale, "feed.filter.maxYoe")}
                  type="number"
                  aria-label={t(locale, "feed.filter.maxYoe")}
                  value={maxYoE}
                  onChange={(e) => onFilter(() => setMaxYoE(e.target.value))}
                />
                <TextInput
                  label={t(locale, "feed.filter.postedAfter")}
                  type="date"
                  aria-label={t(locale, "feed.filter.postedAfter")}
                  value={postedAfter}
                  onChange={(e) => onFilter(() => setPostedAfter(e.target.value))}
                />
                <NativeSelect
                  label={t(locale, "feed.filter.source")}
                  aria-label={t(locale, "feed.filter.source")}
                  data={[{ value: "", label: t(locale, "feed.filter.all") }, ...SOURCE_IDS.map((s) => ({ value: s, label: s }))]}
                  value={source}
                  onChange={(e) => onFilter(() => setSource(e.target.value))}
                />
                <NativeSelect
                  label={t(locale, "feed.filter.employmentType")}
                  aria-label={t(locale, "feed.filter.employmentType")}
                  data={[{ value: "", label: t(locale, "feed.filter.all") }, ...EMPLOYMENT_TYPES.map((etype) => ({ value: etype, label: etype.replace("_", " ") }))]}
                  value={employmentType}
                  onChange={(e) => onFilter(() => setEmploymentType(e.target.value))}
                />
                <Checkbox
                  label={t(locale, "feed.filter.remote")}
                  checked={remoteOnly}
                  onChange={(e) => onFilter(() => setRemoteOnly(e.currentTarget.checked))}
                />
                <Button type="submit" fullWidth>
                  {t(locale, "feed.filter.apply")}
                </Button>
                <Button variant="default" fullWidth onClick={clearFilters}>
                  {t(locale, "feed.filter.clear")}
                </Button>
              </Stack>
            </form>
          </Paper>
        </Grid.Col>

        <Grid.Col span={{ base: 12, lg: 9 }}>
          {status === "loading" && (
            <Stack gap="md" aria-busy="true" aria-live="polite">
              <Text size="sm" c="dimmed">
                {t(locale, "feed.loading")}
              </Text>
              <SimpleGrid cols={{ base: 1, sm: 2, xl: 3 }} spacing="md">
                {Array.from({ length: 6 }, (_, i) => (
                  <Card key={i} withBorder radius="lg" p="lg">
                    <Skeleton height={18} width="70%" mb="md" />
                    <Skeleton height={14} width="50%" mb="lg" />
                    <Skeleton height={36} radius="md" />
                  </Card>
                ))}
              </SimpleGrid>
            </Stack>
          )}

          {status === "error" && (
            <Alert color="red" role="alert">
              {t(locale, "feed.error")}
            </Alert>
          )}

          {status === "ready" && data && (
            <Stack gap="md">
              <Text size="sm" c="dimmed">
                {data.total} {t(locale, "feed.results.count")}
              </Text>

              {data.jobs.length === 0 ? (
                <Paper withBorder p="xl" radius="lg" style={{ borderStyle: "dashed", textAlign: "center" }}>
                  <Text size="sm" c="dimmed">
                    {t(locale, "feed.empty")}
                  </Text>
                </Paper>
              ) : (
                <SimpleGrid cols={{ base: 1, sm: 2, xl: 3 }} spacing="md">
                  {data.jobs.map((job) => (
                    <JobCard
                      key={`${job.source}:${job.source_url}`}
                      job={job}
                      locale={locale}
                      canFill={canFill}
                      fillReason={fillReason}
                      signedIn={signedIn}
                      onDismiss={() =>
                        setData((cur) => (cur ? { ...cur, jobs: cur.jobs.filter((j) => j !== job) } : cur))
                      }
                    />
                  ))}
                </SimpleGrid>
              )}

              {pages > 1 && (
                <Group justify="center" gap="md">
                  <Button
                    variant="default"
                    disabled={offset === 0}
                    onClick={() => setOffset((o) => Math.max(0, o - PAGE_SIZE))}
                  >
                    {t(locale, "feed.page.prev")}
                  </Button>
                  <Text size="sm" fw={600} c="dimmed">
                    {t(locale, "feed.page.status", { page, pages })}
                  </Text>
                  <Button
                    variant="default"
                    disabled={offset + PAGE_SIZE >= total}
                    onClick={() => setOffset((o) => o + PAGE_SIZE)}
                  >
                    {t(locale, "feed.page.next")}
                  </Button>
                </Group>
              )}
            </Stack>
          )}
        </Grid.Col>
      </Grid>
    </Stack>
  );
}
