import { Box, Group, Stack, Text, Title } from "@mantine/core";
import { Illustration, type IllustrationName } from "./Illustration";

export function PageHeader({
  title,
  description,
  illustration,
  overlap = false,
}: {
  title: string;
  description?: string;
  illustration?: IllustrationName;
  /** Leaves room for the next block to overlap the hero's bottom edge. */
  overlap?: boolean;
}) {
  return (
    <Box
      className="page-hero"
      px={{ base: "lg", sm: 40 }}
      pt={{ base: "lg", sm: 36 }}
      pb={overlap ? 64 : { base: "lg", sm: 36 }}
      mb={overlap ? 0 : "xl"}
    >
      <Group justify="space-between" align="center" wrap="nowrap" gap="xl">
        <Stack gap="sm" maw={640}>
          <Title order={1} style={{ letterSpacing: "-0.03em", fontSize: "clamp(1.6rem, 1.2rem + 1.6vw, 2.4rem)" }}>
            {title}
          </Title>
          {description && (
            <Text c="dimmed" size="lg" style={{ lineHeight: 1.5 }}>
              {description}
            </Text>
          )}
        </Stack>
        {illustration && (
          <Box visibleFrom="sm" style={{ flex: "none" }}>
            <Illustration name={illustration} width={220} />
          </Box>
        )}
      </Group>
    </Box>
  );
}
