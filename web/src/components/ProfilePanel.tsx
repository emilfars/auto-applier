import { useCallback, useEffect, useState } from "react";
import type { ReactNode } from "react";
import {
  Alert,
  Button,
  Card,
  Checkbox,
  CloseButton,
  Grid,
  Group,
  NumberInput,
  Paper,
  Select,
  Skeleton,
  Stack,
  Text,
  TextInput,
  Textarea,
  ThemeIcon,
  Title,
} from "@mantine/core";
import { IconAlertCircle, IconCheck, IconCircleCheck, IconPlus } from "@tabler/icons-react";
import { useForm, type UseFormReturnType } from "@mantine/form";
import { t, type Locale, type TranslationKey } from "../i18n";
import { useSession } from "../auth/session";
import {
  confirmProfile,
  getProfile,
  patchProfile,
  EMPLOYMENT_TYPES,
  type EducationEntry,
  type Profile,
  type ProfilePatch,
  type WorkEntry,
} from "../api/profile";
import { errorMessage } from "../i18n/errors";
import { employmentLabel } from "./JobCard";
import { StateMarker } from "./StateMarker";

interface FormState {
  full_name: string;
  email: string;
  phone: string;
  linkedin_url: string;
  github_url: string;
  portfolio_url: string;
  address: string;
  city: string;
  summary: string;
  current_employer: string;
  current_title: string;
  highest_education: string;
  education: EducationEntry[];
  work_history: WorkEntry[];
  expected_salary: string;
  notice_period_days: string;
  work_authorization: string;
  employment_type: string;
  open_to_relocation: boolean;
  skills: string;
  preferred_locations: string;
}

function toForm(p: Profile): FormState {
  return {
    full_name: p.full_name,
    email: p.email,
    phone: p.phone,
    linkedin_url: p.linkedin_url,
    github_url: p.github_url,
    portfolio_url: p.portfolio_url,
    address: p.address,
    city: p.city,
    summary: p.summary,
    current_employer: p.current_employer,
    current_title: p.current_title,
    highest_education: p.highest_education,
    education: p.education ?? [],
    work_history: p.work_history ?? [],
    expected_salary: p.expected_salary != null ? String(p.expected_salary) : "",
    notice_period_days: p.notice_period_days != null ? String(p.notice_period_days) : "",
    work_authorization: p.work_authorization,
    employment_type: p.employment_type,
    open_to_relocation: p.open_to_relocation,
    skills: (p.skills ?? []).join(", "),
    preferred_locations: (p.preferred_locations ?? []).join(", "),
  };
}

function splitList(s: string): string[] {
  return s
    .split(",")
    .map((x) => x.trim())
    .filter(Boolean);
}

const EMPTY_EDUCATION: EducationEntry = { institution: "", degree: "", field: "", start_year: "", end_year: "" };
const EMPTY_WORK: WorkEntry = { company: "", title: "", start_date: "", end_date: "" };

/** Rows the user added but left completely blank are not sent. */
function filled<T extends object>(rows: T[]): T[] {
  return rows.filter((row) => Object.values(row).some((value) => String(value).trim() !== ""));
}

function toPatch(f: FormState): ProfilePatch {
  return {
    full_name: f.full_name,
    email: f.email,
    phone: f.phone,
    linkedin_url: f.linkedin_url,
    github_url: f.github_url,
    portfolio_url: f.portfolio_url,
    address: f.address,
    city: f.city,
    summary: f.summary,
    current_employer: f.current_employer,
    current_title: f.current_title,
    highest_education: f.highest_education,
    education: filled(f.education),
    work_history: filled(f.work_history),
    work_authorization: f.work_authorization,
    employment_type: f.employment_type,
    open_to_relocation: f.open_to_relocation,
    skills: splitList(f.skills),
    preferred_locations: splitList(f.preferred_locations),
    expected_salary: f.expected_salary.trim() ? Number(f.expected_salary) : null,
    notice_period_days: f.notice_period_days.trim() ? Number(f.notice_period_days) : null,
  };
}

