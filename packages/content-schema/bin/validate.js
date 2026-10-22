#!/usr/bin/env node
// Validates KubeDojo content against the JSON Schemas plus cross-file rules.
//
//   kubedojo-content-validate <content-dir> [--partial]
//   kubedojo-content-validate --expect-invalid <dir>   (every file must fail; type from name prefix)
import { readdirSync } from "node:fs";
import { join } from "node:path";
import { createValidators, validateFile, validateTree, typeFromName } from "../src/index.js";

const args = process.argv.slice(2);

if (args[0] === "--expect-invalid") {
  const dir = args[1];
  if (!dir) {
    console.error("usage: kubedojo-content-validate --expect-invalid <dir>");
    process.exit(2);
  }
  const validators = createValidators();
  let failures = 0;
  const files = readdirSync(dir)
    .filter((f) => f.endsWith(".yaml"))
    .sort();
  for (const name of files) {
    const file = join(dir, name);
    const type = typeFromName(file);
    if (!type) {
      console.error(`✗ ${file}: name must start with course-, challenge- or fixture-`);
      failures++;
      continue;
    }
    const { errors } = validateFile(validators, type, file);
    if (errors.length === 0) {
      console.error(`✗ ${file}: expected validation to fail, but it passed`);
      failures++;
    } else {
      console.log(`✓ ${file} rejected (${errors[0]})`);
    }
  }
  if (files.length === 0) {
    console.error(`✗ no .yaml files in ${dir}`);
    process.exit(1);
  }
  process.exit(failures ? 1 : 0);
}

const root = args.find((a) => !a.startsWith("--"));
if (!root) {
  console.error("usage: kubedojo-content-validate <content-dir> [--partial]");
  process.exit(2);
}
const problems = validateTree(root, { partial: args.includes("--partial") });
for (const p of problems) console.error(`✗ ${p.file}: ${p.message}`);
if (problems.length) {
  console.error(`\n${problems.length} problem(s) found.`);
  process.exit(1);
}
console.log(`✓ ${root}: all content valid`);
