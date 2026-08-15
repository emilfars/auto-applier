import type { PortalMap } from "../types.js";

/**
 * Workable-hosted application forms (apply.workable.com). Split first/last name
 * inputs with stable `name` attributes.
 */
export const workable: PortalMap = {
  id: "workable",
  version: "1.1.0",
  hosts: ["workable.com", "apply.workable.com"],
  fields: [
    { key: "first_name", selectors: ['input[name="firstname"]', "#firstname"], confidence: 0.95 },
    { key: "last_name", selectors: ['input[name="lastname"]', "#lastname"], confidence: 0.95 },
    { key: "email", selectors: ['input[name="email"]', 'input[type="email"]'], confidence: 0.95 },
    { key: "phone", selectors: ['input[name="phone"]', 'input[type="tel"]'], confidence: 0.9 },
    { key: "address", selectors: ['input[name="address"]', "#address"], confidence: 0.75 },
    {
      key: "linkedin_url",
      selectors: ['input[name*="linkedin" i]', 'input[aria-label*="linkedin" i]'],
      confidence: 0.7,
    },
    {
      key: "cv_file",
      selectors: [
        'input[type="file"][name*="resume" i]',
        'input[type="file"][name*="cv" i]',
        'input[type="file"][id*="resume" i]',
      ],
      confidence: 0.85,
      file: true,
    },
  ],
};
