#!/usr/bin/env node
// Fails if server secrets (by name or by value) or server-only modules appear in the browser
// bundles under .next/static (threat T4). Run after `pnpm build`.
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

const root = new URL("../.next/static", import.meta.url).pathname;

const forbiddenNames = [
  "BFF_SIGNING_PRIVATE_KEY",
  "AUTH_SECRET",
  "AUTH_GITHUB_SECRET",
  "API_JWT_PUBLIC_KEYS",
  "API_INTERNAL_URL",
  "kubedojo-api-provision",
  "BEGIN PRIVATE KEY",
];
// Actual secret values, when present in the environment (CI and local builds).
const forbiddenValues = ["BFF_SIGNING_PRIVATE_KEY", "AUTH_SECRET", "AUTH_GITHUB_SECRET"]
  .map((k) => process.env[k])
  .filter((v) => v && v.length >= 16);

function* files(dir) {
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) yield* files(full);
    else if (/\.(js|mjs|json|html|css)$/.test(entry)) yield full;
  }
}

let scanned = 0;
const problems = [];
for (const file of files(root)) {
  const text = readFileSync(file, "utf8");
  scanned++;
  for (const name of forbiddenNames) if (text.includes(name)) problems.push(`${file}: contains "${name}"`);
  for (const value of forbiddenValues) if (text.includes(value)) problems.push(`${file}: contains a secret value`);
}
if (scanned === 0) {
  console.error("No client bundle found. Run `pnpm build` first.");
  process.exit(1);
}
if (problems.length) {
  console.error("Server secrets leaked into client bundles:\n  " + problems.join("\n  "));
  process.exit(1);
}
console.log(`✓ ${scanned} client files contain no server secrets`);
