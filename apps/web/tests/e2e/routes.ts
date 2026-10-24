/** Every page route in the app, used by the accessibility and responsive suites. */
export const marketingRoutes = ["/", "/learn", "/learn/glossary", "/about", "/faq", "/privacy", "/terms", "/changelog"];
export const appRoutes = [
  "/dashboard",
  "/paths",
  "/profile/kube-learner",
  "/settings",
  "/leaderboard",
  "/lab/00000000-0000-4000-8000-000000000000",
  "/admin",
  "/admin/content",
  "/admin/audit",
];
export const allRoutes = [...marketingRoutes, "/login", ...appRoutes, "/design-system", "/does-not-exist"];
