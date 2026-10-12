import { Component, type ReactNode } from "react";
import { Button, Container, Stack, Text, Title } from "@mantine/core";
import { DEFAULT_LOCALE, t } from "./i18n";

/**
 * Last line of defense: a render error shows a way out instead of a blank page.
 * It sits above the language switch, so it speaks the default locale. Nothing
 * about the error is logged or shown, since it may carry profile data.
 */
export class ErrorBoundary extends Component<{ children: ReactNode }, { failed: boolean }> {
  state = { failed: false };

  static getDerivedStateFromError() {
    return { failed: true };
  }

  render() {
    if (!this.state.failed) return this.props.children;
    return (
      <Container size="xs" py="xl" role="alert">
        <Stack gap="sm" align="flex-start">
          <Title order={1} size="h3">
            {t(DEFAULT_LOCALE, "error.boundary.title")}
          </Title>
          <Text c="dimmed">{t(DEFAULT_LOCALE, "error.boundary.body")}</Text>
          <Button onClick={() => window.location.reload()}>{t(DEFAULT_LOCALE, "error.boundary.reload")}</Button>
        </Stack>
      </Container>
    );
  }
}
