import type { PortalMap } from "../types.js";
import { digitsOnly } from "../transforms.js";

/**
 * Glints (glints.com) application forms. React app with `name`/aria-based
 * inputs; moderate confidence with looser fallbacks.
 */
export const glints: PortalMap = {
  id: "glints",
  version: "1.1.0",
  hosts: ["glints.com", "glints.co"],
  fields: [
    { key: "full_name", selectors: ['input[name="name"]', 'input[name="fullName"]', 'input[aria-label*="name" i]'], confidence: 0.8 },
    { key: "email", selectors: ['input[name="email"]', 'input[type="email"]'], confidence: 0.85 },
    { key: "phone", selectors: ['input[name="phoneNumber"]', 'input[type="tel"]', 'input[name*="phone" i]'], confidence: 0.8 },
    { key: "current_title", selectors: ['input[name="jobTitle"]', 'input[name*="title" i]'], confidence: 0.65 },
    {
      key: "expected_salary",
      selectors: ['input[name="expectedSalary"]', 'input[name*="salary" i]'],
      confidence: 0.65,
      transform: digitsOnly,
    },
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
