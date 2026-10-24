/**
 * KubeDojo design tokens: the single source of truth for color values.
 * app/globals.css declares the same values as CSS variables; tests/unit/design-tokens.test.ts
 * fails if the two drift or if any text/background pair drops below WCAG 2.2 AA contrast.
 */

export type ThemeName = "dark" | "light";

/** Semantic color roles, mapped 1:1 to CSS variables (e.g. `background` → `--background`). */
export interface ThemeColors {
  background: string;
  foreground: string;
  card: string;
  "card-foreground": string;
  popover: string;
  "popover-foreground": string;
  primary: string;
  "primary-foreground": string;
  secondary: string;
  "secondary-foreground": string;
  muted: string;
  "muted-foreground": string;
  accent: string;
  "accent-foreground": string;
  destructive: string;
  border: string;
  input: string;
  ring: string;
  link: string;
  highlight: string;
  success: string;
  warning: string;
  danger: string;
  "sim-badge": string;
  "sim-badge-foreground": string;
}

/** Brand palette from the master plan (Part 03). */
export const brand = {
  navy: "#09111F",
  surface: "#111C2E",
  kubernetesBlue: "#326CE5",
  cyan: "#22D3EE",
  green: "#22C55E",
  amber: "#FBBF24",
  red: "#F87171",
} as const;

export const themes: Record<ThemeName, ThemeColors> = {
  dark: {
    background: brand.navy,
    foreground: "#E6EDF7",
    card: brand.surface,
    "card-foreground": "#E6EDF7",
    popover: brand.surface,
    "popover-foreground": "#E6EDF7",
    primary: brand.kubernetesBlue,
    "primary-foreground": "#FFFFFF",
    secondary: "#1A2740",
    "secondary-foreground": "#E6EDF7",
    muted: "#1A2740",
    "muted-foreground": "#9AABC4",
    accent: "#1C2B47",
    "accent-foreground": "#E6EDF7",
    destructive: brand.red,
    border: "#22314D",
    input: "#5B6F92",
    ring: brand.cyan,
    link: "#8CB4FF",
    highlight: brand.cyan,
    success: brand.green,
    warning: brand.amber,
    danger: brand.red,
    "sim-badge": "#3B2F0B",
    "sim-badge-foreground": "#FCD34D",
  },
  light: {
    background: "#F7F9FC",
    foreground: "#0B1324",
    card: "#FFFFFF",
    "card-foreground": "#0B1324",
    popover: "#FFFFFF",
    "popover-foreground": "#0B1324",
    primary: brand.kubernetesBlue,
    "primary-foreground": "#FFFFFF",
    secondary: "#E9EEF6",
    "secondary-foreground": "#0B1324",
    muted: "#EEF2F8",
    "muted-foreground": "#4A5A75",
    accent: "#E3EBF9",
    "accent-foreground": "#0B1324",
    destructive: "#B91C1C",
    border: "#D5DDEA",
    input: "#7A8AA6",
    ring: "#0E7490",
    link: "#1F56C9",
    highlight: "#0E7490",
    success: "#15803D",
    warning: "#B45309",
    danger: "#B91C1C",
    "sim-badge": "#FEF3C7",
    "sim-badge-foreground": "#78350F",
  },
};

/** Text-on-background pairs that must meet WCAG AA (4.5:1 for normal text). */
export const textContrastPairs: Array<[keyof ThemeColors, keyof ThemeColors]> = [
  ["foreground", "background"],
  ["card-foreground", "card"],
  ["popover-foreground", "popover"],
  ["primary-foreground", "primary"],
  ["secondary-foreground", "secondary"],
  ["muted-foreground", "background"],
  ["muted-foreground", "card"],
  ["muted-foreground", "muted"],
  ["accent-foreground", "accent"],
  ["link", "background"],
  ["link", "card"],
  ["destructive", "background"],
  ["destructive", "card"],
  ["success", "background"],
  ["success", "card"],
  ["warning", "background"],
  ["warning", "card"],
  ["danger", "card"],
  ["highlight", "background"],
  ["sim-badge-foreground", "sim-badge"],
];

/** Non-text UI pairs that must meet 3:1 (focus indicators, input borders). */
export const uiContrastPairs: Array<[keyof ThemeColors, keyof ThemeColors]> = [
  ["ring", "background"],
  ["ring", "card"],
  ["input", "card"],
  ["primary", "background"],
];

/** Terminal palette (xterm.js theme, Part 14). Dark in both app themes, like most terminals. */
export const terminal = {
  background: "#050B16",
  foreground: "#D6E2F5",
  cursor: brand.cyan,
  selectionBackground: "#24406E",
  black: "#1B2436",
  red: "#FF7B7B",
  green: "#4ADE80",
  yellow: "#FACC15",
  blue: "#7AA7FF",
  magenta: "#D8A1FF",
  cyan: "#5EEAD4",
  white: "#D6E2F5",
  brightBlack: "#8293B0",
} as const;

/** Terminal colors that are used as text on the terminal background. */
export const terminalTextColors = [
  "foreground",
  "red",
  "green",
  "yellow",
  "blue",
  "magenta",
  "cyan",
  "white",
  "brightBlack",
] as const;

/**
 * Cluster-map node styles (React Flow, Part 15). Shape and icon distinguish kinds so color is
 * never the only signal (WCAG 1.4.1).
 */
export const graphNodes = {
  Namespace: { shape: "frame", icon: "FolderTree", color: "#64748B" },
  Node: { shape: "rounded-rect", icon: "Server", color: "#94A3B8" },
  Deployment: { shape: "rounded-rect", icon: "Layers", color: brand.kubernetesBlue },
  ReplicaSet: { shape: "rounded-rect", icon: "Copy", color: "#6366F1" },
  Pod: { shape: "circle", icon: "Box", color: brand.cyan },
  Service: { shape: "hexagon", icon: "Network", color: "#A855F7" },
  EndpointSlice: { shape: "pill", icon: "Waypoints", color: "#C084FC" },
  ConfigMap: { shape: "document", icon: "FileCog", color: "#14B8A6" },
  Secret: { shape: "document", icon: "KeyRound", color: "#F59E0B" },
} as const;

/** Status styles shared by the terminal summaries, graph and lists. Icons pair with color. */
export const statusStyles = {
  Running: { color: "success", icon: "CircleCheck" },
  Ready: { color: "success", icon: "CircleCheck" },
  Pending: { color: "warning", icon: "Clock" },
  ContainerCreating: { color: "warning", icon: "Loader" },
  CrashLoopBackOff: { color: "danger", icon: "RotateCcw" },
  ImagePullBackOff: { color: "danger", icon: "ImageOff" },
  ErrImagePull: { color: "danger", icon: "ImageOff" },
  CreateContainerConfigError: { color: "danger", icon: "FileWarning" },
  Terminating: { color: "muted-foreground", icon: "CircleSlash" },
} as const;

// WCAG relative luminance and contrast ratio.
function channel(c: number): number {
  const s = c / 255;
  return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
}

export function luminance(hex: string): number {
  const m = /^#([0-9a-f]{6})$/i.exec(hex);
  if (!m?.[1]) throw new Error(`Not a #RRGGBB color: ${hex}`);
  const n = parseInt(m[1], 16);
  return 0.2126 * channel((n >> 16) & 255) + 0.7152 * channel((n >> 8) & 255) + 0.0722 * channel(n & 255);
}

export function contrastRatio(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x) as [number, number];
  return (hi + 0.05) / (lo + 0.05);
}
