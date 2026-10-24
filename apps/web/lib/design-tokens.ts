/**
 * KubeDojo design tokens: the single source of truth for color values.
 * app/globals.css declares the same values as CSS variables; tests/unit/design-tokens.test.ts
 * fails if the two drift or if any text/background pair drops below WCAG 2.2 AA contrast.
 *
 * The palette is shared with GitDojo, KubeDojo's sister project: deep neutral surfaces, thin
 * borders and one restrained blue accent, so the two products read as one family.
 */

export type ThemeName = "dark" | "light";

/** Semantic color roles, mapped 1:1 to CSS variables (e.g. `background` → `--background`). */
export interface ThemeColors {
  background: string;
  foreground: string;
  /** Body copy that sits under a heading: one step quieter than `foreground`. */
  "foreground-secondary": string;
  /** Banded page sections and quiet wells. */
  surface: string;
  card: string;
  "card-foreground": string;
  /** Raised controls: chips, keycaps, popovers. */
  elevated: string;
  /** Hover fill for rows, nav links and ghost buttons. */
  hover: string;
  popover: string;
  "popover-foreground": string;
  primary: string;
  "primary-foreground": string;
  secondary: string;
  "secondary-foreground": string;
  muted: string;
  "muted-foreground": string;
  /** Non-text only (icons, markers, rules): 3:1 on every surface. Use muted-foreground for text. */
  faint: string;
  accent: string;
  "accent-foreground": string;
  destructive: string;
  border: string;
  "border-subtle": string;
  "border-strong": string;
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

/** Brand palette, sampled from the logo artwork (brand/kubedojo-logo-source.png). */
export const brand = {
  ink: "#040D1D",
  blue: "#0165F6",
  brightBlue: "#0381FD",
  cyan: "#22E2F9",
} as const;

export const themes: Record<ThemeName, ThemeColors> = {
  dark: {
    background: "#0B0D10",
    foreground: "#F4F7FB",
    "foreground-secondary": "#AEB6C2",
    surface: "#111419",
    card: "#151920",
    "card-foreground": "#F4F7FB",
    elevated: "#1A1F27",
    hover: "#202630",
    popover: "#1A1F27",
    "popover-foreground": "#F4F7FB",
    // Filled accent under white text: deep enough for 4.5:1.
    primary: "#4A67E0",
    "primary-foreground": "#FFFFFF",
    secondary: "#1A1F27",
    "secondary-foreground": "#F4F7FB",
    muted: "#111419",
    "muted-foreground": "#8A93A1",
    faint: "#6F7886",
    accent: "#202630",
    "accent-foreground": "#F4F7FB",
    destructive: "#F06A78",
    border: "#262C35",
    "border-subtle": "#1D222A",
    "border-strong": "#343C48",
    input: "#6F7886",
    ring: "#6C8CFF",
    link: "#6C8CFF",
    highlight: "#6C8CFF",
    success: "#4FD18B",
    warning: "#F5C451",
    danger: "#F06A78",
    "sim-badge": "#2B2414",
    "sim-badge-foreground": "#F5C451",
  },
  light: {
    background: "#F4F6F9",
    foreground: "#111827",
    "foreground-secondary": "#3D4756",
    surface: "#F9FAFB",
    card: "#FFFFFF",
    "card-foreground": "#111827",
    elevated: "#EEF1F5",
    hover: "#E5E9EF",
    popover: "#FFFFFF",
    "popover-foreground": "#111827",
    primary: "#3A5AD6",
    "primary-foreground": "#FFFFFF",
    secondary: "#EEF1F5",
    "secondary-foreground": "#111827",
    muted: "#F9FAFB",
    "muted-foreground": "#556070",
    faint: "#78828F",
    accent: "#E5E9EF",
    "accent-foreground": "#111827",
    destructive: "#C0263A",
    border: "#D6DCE4",
    "border-subtle": "#E4E8EE",
    "border-strong": "#C2C9D3",
    input: "#78828F",
    ring: "#3A5AD6",
    link: "#3A5AD6",
    highlight: "#3A5AD6",
    success: "#11703A",
    warning: "#8A5A00",
    danger: "#C0263A",
    "sim-badge": "#FBF1D9",
    "sim-badge-foreground": "#6B4500",
  },
};

/** Text-on-background pairs that must meet WCAG AA (4.5:1 for normal text). */
export const textContrastPairs: Array<[keyof ThemeColors, keyof ThemeColors]> = [
  ["foreground", "background"],
  ["foreground-secondary", "background"],
  ["foreground-secondary", "surface"],
  ["foreground-secondary", "card"],
  ["card-foreground", "card"],
  ["popover-foreground", "popover"],
  ["primary-foreground", "primary"],
  ["secondary-foreground", "secondary"],
  ["muted-foreground", "background"],
  ["muted-foreground", "surface"],
  ["muted-foreground", "card"],
  ["muted-foreground", "muted"],
  ["muted-foreground", "elevated"],
  ["muted-foreground", "hover"],
  ["accent-foreground", "accent"],
  ["link", "background"],
  ["link", "surface"],
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

/** Non-text UI pairs that must meet 3:1 (focus indicators, input borders, icons). */
export const uiContrastPairs: Array<[keyof ThemeColors, keyof ThemeColors]> = [
  ["ring", "background"],
  ["ring", "card"],
  ["input", "card"],
  ["faint", "background"],
  ["faint", "card"],
];

/** Terminal palette (xterm.js theme, Part 14). Dark in both app themes, like most terminals. */
export const terminal = {
  background: "#080A0D",
  foreground: "#D6DCE5",
  cursor: "#6C8CFF",
  selectionBackground: "#26365F",
  black: "#1B2028",
  red: "#F06A78",
  green: "#4FD18B",
  yellow: "#F5C451",
  blue: "#7C9AFF",
  magenta: "#C4A5FF",
  cyan: "#5EEAD4",
  white: "#D6DCE5",
  brightBlack: "#8A93A1",
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
  Deployment: { shape: "rounded-rect", icon: "Layers", color: brand.brightBlue },
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
