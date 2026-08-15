import type { PortalMap } from "../types.js";
import { digitsOnly } from "../transforms.js";

/**
 * Generic career-form fallback for unrecognized ATS pages. Selectors are broad
 * (autocomplete/name/type heuristics) so base confidence is deliberately low —
 * most matches land as `uncertain` and require user review before commit.
 */
export const generic: PortalMap = {
  id: "generic",
  version: "1.2.0",
  hosts: [],
  fields: [
    {
      key: "full_name",
      selectors: ['input[autocomplete="name"]', 'input[name="name"]', "#name", "#full_name", 'input[name*="fullname" i]'],
      confidence: 0.59,
    },
    {
      key: "email",
      selectors: ['input[type="email"]', 'input[autocomplete="email"]', 'input[name="email"]', 'input[name*="email" i]'],
      confidence: 0.59,
    },
    {
      key: "phone",
      selectors: ['input[type="tel"]', 'input[autocomplete="tel"]', 'input[name*="phone" i]'],
      confidence: 0.59,
    },
    {
      key: "city",
      selectors: ['input[autocomplete="address-level2"]', 'input[name*="city" i]'],
      confidence: 0.59,
    },
    {
      key: "linkedin_url",
      selectors: ['input[name*="linkedin" i]', 'input[placeholder*="linkedin" i]'],
      confidence: 0.59,
    },
    {
      key: "expected_salary",
      selectors: ['input[name*="salary" i]', 'input[placeholder*="salary" i]'],
      confidence: 0.59,
      transform: digitsOnly,
    },
    {
      key: "cv_file",
      selectors: [
        'input[type="file"][name*="resume" i]',
        'input[type="file"][name*="cv" i]',
        'input[type="file"][id*="resume" i]',
      ],
      confidence: 0.59,
      file: true,
    },
  ],
};
