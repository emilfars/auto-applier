import { useCallback, useEffect, useState } from "react";
import {
  Alert,
  Badge,
  Button,
  Checkbox,
  Grid,
  Group,
  JsonInput,
  NumberInput,
  Select,
  Skeleton,
  Stack,
  Text,
  TextInput,
  Textarea,
} from "@mantine/core";
import { useForm } from "@mantine/form";
import { t, type Locale } from "../i18n";
import { useSession } from "../auth/session";
import {
  confirmProfile,
  getProfile,
  patchProfile,
  EMPLOYMENT_TYPES,
  type Profile,
  type ProfilePatch,
} from "../api/profile";
import { ApiError } from "../api/http";

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
  education: string;
  work_history: string;
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
    education: JSON.stringify(p.education ?? [], null, 2),
    work_history: JSON.stringify(p.work_history ?? [], null, 2),
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
  return s.split(",").map((x) => x.trim()).filter(Boolean);
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
    education: JSON.parse(f.education) as Profile["education"],
    work_history: JSON.parse(f.work_history) as Profile["work_history"],
    work_authorization: f.work_authorization,
    employment_type: f.employment_type,
    open_to_relocation: f.open_to_relocation,
    skills: splitList(f.skills),
    preferred_locations: splitList(f.preferred_locations),
    expected_salary: f.expected_salary.trim() ? Number(f.expected_salary) : null,
    notice_period_days: f.notice_period_days.trim() ? Number(f.notice_period_days) : null,
  };
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
      education: "[]",
      work_history: "[]",
      expected_salary: "",
      notice_period_days: "",
      work_authorization: "",
      employment_type: "",
      open_to_relocation: false,
      skills: "",
      preferred_locations: "",
    },
    validate: {
      full_name: (v) => (v.trim() ? null : "Required"),
      email: (v) => (/^\S+@\S+$/.test(v) ? null : "Invalid email"),
      phone: (v) => (v.trim() ? null : "Required"),
      work_authorization: (v) => (v.trim() ? null : "Required"),
      employment_type: (v) => (v.trim() ? null : "Required"),
      education: (v) => {
        try { JSON.parse(v); return null; } catch { return "Invalid JSON"; }
      },
      work_history: (v) => {
        try { JSON.parse(v); return null; } catch { return "Invalid JSON"; }
      },
    },
  });

  const [loaded, setLoaded] = useState(false);

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
    const controller = new AbortController();
    getProfile(controller.signal)
      .then(applyProfile)
      .catch((err: unknown) => {
        if (err instanceof DOMException && err.name === "AbortError") return;
        setError(t(locale, "auth.error"));
      });
    return () => controller.abort();
  }, [user, applyProfile]);

  if (!user) {
    return <Text size="sm" c="dimmed">{t(locale, "profile.loginRequired")}</Text>;
  }
  if (!loaded) {
    return (
      <Stack gap="sm">
        <Skeleton height={40} radius="md" />
        <Skeleton height={40} radius="md" />
        <Skeleton height={96} radius="md" />
      </Stack>
    );
  }

  async function run(fn: () => Promise<void>) {
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await fn();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t(locale, "auth.error"));
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

  return (
    <Stack gap="md">
      <form onSubmit={onSave} noValidate data-testid="profile-form">
        <Grid gutter="md">
          <Grid.Col span={12}>
            <TextInput label={t(locale, "profile.fullName")} required data-testid="profile-full_name" {...form.getInputProps("full_name")} />
          </Grid.Col>
          <Grid.Col span={{ base: 12, md: 6 }}>
            <TextInput label={t(locale, "profile.email")} required type="email" data-testid="profile-email" {...form.getInputProps("email")} />
          </Grid.Col>
          <Grid.Col span={{ base: 12, md: 6 }}>
            <TextInput label={t(locale, "profile.phone")} required data-testid="profile-phone" {...form.getInputProps("phone")} />
          </Grid.Col>
          <Grid.Col span={{ base: 12, md: 6 }}>
            <TextInput label={t(locale, "profile.linkedin")} type="url" data-testid="profile-linkedin_url" {...form.getInputProps("linkedin_url")} />
          </Grid.Col>
          <Grid.Col span={{ base: 12, md: 6 }}>
            <TextInput label={t(locale, "profile.github")} type="url" data-testid="profile-github_url" {...form.getInputProps("github_url")} />
          </Grid.Col>
          <Grid.Col span={{ base: 12, md: 6 }}>
            <TextInput label={t(locale, "profile.portfolio")} type="url" data-testid="profile-portfolio_url" {...form.getInputProps("portfolio_url")} />
          </Grid.Col>
          <Grid.Col span={{ base: 12, md: 6 }}>
            <TextInput label={t(locale, "profile.city")} data-testid="profile-city" {...form.getInputProps("city")} />
          </Grid.Col>
          <Grid.Col span={12}>
            <TextInput label={t(locale, "profile.address")} data-testid="profile-address" {...form.getInputProps("address")} />
          </Grid.Col>
          <Grid.Col span={12}>
            <Textarea label={t(locale, "profile.summary")} rows={4} data-testid="profile-summary" {...form.getInputProps("summary")} />
          </Grid.Col>
          <Grid.Col span={{ base: 12, md: 6 }}>
            <TextInput label={t(locale, "profile.currentEmployer")} data-testid="profile-current_employer" {...form.getInputProps("current_employer")} />
          </Grid.Col>
          <Grid.Col span={{ base: 12, md: 6 }}>
            <TextInput label={t(locale, "profile.currentTitle")} data-testid="profile-current_title" {...form.getInputProps("current_title")} />
          </Grid.Col>
          <Grid.Col span={{ base: 12, md: 6 }}>
            <TextInput label={t(locale, "profile.highestEducation")} data-testid="profile-highest_education" {...form.getInputProps("highest_education")} />
          </Grid.Col>
          <Grid.Col span={{ base: 12, md: 6 }}>
            <TextInput label={t(locale, "profile.workAuth")} required data-testid="profile-work_authorization" {...form.getInputProps("work_authorization")} />
          </Grid.Col>
          <Grid.Col span={{ base: 12, md: 4 }}>
            <NumberInput
              label={t(locale, "profile.expectedSalary")}
              min={0}
              data-testid="profile-expected_salary"
              value={form.values.expected_salary ? Number(form.values.expected_salary) : undefined}
              onChange={(v) => form.setFieldValue("expected_salary", v ? String(v) : "")}
              placeholder="IDR"
              hideControls
            />
          </Grid.Col>
          <Grid.Col span={{ base: 12, md: 4 }}>
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
          <Grid.Col span={{ base: 12, md: 4 }}>
            <Select
              label={t(locale, "profile.employmentType")}
              required
              data-testid="profile-employment_type"
              data={EMPLOYMENT_TYPES.map((et) => ({ value: et, label: et.replace("_", " ") }))}
              {...form.getInputProps("employment_type")}
            />
          </Grid.Col>
          <Grid.Col span={12}>
            <Checkbox label={t(locale, "profile.relocation")} data-testid="profile-open_to_relocation" {...form.getInputProps("open_to_relocation", { type: "checkbox" })} />
          </Grid.Col>
          <Grid.Col span={12}>
            <JsonInput label={t(locale, "profile.education")} formatOnBlur autosize minRows={6} data-testid="profile-education" {...form.getInputProps("education")} />
          </Grid.Col>
          <Grid.Col span={12}>
            <JsonInput label={t(locale, "profile.workHistory")} formatOnBlur autosize minRows={6} data-testid="profile-work_history" {...form.getInputProps("work_history")} />
          </Grid.Col>
          <Grid.Col span={12}>
            <TextInput label={t(locale, "profile.skills")} required data-testid="profile-skills" {...form.getInputProps("skills")} />
          </Grid.Col>
          <Grid.Col span={12}>
            <TextInput label={t(locale, "profile.preferredLocations")} required data-testid="profile-preferred_locations" {...form.getInputProps("preferred_locations")} />
          </Grid.Col>
          <Grid.Col span={12}>
            <Group>
              <Button type="submit" loading={busy} data-testid="profile-save">{t(locale, "profile.save")}</Button>
            </Group>
          </Grid.Col>
        </Grid>
      </form>

      <Stack gap="xs" pt="md" style={{ borderTop: "1px solid var(--mantine-color-default-border)" }}>
        {confirmed ? (
          <Group gap="xs" data-testid="profile-confirmed">
            <Badge color="teal" size="lg" circle data-testid="profile-confirmed-badge">✓</Badge>
            <Text size="sm" fw={600} c="teal">{t(locale, "profile.confirmed")}</Text>
          </Group>
        ) : (
          <>
            <Text size="sm" c="dimmed">{t(locale, "profile.confirmNote")}</Text>
            <Button variant="default" loading={busy} onClick={onConfirm} data-testid="profile-confirm" style={{ alignSelf: "flex-start" }}>
              {t(locale, "profile.confirm")}
            </Button>
          </>
        )}
      </Stack>

      {error && <Alert color="red" role="alert" data-testid="profile-status">{error}</Alert>}
      {notice && <Alert color="teal" data-testid="profile-status">{notice}</Alert>}
    </Stack>
  );
}
