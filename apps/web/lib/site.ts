/** Site-wide constants and navigation. */
export const site = {
  name: "KubeDojo",
  tagline: "Master Kubernetes, one command at a time.",
  description:
    "Learn Kubernetes by doing: run kubectl against a safe simulated cluster, see what changed, and get instant feedback on every challenge. Free and open source.",
  repoUrl: "https://github.com/kubedojolearn/kubedojo",
  compatibilityUrl: "https://github.com/kubedojolearn/kubedojo/blob/main/docs/architecture/simulator-compatibility.md",
  securityUrl: "https://github.com/kubedojolearn/kubedojo/security/advisories/new",
  kubernetesVersion: "1.37",
} as const;

export interface NavItem {
  href: string;
  label: string;
}

export const marketingNav: NavItem[] = [
  { href: "/learn", label: "Curriculum" },
  { href: "/about", label: "About" },
  { href: "/faq", label: "FAQ" },
  { href: "/changelog", label: "Changelog" },
];

export const footerNav: Array<{ title: string; items: NavItem[] }> = [
  {
    title: "Learn",
    items: [
      { href: "/learn", label: "Curriculum" },
      { href: "/faq", label: "FAQ" },
      { href: "/changelog", label: "Changelog" },
    ],
  },
  {
    title: "Project",
    items: [
      { href: "/about", label: "About" },
      { href: site.repoUrl, label: "Source code" },
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

export const appNav: Array<{ title?: string; items: Array<NavItem & { icon: string }> }> = [
  {
    items: [
      { href: "/dashboard", label: "Dashboard", icon: "LayoutDashboard" },
      { href: "/learn", label: "Curriculum", icon: "BookOpen" },
      { href: "/paths", label: "Learning path", icon: "Route" },
      { href: "/leaderboard", label: "Leaderboard", icon: "Trophy" },
      { href: "/settings", label: "Settings", icon: "Settings" },
    ],
  },
  {
    title: "Admin",
    items: [
      { href: "/admin", label: "Overview", icon: "Shield" },
      { href: "/admin/content", label: "Content", icon: "FileStack" },
      { href: "/admin/audit", label: "Audit log", icon: "ScrollText" },
    ],
  },
];