/** Repeatable rows for education and work history: one labeled input per field. */
function EntryList<N extends "education" | "work_history">({
  form,
  locale,
  name,
  blank,
}: {
  form: UseFormReturnType<FormState>;
  locale: Locale;
  name: N;
  blank: FormState[N][number];
}) {
  const rows = form.values[name];
  const heading = t(locale, name === "education" ? "profile.education" : "profile.workHistory");
  return (
    <Stack gap="sm" role="group" aria-label={heading}>
      <Text fw={600} size="sm">
        {heading}
      </Text>
      {rows.length === 0 && (
        <Text size="sm" c="dimmed">
          {t(locale, `profile.${name}.empty`)}
        </Text>
      )}
      {rows.map((_, index) => (
        <Paper key={index} withBorder p="md" radius="md" bg="var(--mantine-color-default-hover)">
          <Group justify="space-between" mb="xs">
            <Text size="sm" fw={500} c="dimmed">
              {heading} {index + 1}
            </Text>
            <CloseButton
              size={44}
              aria-label={t(locale, "profile.entry.remove", { item: heading, n: index + 1 })}
              onClick={() => form.removeListItem(name, index)}
            />
          </Group>
          <Grid gutter="sm">
            {(Object.keys(blank) as Array<keyof typeof blank & string>).map((field) => (
              <Grid.Col key={field} span={{ base: 12, sm: 6 }}>
                <TextInput
                  label={t(locale, `profile.${name}.${field}` as TranslationKey)}
                  data-testid={`profile-${name}-${index}-${field}`}
                  {...form.getInputProps(`${name}.${index}.${field}`)}
                />
              </Grid.Col>
            ))}
          </Grid>
        </Paper>
      ))}
      <Button
        variant="light"
        leftSection={<IconPlus size={16} aria-hidden />}
        onClick={() => form.insertListItem(name, { ...blank })}
        data-testid={`profile-${name}-add`}
        style={{ alignSelf: "flex-start" }}
      >
        {t(locale, `profile.${name}.add`)}
      </Button>
    </Stack>
  );
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Card>
      <Title order={2} size="h3" mb="md">
        {title}
      </Title>
      <Grid gutter="md">{children}</Grid>
    </Card>
  );
}

