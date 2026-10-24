/** Site-wide constants and navigation. */
export const site = {
  name: "KubeDojo",
  tagline: "Master Kubernetes, one command at a time.",
  description:
    "Learn Kubernetes by doing: run kubectl against a safe simulated cluster, see what changed, and get instant feedback on every challenge. Free and open source.",
  repoUrl: "https://github.com/kubedojolearn/kubedojo",
  docsUrl: "https://github.com/kubedojolearn/kubedojo/tree/main/docs",
  compatibilityUrl: "https://github.com/kubedojolearn/kubedojo/blob/main/docs/architecture/simulator-compatibility.md",
  securityUrl: "https://github.com/kubedojolearn/kubedojo/security/advisories/new",
  kubernetesVersion: "1.37",
} as const;

export interface NavItem {
  href: string;
  label: string;
}

/** The header's links, on every page (ordered like GitDojo's: learning first, docs last). About and FAQ live in the footer. */
export const mainNav: NavItem[] = [
  { href: "/learn", label: "Learn" },
  { href: "/paths", label: "Learning path" },
  { href: "/leaderboard", label: "Leaderboard" },
  { href: "/dashboard", label: "Dashboard" },
  { href: "/changelog", label: "Changelog" },
  { href: site.docsUrl, label: "Docs" },
];

export const footerNav: Array<{ title: string; items: NavItem[] }> = [
  {
    title: "Learn",
    items: [
      { href: "/learn", label: "Curriculum" },
      { href: "/paths", label: "Learning path" },
      { href: "/learn/glossary", label: "Glossary" },
      { href: "/leaderboard", label: "Leaderboard" },
      { href: "/dashboard", label: "Your dashboard" },
    ],
  },
  {
    title: "Project",
    items: [
      { href: "/about", label: "About" },
      { href: "/faq", label: "FAQ" },
      { href: "/changelog", label: "Changelog" },
      { href: site.docsUrl, label: "Documentation" },
      { href: site.repoUrl, label: "GitHub" },
      { href: site.compatibilityUrl, label: "Simulator coverage" },
    ],
  },
  {
    title: "Legal",
    items: [
      { href: "/privacy", label: "Privacy" },
      { href: "/terms", label: "Terms" },
      { href: site.securityUrl, label: "Report a vulnerability" },
    ],
  },
];

/** Admin pages, listed in the account menu for admins only. The API enforces the role regardless. */
export const adminNav: NavItem[] = [
  { href: "/admin", label: "Admin overview" },
  { href: "/admin/content", label: "Content" },
  { href: "/admin/audit", label: "Audit log" },
];
