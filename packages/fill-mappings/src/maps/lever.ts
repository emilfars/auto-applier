import type { PortalMap } from "../types.js";

/**
 * Lever-hosted application forms (jobs.lever.co). Inputs use stable `name`
 * attributes, including the `urls[...]` pattern for profile links.
 */
export const lever: PortalMap = {
  id: "lever",
  version: "1.1.0",
  hosts: ["lever.co", "jobs.lever.co"],
  fields: [
    { key: "full_name", selectors: ['input[name="name"]'], confidence: 0.95 },
    { key: "email", selectors: ['input[name="email"]', 'input[type="email"]'], confidence: 0.95 },
    { key: "phone", selectors: ['input[name="phone"]', 'input[type="tel"]'], confidence: 0.9 },
    { key: "current_company", selectors: ['input[name="org"]'], confidence: 0.85 },
    { key: "linkedin_url", selectors: ['input[name="urls[LinkedIn]"]', 'input[name*="linkedin" i]'], confidence: 0.85 },
    { key: "github_url", selectors: ['input[name="urls[GitHub]"]', 'input[name*="github" i]'], confidence: 0.85 },
    { key: "portfolio_url", selectors: ['input[name="urls[Portfolio]"]', 'input[name*="portfolio" i]'], confidence: 0.8 },
    {
      key: "cv_file",
      selectors: [
        'input[name="resume"][type="file"]',
        'input[name*="cv" i][type="file"]',
        'input[id*="resume" i][type="file"]',
      ],
      confidence: 0.85,
      file: true,
    },
  ],
};
