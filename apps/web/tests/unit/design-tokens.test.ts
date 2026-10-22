import { readFileSync } from "node:fs";
import { join } from "node:path";
import {
  contrastRatio,
  terminal,
  terminalTextColors,
  textContrastPairs,
  themes,
  uiContrastPairs,
  type ThemeName,
} from "@/lib/design-tokens";

// Vitest runs from apps/web (jsdom gives import.meta.url an http: scheme).
const css = readFileSync(join(process.cwd(), "app/globals.css"), "utf8");

function cssBlock(selector: string): Record<string, string> {
  const start = css.indexOf(`${selector} {`);
  if (start < 0) throw new Error(`no ${selector} block in globals.css`);
  const body = css.slice(start, css.indexOf("}", start));
  const vars: Record<string, string> = {};
  for (const m of body.matchAll(/--([a-z0-9-]+):\s*(#[0-9a-f]{6})\s*;/gi)) vars[m[1]!] = m[2]!.toLowerCase();
  return vars;
}

const blocks: Record<ThemeName, Record<string, string>> = { light: cssBlock(":root"), dark: cssBlock(".dark") };

describe.each(["dark", "light"] as const)("%s theme", (theme) => {
  const colors = themes[theme];

  it("globals.css matches lib/design-tokens.ts", () => {
    for (const [name, value] of Object.entries(colors)) {
      expect(blocks[theme][name], `--${name}`).toBe(value.toLowerCase());
    }
  });

  it.each(textContrastPairs)("%s on %s meets 4.5:1", (fg, bg) => {
    expect(contrastRatio(colors[fg], colors[bg])).toBeGreaterThanOrEqual(4.5);
  });

  it.each(uiContrastPairs)("%s against %s meets 3:1", (fg, bg) => {
    expect(contrastRatio(colors[fg], colors[bg])).toBeGreaterThanOrEqual(3);
  });
});

describe("terminal palette", () => {
  it.each(terminalTextColors)("%s text meets 4.5:1 on the terminal background", (name) => {
    expect(contrastRatio(terminal[name], terminal.background)).toBeGreaterThanOrEqual(4.5);
  });
});

describe("contrastRatio", () => {
  it("matches known WCAG values", () => {
    expect(contrastRatio("#000000", "#FFFFFF")).toBeCloseTo(21, 5);
    expect(contrastRatio("#326CE5", "#FFFFFF")).toBeCloseTo(4.76, 2);
  });
});
