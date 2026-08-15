import type { PortalMap } from "../types.js";
import { digitsOnly } from "../transforms.js";

/**
 * Jobstreet (SEEK-based, id.jobstreet.com). SEEK exposes `data-automation`
 * hooks; loose name-based fallbacks cover template variations.
 */
export const jobstreet: PortalMap = {
  id: "jobstreet",
  version: "1.1.0",
  hosts: ["jobstreet.com", "jobstreet.co.id", "id.jobstreet.com"],
  fields: [
    {
      key: "full_name",
      selectors: ['[data-automation="name-input"]', 'input[name="name"]', "#name"],
      confidence: 0.85,
    },
    {
      key: "email",
      selectors: ['[data-automation="email-input"]', 'input[type="email"]', 'input[name="email"]'],
      confidence: 0.85,
    },
    {
      key: "phone",
      selectors: ['[data-automation="phone-input"]', 'input[type="tel"]', 'input[name*="phone" i]'],
      confidence: 0.8,
    },
    {
      key: "expected_salary",
      selectors: ['[data-automation="salary-input"]', 'input[name*="salary" i]'],
      confidence: 0.65,
      transform: digitsOnly,
    },
    {
      key: "cv_file",
      selectors: [
        '[data-automation="resume-upload"] input[type="file"]',
        'input[type="file"][name*="resume" i]',
        'input[type="file"][name*="cv" i]',
      ],
      confidence: 0.8,
      file: true,
    },
  ],
};