export function ProfilePanel({
  locale,
  onConfirmedChange,
}: {
  locale: Locale;
  onConfirmedChange?: (confirmed: boolean) => void;
}) {
  const { user } = useSession();
  const [confirmed, setConfirmed] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const form = useForm<FormState>({
    initialValues: {
      full_name: "",
      email: "",
      phone: "",
      linkedin_url: "",
      github_url: "",
      portfolio_url: "",
      address: "",
      city: "",
      summary: "",
      current_employer: "",
      current_title: "",
      highest_education: "",
      education: [],
      work_history: [],
      expected_salary: "",
      notice_period_days: "",
      work_authorization: "",
      employment_type: "",
      open_to_relocation: false,
      skills: "",
      preferred_locations: "",
    },
    validate: {
      full_name: (v) => (v.trim() ? null : t(locale, "validation.required")),
      email: (v) => (/^\S+@\S+$/.test(v) ? null : t(locale, "validation.email")),
      phone: (v) => (v.trim() ? null : t(locale, "validation.required")),
      work_authorization: (v) => (v.trim() ? null : t(locale, "validation.required")),
      employment_type: (v) => (v.trim() ? null : t(locale, "validation.required")),
    },
  });

  const [loaded, setLoaded] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);

  const applyProfile = useCallback(
    (p: Profile) => {
      form.setValues(toForm(p));
      setConfirmed(p.confirmed);
      onConfirmedChange?.(p.confirmed);
      setLoaded(true);
    },
    // Keyed on onConfirmedChange only: Mantine form methods are stable, so
    // including them would recreate the callback on every render.
    [onConfirmedChange],
  );

  useEffect(() => {
    if (!user) return;
    setLoaded(false);
    setLoadError(null);
    const controller = new AbortController();
    getProfile(controller.signal)
      .then(applyProfile)
      .catch((err: unknown) => {
        if (err instanceof DOMException && err.name === "AbortError") return;
        setLoadError(errorMessage(locale, err));
      });
    return () => controller.abort();
  }, [user, applyProfile, attempt]);

  if (!user) {
    return (
      <Text size="sm" c="dimmed">
        {t(locale, "profile.loginRequired")}
      </Text>
    );
  }
  if (loadError) {
    return (
      <Alert color="red" variant="light" role="alert" icon={<IconAlertCircle aria-hidden />}>
        <Stack gap="xs" align="flex-start">
          <Text size="sm">
            {t(locale, "profile.loadError")} {loadError}
          </Text>
          <Button variant="default" onClick={() => setAttempt((n) => n + 1)}>
            {t(locale, "common.retry")}
          </Button>
        </Stack>
      </Alert>
    );
  }
  if (!loaded) {
    return (
      <Card>
        <Stack gap="sm" aria-busy="true" aria-live="polite">
          <Text size="sm" c="dimmed">
            {t(locale, "profile.loading")}
          </Text>
          <Skeleton height={40} radius="md" />
          <Skeleton height={40} radius="md" />
          <Skeleton height={96} radius="md" />
        </Stack>
      </Card>
    );
  }

  async function run(fn: () => Promise<void>) {
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await fn();
    } catch (err) {
      setError(errorMessage(locale, err));
    } finally {
      setBusy(false);
    }
  }

  const onSave = form.onSubmit((values) => {
    void run(async () => {
      const saved = await patchProfile(toPatch(values));
      applyProfile(saved);
      setNotice(t(locale, "profile.saved"));
    });
  });

  const onConfirm = () => {
    void run(async () => {
      const values = form.getValues();
      await patchProfile(toPatch(values));
      const saved = await confirmProfile();
      applyProfile(saved);
      setNotice(t(locale, "profile.confirmed"));
    });
  };

  const span2 = { base: 12, md: 6 };
  const field = (name: keyof FormState) => ({ "data-testid": `profile-${name}`, ...form.getInputProps(name) });

  const status = (
    <Card className="sticky-aside">
      <Stack gap="sm">
        <Title order={2} size="h3">
          {t(locale, "profile.status.heading")}
        </Title>
        {confirmed ? (
          <Group data-testid="profile-confirmed" gap="sm" wrap="nowrap" align="center">
            <ThemeIcon size={40} radius="xl" color="teal" className="pop" aria-hidden style={{ flex: "none" }}>
              <IconCheck size={22} stroke={3} />
            </ThemeIcon>
            <span data-testid="profile-confirmed-badge">
              <StateMarker state="success">{t(locale, "profile.confirmed")}</StateMarker>
            </span>
          </Group>
        ) : (
          <>
            <StateMarker state="warning">{t(locale, "profile.unconfirmed")}</StateMarker>
            <Text size="sm" c="dimmed">
              {t(locale, "profile.confirmNote")}
            </Text>
            <Button loading={busy} onClick={onConfirm} data-testid="profile-confirm" fullWidth>
              {t(locale, "profile.confirm")}
            </Button>
          </>
        )}
        {error && (
          <Alert
            color="red"
            variant="light"
            role="alert"
            icon={<IconAlertCircle aria-hidden />}
            data-testid="profile-status"
          >
            {error}
          </Alert>
        )}
        {notice && (
          <Alert color="teal" variant="light" icon={<IconCircleCheck aria-hidden />} data-testid="profile-status">
            {notice}
          </Alert>
        )}
      </Stack>
    </Card>
  );

  return (
    <Grid gutter="xl" align="flex-start">
      <Grid.Col span={{ base: 12, lg: 4 }} order={{ base: 1, lg: 2 }}>
        {status}
      </Grid.Col>
      <Grid.Col span={{ base: 12, lg: 8 }} order={{ base: 2, lg: 1 }}>
        <form onSubmit={onSave} noValidate data-testid="profile-form">
          <Stack gap="lg">
            <Section title={t(locale, "profile.section.contact")}>
              <Grid.Col span={12}>
                <TextInput label={t(locale, "profile.fullName")} required {...field("full_name")} />
              </Grid.Col>
              <Grid.Col span={span2}>
                <TextInput label={t(locale, "profile.email")} required type="email" {...field("email")} />
              </Grid.Col>
              <Grid.Col span={span2}>
                <TextInput label={t(locale, "profile.phone")} required type="tel" {...field("phone")} />
              </Grid.Col>
              <Grid.Col span={span2}>
                <TextInput label={t(locale, "profile.city")} {...field("city")} />
              </Grid.Col>
              <Grid.Col span={12}>
                <TextInput label={t(locale, "profile.address")} {...field("address")} />
              </Grid.Col>
            </Section>

            <Section title={t(locale, "profile.section.links")}>
              <Grid.Col span={span2}>
                <TextInput label={t(locale, "profile.linkedin")} type="url" {...field("linkedin_url")} />
              </Grid.Col>
              <Grid.Col span={span2}>
                <TextInput label={t(locale, "profile.github")} type="url" {...field("github_url")} />
              </Grid.Col>
              <Grid.Col span={span2}>
                <TextInput label={t(locale, "profile.portfolio")} type="url" {...field("portfolio_url")} />
              </Grid.Col>
            </Section>

            <Section title={t(locale, "profile.section.work")}>
              <Grid.Col span={12}>
                <Textarea label={t(locale, "profile.summary")} rows={4} autosize minRows={4} {...field("summary")} />
              </Grid.Col>
              <Grid.Col span={span2}>
                <TextInput label={t(locale, "profile.currentEmployer")} {...field("current_employer")} />
              </Grid.Col>
              <Grid.Col span={span2}>
                <TextInput label={t(locale, "profile.currentTitle")} {...field("current_title")} />
              </Grid.Col>
              <Grid.Col span={12}>
                <EntryList form={form} locale={locale} name="work_history" blank={EMPTY_WORK} />
              </Grid.Col>
            </Section>

            <Section title={t(locale, "profile.section.education")}>
              <Grid.Col span={span2}>
                <TextInput label={t(locale, "profile.highestEducation")} {...field("highest_education")} />
              </Grid.Col>
              <Grid.Col span={12}>
                <EntryList form={form} locale={locale} name="education" blank={EMPTY_EDUCATION} />
              </Grid.Col>
            </Section>

            <Section title={t(locale, "profile.section.preferences")}>
              <Grid.Col span={span2}>
                <TextInput label={t(locale, "profile.workAuth")} required {...field("work_authorization")} />
              </Grid.Col>
              <Grid.Col span={span2}>
                <Select
                  label={t(locale, "profile.employmentType")}
                  required
                  data={EMPLOYMENT_TYPES.map((et) => ({ value: et, label: employmentLabel(locale, et) }))}
                  {...field("employment_type")}
                />
              </Grid.Col>
              <Grid.Col span={span2}>
                <NumberInput
                  label={t(locale, "profile.expectedSalary")}
                  min={0}
                  allowDecimal={false}
                  thousandSeparator={locale === "en" ? "," : "."}
                  decimalSeparator={locale === "en" ? "." : ","}
                  data-testid="profile-expected_salary"
                  value={form.values.expected_salary ? Number(form.values.expected_salary) : undefined}
                  onChange={(v) => form.setFieldValue("expected_salary", v ? String(v) : "")}
                  hideControls
                />
              </Grid.Col>
              <Grid.Col span={span2}>
                <NumberInput
                  label={t(locale, "profile.noticePeriod")}
                  min={0}
                  max={365}
                  data-testid="profile-notice_period_days"
                  value={form.values.notice_period_days ? Number(form.values.notice_period_days) : undefined}
                  onChange={(v) => form.setFieldValue("notice_period_days", v ? String(v) : "")}
                  hideControls
                />
              </Grid.Col>
              <Grid.Col span={12}>
                <TextInput label={t(locale, "profile.preferredLocations")} required {...field("preferred_locations")} />
              </Grid.Col>
              <Grid.Col span={12}>
                <TextInput label={t(locale, "profile.skills")} required {...field("skills")} />
              </Grid.Col>
              <Grid.Col span={12}>
                <Checkbox
                  label={t(locale, "profile.relocation")}
                  data-testid="profile-open_to_relocation"
                  {...form.getInputProps("open_to_relocation", { type: "checkbox" })}
                />
              </Grid.Col>
            </Section>

            <Group
              className="sticky-actions"
              justify="flex-end"
              py="sm"
              px="md"
              style={{ borderRadius: "var(--mantine-radius-lg)" }}
            >
              <Button type="submit" loading={busy} data-testid="profile-save">
                {t(locale, "profile.save")}
              </Button>
            </Group>
          </Stack>
        </form>
      </Grid.Col>
    </Grid>
  );
}
