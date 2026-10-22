/** Public changelog entries, newest first. */
export interface ChangelogEntry {
  date: string; // ISO date
  title: string;
  items: string[];
}

export const changelog: ChangelogEntry[] = [
  {
    date: "2026-10-22",
    title: "Design system and site foundation",
    items: [
      "New KubeDojo look: navy, Kubernetes blue and cyan, with dark and light themes that meet WCAG AA contrast.",
      "Public pages: home, curriculum overview, about, FAQ, privacy, terms and this changelog.",
      "A preview of the signed-in app layout. It isn't connected to accounts yet.",
      "Keyboard shortcuts (press ? to see them) and reduced-motion support.",
    ],
  },
  {
    date: "2026-10-22",
    title: "Development infrastructure",
    items: [
      "One-command local setup with Docker Compose: PostgreSQL, Redis, the Go API and the web app behind a single edge proxy.",
      "Continuous integration for linting, type checks, unit, integration and end-to-end tests, secret scanning and license checks.",
    ],
  },
  {
    date: "2026-10-22",
    title: "Specification published",
    items: [
      "Product requirements, architecture decisions, threat model and the full 30-lesson curriculum outline.",
      "A public list of exactly which kubectl commands and Kubernetes behaviors the simulator will support.",
    ],
  },
];
