import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, mkdirSync, cpSync, writeFileSync, readFileSync, readdirSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createValidators, validateFile, validateTree, typeFromName } from "../src/index.js";

const examples = new URL("../examples/", import.meta.url).pathname;

test("example content is valid", () => {
  assert.deepEqual([...validateTree(examples, { partial: true })], []);
});

test("every invalid example is rejected", () => {
  const validators = createValidators();
  const dir = join(examples, "invalid");
  const files = readdirSync(dir).filter((f) => f.endsWith(".yaml"));
  assert.ok(files.length >= 7);
  for (const name of files) {
    const file = join(dir, name);
    const { errors } = validateFile(validators, typeFromName(file), file);
    assert.notEqual(errors.length, 0, `${name} should be rejected`);
  }
});

function scratchTree() {
  const root = mkdtempSync(join(tmpdir(), "kubedojo-content-"));
  for (const sub of ["challenges", "fixtures", "courses"])
    cpSync(join(examples, sub), join(root, sub), { recursive: true });
  return root;
}

test("unwritten lessons are planned by default and errors with complete", () => {
  const problems = validateTree(scratchTree());
  assert.ok(problems.planned.includes("workloads/launch-a-pod"));
  assert.ok(!problems.some((p) => p.message.includes("has no challenge file")));
  assert.ok(problems.some((p) => p.message === "course fundamentals not found"));
  const strict = validateTree(scratchTree(), { complete: true });
  assert.ok(strict.some((p) => p.message === "lesson launch-a-pod has no challenge file"));
});

test("the repository's content is valid", () => {
  const content = new URL("../../../content/", import.meta.url).pathname;
  assert.deepEqual([...validateTree(content)], []);
});

test("cross-file rules catch mismatched ids, missing fixtures and answers", () => {
  const root = scratchTree();
  const file = join(root, "challenges", "fundamentals", "cluster-architecture.yaml");
  const text = readFileSync(file, "utf8")
    .replace("id: fundamentals/cluster-architecture", "id: fundamentals/wrong-id")
    .replace("initialFixture: fixtures/three-node-cluster.yaml", "initialFixture: fixtures/missing.yaml")
    .replace("    state-store: etcd\n", "");
  writeFileSync(file, text);
  const messages = validateTree(root, { partial: true }).map((p) => p.message);
  assert.ok(messages.includes("id must be fundamentals/cluster-architecture"));
  assert.ok(messages.includes("fixture fixtures/missing.yaml not found"));
  assert.ok(messages.includes("goal state-store: solution.answers must include an answer"));
});

test("choice answers must be one of the choices", () => {
  const root = scratchTree();
  const file = join(root, "challenges", "fundamentals", "cluster-architecture.yaml");
  writeFileSync(file, readFileSync(file, "utf8").replace("static: etcd", "static: zookeeper"));
  const messages = validateTree(root, { partial: true }).map((p) => p.message);
  assert.ok(messages.includes("goal state-store: expected answer must be one of the choices"));
});

test("duplicate YAML keys are rejected", () => {
  const root = mkdtempSync(join(tmpdir(), "kubedojo-content-"));
  mkdirSync(join(root, "courses"));
  writeFileSync(
    join(root, "courses", "dup.yaml"),
    "schemaVersion: 1\nslug: dup\nslug: dup\ntitle: Dup\nsummary: Duplicate keys test.\ndescription: x\ndifficulty: beginner\ndisplayOrder: 1\nlessons: [a]\n",
  );
  const problems = validateTree(root, { partial: true });
  assert.ok(problems.some((p) => p.message.startsWith("YAML parse error")));
});
