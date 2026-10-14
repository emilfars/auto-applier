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
  Paper,
  Pill,
  Skeleton,
  Stack,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import { useDisclosure, useMediaQuery } from "@mantine/hooks";
import {
  IconAdjustmentsHorizontal,
  IconAlertCircle,
  IconChevronLeft,
  IconChevronRight,
  IconInfoCircle,
  IconMapPin,
  IconSearch,
} from "@tabler/icons-react";
import { fetchFeed, type FeedQuery, type FeedResponse } from "../api/feed";
import { EMPLOYMENT_TYPES } from "../api/profile";
import { t, type Locale } from "../i18n";
import { JobCard, employmentLabel } from "./JobCard";
import { Illustration } from "./Illustration";

type Status = "loading" | "ready" | "error";

const PAGE_SIZE = 24;
const FORM_ID = "feed-filters";
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

  const activeMore =
    [payMin, payMax, skills, maxYoE, postedAfter, source, employmentType].filter((v) => v !== "").length +
    (remoteOnly ? 1 : 0);

  const chips = [
    { key: "q", value: search, label: search, clear: () => setSearch("") },
    { key: "location", value: location, label: location, clear: () => setLocation("") },
    {
      key: "remote",
      value: remoteOnly ? "1" : "",
      label: t(locale, "feed.filter.remote"),
      clear: () => setRemoteOnly(false),
    },
    { key: "payMin", value: payMin, label: `≥ ${payMin}`, clear: () => setPayMin("") },
    { key: "payMax", value: payMax, label: `≤ ${payMax}`, clear: () => setPayMax("") },
    { key: "skills", value: skills, label: skills, clear: () => setSkills("") },
    { key: "maxYoE", value: maxYoE, label: `≤ ${maxYoE} ${t(locale, "feed.experience")}`, clear: () => setMaxYoE("") },
    { key: "postedAfter", value: postedAfter, label: postedAfter, clear: () => setPostedAfter("") },
    { key: "source", value: source, label: source, clear: () => setSource("") },
    {
      key: "employmentType",
      value: employmentType,
      label: employmentType && employmentLabel(locale, employmentType),
      clear: () => setEmploymentType(""),
    },
  ].filter((chip) => chip.value.trim() !== "");

  return (
    <Stack gap="lg" id="feed">
      <Paper
        withBorder
        p="sm"
        shadow="lg"
        bg="var(--mantine-color-default)"
        className="hero-overlap"
        mx={{ base: "sm", sm: "xl" }}
      >
        <form
          id={FORM_ID}
          aria-label={t(locale, "feed.filters.label")}
          onSubmit={(e) => {
            e.preventDefault();
            if (validNumberFilters(payMin, payMax, maxYoE)) setOffset(0);
          }}
        >
          <Group gap="sm" wrap="wrap" align="stretch">
            <TextInput
              size="md"
              variant="unstyled"
              leftSection={<IconSearch size={20} aria-hidden />}
              placeholder={t(locale, "feed.search.placeholder")}
              aria-label={t(locale, "feed.search.placeholder")}
              type="search"
              value={search}
              onChange={(e) => onFilter(() => setSearch(e.target.value))}
              style={{ flex: "2 1 240px" }}
            />
            <TextInput
              size="md"
              variant="unstyled"
              leftSection={<IconMapPin size={20} aria-hidden />}
              placeholder={t(locale, "feed.filter.location")}
              aria-label={t(locale, "feed.filter.location")}
              value={location}
              onChange={(e) => onFilter(() => setLocation(e.target.value))}
              style={{
                flex: "1 1 180px",
                [narrow ? "borderTop" : "borderLeft"]: "1px solid var(--mantine-color-default-border)",
              }}
            />
            <Group gap="sm" wrap="nowrap" style={{ flex: narrow ? "1 1 100%" : "none" }}>
              {narrow && (
                <Button
                  size="md"
                  variant="default"
                  leftSection={<IconAdjustmentsHorizontal size={18} aria-hidden />}
                  onClick={toggleMore}
                  aria-expanded={moreOpen}
                  aria-controls="feed-more-filters"
                  style={{ flex: 1 }}
                >
                  {t(locale, "feed.filter.more")}
                  {activeMore > 0 ? ` (${activeMore})` : ""}
                </Button>
              )}
              <Button size="md" type="submit" style={{ flex: narrow ? 1 : undefined }}>
                {t(locale, "feed.search.submit")}
              </Button>
            </Group>
          </Group>
        </form>
      </Paper>

      {chips.length > 0 && (
        <Group gap="xs" wrap="wrap" className="animate-in">
          {chips.map((chip) => (
            <Pill
              key={chip.key}
              size="lg"
              withRemoveButton
              onRemove={() => onFilter(chip.clear)}
              removeButtonProps={{ "aria-label": t(locale, "m5.removeItem", { name: chip.label }) }}
            >
              {chip.label}
            </Pill>
          ))}
        </Group>
      )}

      <Grid gutter={{ base: "md", md: "xl" }} align="flex-start">
        <Grid.Col span={{ base: 12, md: 4, lg: 3 }}>
          <Collapse in={!narrow || moreOpen} id="feed-more-filters" transitionDuration={0}>
            <Card padding="md" className={narrow ? undefined : "sticky-aside"}>
              <Group justify="space-between" mb="sm">
                <Title order={2} size="h4">
                  {t(locale, "feed.filters.heading")}
                </Title>
              </Group>
              <Stack gap="sm">
                <TextInput
                  form={FORM_ID}
                  label={t(locale, "feed.filter.payMin")}
                  type="number"
                  inputMode="numeric"
                  value={payMin}
                  onChange={(e) => onFilter(() => setPayMin(e.target.value))}
                />
                <TextInput
                  form={FORM_ID}
                  label={t(locale, "feed.filter.payMax")}
                  type="number"
                  inputMode="numeric"
                  value={payMax}
                  onChange={(e) => onFilter(() => setPayMax(e.target.value))}
                />
                <TextInput
                  form={FORM_ID}
                  label={t(locale, "feed.filter.skills")}
                  value={skills}
                  onChange={(e) => onFilter(() => setSkills(e.target.value))}
                />
                <TextInput
                  form={FORM_ID}
                  label={t(locale, "feed.filter.maxYoe")}
                  type="number"
                  inputMode="numeric"
                  value={maxYoE}
                  onChange={(e) => onFilter(() => setMaxYoE(e.target.value))}
                />
                <TextInput
                  form={FORM_ID}
                  label={t(locale, "feed.filter.postedAfter")}
                  type="date"
                  value={postedAfter}
                  onChange={(e) => onFilter(() => setPostedAfter(e.target.value))}
                />
                <NativeSelect
                  form={FORM_ID}
                  label={t(locale, "feed.filter.source")}
                  data={[
                    { value: "", label: t(locale, "feed.filter.all") },
                    ...SOURCE_IDS.map((s) => ({ value: s, label: s })),
                  ]}
                  value={source}
                  onChange={(e) => onFilter(() => setSource(e.target.value))}
                />
                <NativeSelect
                  form={FORM_ID}
                  label={t(locale, "feed.filter.employmentType")}
                  data={[
                    { value: "", label: t(locale, "feed.filter.all") },
                    ...EMPLOYMENT_TYPES.map((etype) => ({ value: etype, label: employmentLabel(locale, etype) })),
                  ]}
                  value={employmentType}
                  onChange={(e) => onFilter(() => setEmploymentType(e.target.value))}
                />
                <Checkbox
                  form={FORM_ID}
                  label={t(locale, "feed.filter.remote")}
                  checked={remoteOnly}
                  onChange={(e) => onFilter(() => setRemoteOnly(e.currentTarget.checked))}
                  styles={{ body: { alignItems: "center", minHeight: 44 } }}
                />
                <Button variant="default" onClick={clearFilters}>
                  {t(locale, "feed.filter.clear")}
                </Button>
              </Stack>
            </Card>
          </Collapse>
        </Grid.Col>

        <Grid.Col span={{ base: 12, md: 8, lg: 9 }}>
          <Stack gap="md">
            <Paper p="sm" radius="md" bg="var(--mantine-color-blue-light)">
              <Group gap="sm" wrap="nowrap" align="flex-start">
                <IconInfoCircle
                  size={20}
                  aria-hidden
                  style={{ flex: "none", color: "var(--mantine-color-blue-light-color)" }}
                />
                <Text size="sm">{t(locale, "feed.openFill.note")}</Text>
              </Group>
            </Paper>

            {status === "loading" && (
              <Stack gap="md" aria-busy="true" aria-live="polite">
                <Text size="sm" c="dimmed">
                  {t(locale, "feed.loading")}
                </Text>
                {Array.from({ length: 4 }, (_, i) => (
                  <Card key={i}>
                    <Group align="flex-start" wrap="nowrap" gap="md">
                      <Skeleton height={48} width={48} radius="md" />
                      <Stack gap="sm" style={{ flex: 1 }}>
                        <Skeleton height={18} width="55%" />
                        <Skeleton height={14} width="35%" />
                        <Skeleton height={14} width="70%" />
                      </Stack>
                    </Group>
                  </Card>
                ))}
              </Stack>
            )}

            {status === "error" && (
              <Alert color="red" variant="light" role="alert" icon={<IconAlertCircle aria-hidden />} radius="md">
                <Stack gap="xs" align="flex-start">
                  <Text size="sm">{t(locale, "feed.error")}</Text>
                  <Button variant="default" onClick={() => setAttempt((n) => n + 1)}>
                    {t(locale, "feed.retry")}
                  </Button>
                </Stack>
              </Alert>
            )}

            {status === "ready" && data && (
              <>
                <Text size="sm" fw={600} c="dimmed">
                  {data.total} {t(locale, "feed.results.count")}
                </Text>

                {data.jobs.length === 0 ? (
                  <Card padding="xl">
                    <Stack gap="sm" align="center" py="lg" ta="center">
                      <Illustration name="search" width={200} />
                      <Text fw={600}>{t(locale, "feed.empty")}</Text>
                      <Button variant="default" onClick={clearFilters}>
                        {t(locale, "feed.filter.clear")}
                      </Button>
                    </Stack>
                  </Card>
                ) : (
                  <Stack gap="md" className="stagger" key={`${offset}-${attempt}`}>
                    {data.jobs.map((job, index) => (
                      <JobCard
                        index={index}
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
                  </Stack>
                )}

                {pages > 1 && (
                  <Group justify="space-between" gap="md" pt="sm">
                    <Button
                      variant="default"
                      leftSection={<IconChevronLeft size={16} aria-hidden />}
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
                      rightSection={<IconChevronRight size={16} aria-hidden />}
                      disabled={offset + PAGE_SIZE >= total}
                      onClick={() => setOffset((o) => o + PAGE_SIZE)}
                    >
                      {t(locale, "feed.page.next")}
                    </Button>
                  </Group>
                )}
              </>
            )}
          </Stack>
        </Grid.Col>
      </Grid>
    </Stack>
  );
}
