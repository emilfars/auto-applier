import type { PortalMap } from "../types.js";

/**
 * Kalibrr (kalibrr.com) application forms. Stable snake_case ids for the core
 * contact fields.
 */
export const kalibrr: PortalMap = {
  id: "kalibrr",
  version: "1.1.0",
  hosts: ["kalibrr.com", "kalibrr.id"],
  fields: [
    { key: "first_name", selectors: ["#first_name", 'input[name="first_name"]'], confidence: 0.9 },
    { key: "last_name", selectors: ["#last_name", 'input[name="last_name"]'], confidence: 0.9 },
    { key: "email", selectors: ["#email", 'input[type="email"]'], confidence: 0.9 },
    { key: "phone", selectors: ["#phone_number", 'input[name="phone_number"]', 'input[type="tel"]'], confidence: 0.85 },
    { key: "linkedin_url", selectors: ['input[name*="linkedin" i]'], confidence: 0.7 },
    {
      key: "cv_file",
      selectors: [
        'input[type="file"][name*="resume" i]',
        'input[type="file"][name*="cv" i]',
        'input[type="file"][id*="resume" i]',
      ],
      confidence: 0.8,
      file: true,
    },
  ],
};
