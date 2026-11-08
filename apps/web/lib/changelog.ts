/** Public changelog entries, newest first. */
export interface ChangelogEntry {
  date: string; // ISO date
  title: string;
  items: string[];
}

export const changelog: ChangelogEntry[] = [
  {
    date: "2026-11-08",
    title: "A map of your cluster",
    items: [
      "The Cluster tab draws your lab as a map: which Deployment owns which ReplicaSet and Pods, which Pods each Service sends traffic to, where Pods run and which ConfigMaps and Secrets they read.",
      "Everything your last command changed is highlighted, so you can watch a rollout or a scale-up happen.",
      "Pods a Service selects but can't send traffic to are faded. Every relationship is also listed as text, and the list view is one click away.",
    ],
  },
  {
    date: "2026-11-08",
    title: "Complete labs and earn XP",
    items: [
      "Check my work grades every goal against your cluster and tells you which still need work, without giving the answer away.",
      "Pass every goal to complete the lab and earn the lesson's XP. Your first check passing earns a bonus; each hint you reveal costs a little.",
      "Questions in the goals panel take your answer, and hints appear one at a time when you ask.",
      "Completing lessons builds your streak and unlocks achievements on your dashboard.",
    ],
  },
  {
    date: "2026-11-08",
    title: "Write manifests in the lab editor",
    items: [
      "Every lab has a YAML editor. Lessons that start from a manifest put it there for you.",
      "kubectl apply -f, create -f and delete -f read the files you save, and Apply in the editor saves and applies in one step.",
      "apply works like kubectl's: it remembers what you last applied and merges your changes with what the controllers and your other commands did.",
      "YAML mistakes are underlined as you type, and the cluster explains anything it can't accept when you apply.",
    ],
  },
  {
    date: "2026-11-08",
    title: "Change your cluster from the terminal",
    items: [
      "kubectl create, run, expose, scale, set image, set env, label, annotate, delete, rollout undo and rollout restart now work in every lab.",
      "After each change the simulated controllers react before the command returns: scale a Deployment and its ReplicaSet creates the Pods; relabel a Pod and its ReplicaSet lets it go and starts a replacement.",
      "Each change moves the lab's simulated clock forward 10 seconds, so AGE, events and restart counts move with you.",
      "Try a change first with --dry-run=client -o yaml. A command that fails changes nothing.",
      "Change your default namespace with kubectl config set-context --current --namespace=<name>.",
    ],
  },
  {
    date: "2026-10-27",
    title: "Sign in with Google, GitHub or email",
    items: [
      "Sign in with your Google or GitHub account, or with an email and password.",
      "Sign-in is handled by Asgardeo, WSO2's identity service. KubeDojo still never receives your email address.",
      "Accounts created with the earlier GitHub-only sign-in don't carry over; signing in again starts a new account.",
    ],
  },
  {
    date: "2026-10-25",
    title: "Every lesson is written",
    items: [
      "All 30 lessons across the six belts are open, from namespaces and labels to a capstone that brings a broken shop back online.",
      "Each lab starts from its own simulated cluster. The troubleshooting labs start broken, the way you'd meet the problem at work.",
      "Ten new glossary terms, including annotation, EndpointSlice, NodePort and CrashLoopBackOff.",
      "Labs that change the cluster need kubectl create, apply, scale and friends, which are coming to the terminal next. Until then you can read every lesson and explore its cluster.",
    ],
  },
  {
    date: "2026-10-24",
    title: "One header and footer everywhere",
    items: [
      "KubeDojo now shares its look with GitDojo: the same header, footer, colors and type.",
      "Every page, including your dashboard and settings, has the same header with search, the theme switch and your account menu.",
      "The home page walks through how a lesson goes, the six belts, what the simulator does, and how belts and XP work.",
    ],
  },
  {
    date: "2026-10-24",
    title: "A new look, search, and your data",
    items: [
      "KubeDojo has its own logo and look: every course is now a belt, from white to black.",
      "The curriculum page shows all six courses and every lesson, with the ones you can open today linked.",
      "Search lessons, glossary terms and pages from anywhere in the app with Ctrl+K (⌘K on a Mac).",
      "Labs show their goals next to the terminal, and your dashboard lists labs you can pick up again.",
      "The leaderboard is live for learners who opt in, in settings.",
      "Settings now let you download everything we store about you, or delete your account.",
    ],
  },
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
