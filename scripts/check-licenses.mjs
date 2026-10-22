#!/usr/bin/env node
// Fails if any production JavaScript dependency uses a license outside the allowlist.
// Go and npm dependencies are also checked on pull requests by GitHub's dependency review.
import { execFileSync } from "node:child_process";

const ALLOWED = new Set([
  "MIT",
  "ISC",
  "Apache-2.0",
  "BSD-2-Clause",
  "BSD-3-Clause",
  "0BSD",
  "BlueOak-1.0.0",
  "CC0-1.0",
  "Unlicense",
  "Python-2.0",
  // caniuse-lite data (attribution only).
  "CC-BY-4.0",
]);

// Packages allowed despite their license, with the reason. Review when adding.
const EXCEPTIONS = new Map([
  // libvips binaries used by Next.js image optimization through sharp; dynamically linked, unmodified.
  [/^@img\/sharp-libvips-/, "LGPL-3.0-or-later"],
]);

/** Evaluates simple SPDX expressions: "A OR B" passes if any part passes, "A AND B" if all do. */
function allowed(expression) {
  const expr = expression.trim().replace(/^\((.*)\)$/, "$1");
  if (/ OR /i.test(expr)) return expr.split(/ OR /i).some(allowed);
  if (/ AND /i.test(expr)) return expr.split(/ AND /i).every(allowed);
  return ALLOWED.has(expr);
}

const report = JSON.parse(execFileSync("pnpm", ["licenses", "list", "--json", "--prod"], { encoding: "utf8" }));
const problems = [];
for (const [license, pkgs] of Object.entries(report)) {
  if (allowed(license)) continue;
  for (const pkg of pkgs) {
    const excepted = [...EXCEPTIONS].some(([pattern, lic]) => pattern.test(pkg.name) && lic === license);
    if (!excepted) problems.push(`${pkg.name}@${pkg.versions?.join(",") ?? "?"}: ${license}`);
  }
}

if (problems.length) {
  console.error("Disallowed licenses in production dependencies:\n  " + problems.join("\n  "));
  process.exit(1);
}
const total = Object.values(report).reduce((n, pkgs) => n + pkgs.length, 0);
console.log(`✓ ${total} production packages use allowed licenses`);
