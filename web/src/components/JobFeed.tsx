import { useEffect, useRef, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Collapse,
  Grid,
  Group,
  NativeSelect,
  SimpleGrid,
  Skeleton,
  Stack,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import { useDisclosure, useMediaQuery } from "@mantine/hooks";
import { IconAdjustmentsHorizontal, IconAlertCircle, IconMapPin, IconSearch } from "@tabler/icons-react";
import { fetchFeed, type FeedQuery, type FeedResponse } from "../api/feed";
import { EMPLOYMENT_TYPES } from "../api/profile";
import { t, type Locale } from "../i18n";
import { JobCard, employmentLabel } from "./JobCard";

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
  const [attempt, setAttempt] = useState(0);
  const [moreOpen, { toggle: toggleMore }] = useDisclosure(false);
  const narrow = useMediaQuery("(max-width: 62em)", false, { getInitialValueInEffect: false });
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
    attempt,
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

  const activeMore = [payMin, payMax, skills, maxYoE, postedAfter, source, employmentType].filter((v) => v !== "").length + (remoteOnly ? 1 : 0);

  return (
    <Stack gap="md" id="feed">
      <Title order={2} size="h3">
        {t(locale, "feed.heading")}
      </Title>

      <Grid gutter="lg" align="flex-start">
        <Grid.Col span={{ base: 12, md: 4, lg: 3 }}>
          <form
            aria-label={t(locale, "feed.filters.label")}
            onSubmit={(e) => {
              e.preventDefault();
              if (validNumberFilters(payMin, payMax, maxYoE)) setOffset(0);
            }}
          >
            <Stack gap="sm">
              <TextInput
                leftSection={<IconSearch size={16} aria-hidden />}
                placeholder={t(locale, "feed.search.placeholder")}
                aria-label={t(locale, "feed.search.placeholder")}
                type="search"
                value={search}
                onChange={(e) => onFilter(() => setSearch(e.target.value))}
              />
              <TextInput
                leftSection={<IconMapPin size={16} aria-hidden />}
                placeholder={t(locale, "feed.filter.location")}
                aria-label={t(locale, "feed.filter.location")}
                value={location}
                onChange={(e) => onFilter(() => setLocation(e.target.value))}
              />
              {narrow && (
                <Button
                  variant="default"
                  leftSection={<IconAdjustmentsHorizontal size={16} aria-hidden />}
                  onClick={toggleMore}
                  aria-expanded={moreOpen}
                  aria-controls="feed-more-filters"
                >
                  {t(locale, "feed.filter.more")}
                  {activeMore > 0 ? ` (${activeMore})` : ""}
                </Button>
              )}
              <Collapse in={!narrow || moreOpen} id="feed-more-filters" transitionDuration={0}>
                <Stack gap="sm">
                  <TextInput
                    label={t(locale, "feed.filter.payMin")}
                    type="number"
                    inputMode="numeric"
                    value={payMin}
                    onChange={(e) => onFilter(() => setPayMin(e.target.value))}
                  />
                  <TextInput
                    label={t(locale, "feed.filter.payMax")}
                    type="number"
                    inputMode="numeric"
                    value={payMax}
                    onChange={(e) => onFilter(() => setPayMax(e.target.value))}
                  />
                  <TextInput
                    label={t(locale, "feed.filter.skills")}
                    value={skills}
                    onChange={(e) => onFilter(() => setSkills(e.target.value))}
                  />
                  <TextInput
                    label={t(locale, "feed.filter.maxYoe")}
                    type="number"
                    inputMode="numeric"
                    value={maxYoE}
                    onChange={(e) => onFilter(() => setMaxYoE(e.target.value))}
                  />
                  <TextInput
                    label={t(locale, "feed.filter.postedAfter")}
                    type="date"
                    value={postedAfter}
                    onChange={(e) => onFilter(() => setPostedAfter(e.target.value))}
                  />
                  <NativeSelect
                    label={t(locale, "feed.filter.source")}
                    data={[{ value: "", label: t(locale, "feed.filter.all") }, ...SOURCE_IDS.map((s) => ({ value: s, label: s }))]}
                    value={source}
                    onChange={(e) => onFilter(() => setSource(e.target.value))}
                  />
                  <NativeSelect
                    label={t(locale, "feed.filter.employmentType")}
                    data={[
                      { value: "", label: t(locale, "feed.filter.all") },
                      ...EMPLOYMENT_TYPES.map((etype) => ({ value: etype, label: employmentLabel(locale, etype) })),
                    ]}
                    value={employmentType}
                    onChange={(e) => onFilter(() => setEmploymentType(e.target.value))}
                  />
                  <Checkbox
                    label={t(locale, "feed.filter.remote")}
                    checked={remoteOnly}
                    onChange={(e) => onFilter(() => setRemoteOnly(e.currentTarget.checked))}
                    styles={{ body: { alignItems: "center", minHeight: 44 } }}
                  />
                  <Button variant="default" onClick={clearFilters}>
                    {t(locale, "feed.filter.clear")}
                  </Button>
                </Stack>
              </Collapse>
            </Stack>
          </form>
        </Grid.Col>

        <Grid.Col span={{ base: 12, md: 8, lg: 9 }}>
          {status === "loading" && (
            <Stack gap="md" aria-busy="true" aria-live="polite">
              <Text size="sm" c="dimmed">
                {t(locale, "feed.loading")}
              </Text>
              <SimpleGrid cols={{ base: 1, sm: 2, xl: 3 }} spacing="md">
                {Array.from({ length: 6 }, (_, i) => (
                  <Card key={i} withBorder padding="md">
                    <Skeleton height={18} width="70%" mb="md" />
                    <Skeleton height={14} width="50%" mb="lg" />
                    <Skeleton height={36} />
                  </Card>
                ))}
              </SimpleGrid>
            </Stack>
          )}

          {status === "error" && (
            <Alert color="red" variant="light" role="alert" icon={<IconAlertCircle aria-hidden />}>
              <Stack gap="xs" align="flex-start">
                <Text size="sm">{t(locale, "feed.error")}</Text>
                <Button variant="default" onClick={() => setAttempt((n) => n + 1)}>
                  {t(locale, "feed.retry")}
                </Button>
              </Stack>
            </Alert>
          )}

          {status === "ready" && data && (
            <Stack gap="md">
              <div>
                <Text size="sm" fw={600}>
                  {data.total} {t(locale, "feed.results.count")}
                </Text>
                <Text size="sm" c="dimmed" maw={640}>
                  {t(locale, "feed.openFill.note")}
                </Text>
              </div>

              {data.jobs.length === 0 ? (
                <Stack gap="xs" align="flex-start" py="lg">
                  <Text>{t(locale, "feed.empty")}</Text>
                  <Button variant="default" onClick={clearFilters}>
                    {t(locale, "feed.filter.clear")}
                  </Button>
                </Stack>
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
