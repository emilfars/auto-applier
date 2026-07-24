import type { PortalMap } from "../types.js";
import { digitsOnly } from "../transforms.js";

/**
 * Greenhouse-hosted application forms (boards.greenhouse.io). Field ids are
 * stable and portal-specific, so confidence is high.
 */
export const greenhouse: PortalMap = {
  id: "greenhouse",
  version: "1.0.0",
  hosts: ["greenhouse.io", "boards.greenhouse.io"],
  fields: [
    { key: "first_name", selectors: ["#first_name", 'input[name="first_name"]'], confidence: 0.95 },
    { key: "last_name", selectors: ["#last_name", 'input[name="last_name"]'], confidence: 0.95 },
    { key: "email", selectors: ["#email", 'input[type="email"]'], confidence: 0.95 },
    { key: "phone", selectors: ["#phone", 'input[type="tel"]'], confidence: 0.9 },
    {
      key: "linkedin_url",
      selectors: ['input[name*="linkedin" i]', 'input[aria-label*="linkedin" i]'],
      confidence: 0.7,
    },
    {
      key: "expected_salary",
      selectors: ['input[name*="salary" i]', 'input[aria-label*="salary" i]'],
      confidence: 0.6,
      transform: digitsOnly,
    },
    {
      key: "cv_file",
      selectors: ['input[type="file"][name*="resume" i]', 'input[type="file"]'],
      confidence: 0.85,
      file: true,
    },
  ],
};
