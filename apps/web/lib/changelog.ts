/** Public changelog entries, newest first. */
export interface ChangelogEntry {
  date: string; // ISO date
  title: string;
  items: string[];
}

export const changelog: ChangelogEntry[] = [
  {
    date: "2026-10-24",
    title: "A kubectl terminal in every lab",
    items: [
      "Run kubectl get, describe, logs, events, explain, rollout status and history against your simulated cluster.",
      "Output follows real kubectl: tables, -o wide, yaml, json and name, label selectors, namespaces and kubectl's error messages.",
      "Your command history is kept with the lab, and you can download a transcript. Secret values are never kept in it.",
      "There is no shell: pipes, ;, && and redirection are refused, not run.",
    ],
  },
  {
    date: "2026-10-23",
    title: "Labs start a simulated cluster",
    items: [
      "Start a lab from a lesson to get your own simulated cluster, loaded from the lesson's starting state.",
      "See every object in the cluster with its status, and the events controllers record as it changes.",
      "Reset a lab to its starting state, or end it. Labs you don't touch for 14 days expire.",
      "The terminal arrives next; for now the lab is read-only.",
    ],
  },
  {
    date: "2026-10-23",
    title: "Your dashboard and profile",
    items: [
      "A short, skippable welcome when you first sign in.",
      "Your dashboard shows where to continue, what to try next, course progress, your streak, XP and recent activity.",
      "Public profiles, if you choose to have one. They stay private otherwise.",
      "Reduced motion now follows your account setting on every device.",
    ],
  },
  {
    date: "2026-10-23",
    title: "First lessons published",
    items: [
      "Three lessons are live: how a cluster is put together, inspecting a running Pod, and scaling a Deployment.",
      "Search and filter the curriculum, read lessons with copyable commands and key terms, and move between lessons with [ and ].",
      "A glossary of the Kubernetes terms used in lessons.",
    ],
  },
  {
    date: "2026-10-23",
    title: "Sign in with GitHub",
    items: [
      "Sign in with your GitHub account. KubeDojo uses only your public profile and never asks for your email.",
      "Choose your handle and display name, theme, time zone and privacy settings. Profiles and the leaderboard stay private unless you opt in.",
    ],
  },
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
