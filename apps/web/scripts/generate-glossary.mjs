#!/usr/bin/env node
// Generates lib/glossary.generated.ts from content/glossary.yaml (validated by the content
// schema in CI). Run `pnpm gen:glossary`; `pnpm check:glossary` fails if the output is stale.
import { readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import * as prettier from "prettier";
import { parse } from "yaml";

const src = new URL("../../../content/glossary.yaml", import.meta.url);
const out = new URL("../lib/glossary.generated.ts", import.meta.url);
const doc = parse(readFileSync(src, "utf8"));
const terms = [...doc.terms].sort((a, b) => a.term.localeCompare(b.term, "en"));
const raw = `// Generated from content/glossary.yaml by scripts/generate-glossary.mjs. Do not edit.

export interface GlossaryTerm {
  id: string;
  term: string;
  definition: string;
  docsUrl?: string;
}

export const glossary: GlossaryTerm[] = ${JSON.stringify(terms, null, 2)};

export const glossaryById = new Map(glossary.map((t) => [t.id, t]));
`;
const outPath = fileURLToPath(out);
const body = await prettier.format(raw, { ...(await prettier.resolveConfig(outPath)), parser: "typescript" });
if (process.argv.includes("--check")) {
  if (readFileSync(out, "utf8") !== body) {
    console.error("lib/glossary.generated.ts is stale. Run: pnpm --filter @kubedojo/web gen:glossary");
    process.exit(1);
  }
  console.log("✓ glossary is up to date");
} else {
  writeFileSync(out, body);
  console.log(`Wrote ${terms.length} glossary terms`);
}
