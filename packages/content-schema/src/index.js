import { readFileSync, readdirSync, statSync, existsSync } from "node:fs";
import { join, relative, basename, dirname, sep } from "node:path";
import { fileURLToPath } from "node:url";
import Ajv2020 from "ajv/dist/2020.js";
import addFormats from "ajv-formats";
import { parse as parseYaml } from "yaml";

const schemaDir = join(dirname(fileURLToPath(import.meta.url)), "..", "schemas");

const CLUSTER_SCOPED = new Set(["Node", "Namespace"]);

function loadSchema(name) {
  return JSON.parse(readFileSync(join(schemaDir, name), "utf8"));
}

/** Builds validators for each content type. */
export function createValidators() {
  const ajv = new Ajv2020({
    allErrors: true,
    strict: true,
    strictTypes: false,
    strictRequired: false,
    discriminator: true,
  });
  addFormats(ajv);
  ajv.addSchema(loadSchema("common.schema.json"));
  return {
    course: ajv.compile(loadSchema("course.schema.json")),
    challenge: ajv.compile(loadSchema("challenge.schema.json")),
    fixture: ajv.compile(loadSchema("fixture.schema.json")),
  };
}

function walkYaml(dir) {
  if (!existsSync(dir)) return [];
  const out = [];
  for (const entry of readdirSync(dir).sort()) {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) out.push(...walkYaml(full));
    else if (entry.endsWith(".yaml") || entry.endsWith(".yml")) out.push(full);
  }
  return out;
}

function readYaml(file) {
  // uniqueKeys rejects duplicate keys, which YAML otherwise silently overwrites.
  return parseYaml(readFileSync(file, "utf8"), { uniqueKeys: true, prettyErrors: true });
}

function formatAjvErrors(errors) {
  return (errors ?? []).map(
    (e) => `${e.instancePath || "/"} ${e.message}${e.params ? " " + JSON.stringify(e.params) : ""}`,
  );
}

/**
 * Validates one file against the schema for `type`.
 * Returns { doc, errors } where errors is a list of strings.
 */
export function validateFile(validators, type, file) {
  let doc;
  try {
    doc = readYaml(file);
  } catch (err) {
    return { doc: undefined, errors: [`YAML parse error: ${err.message}`] };
  }
  const validate = validators[type];
  if (validate(doc)) return { doc, errors: [] };
  return { doc, errors: formatAjvErrors(validate.errors) };
}

function targets(goal) {
  const list = [];
  if (goal.target) list.push(goal.target);
  if (goal.expected?.path?.target) list.push(goal.expected.path.target);
  return list;
}

/**
 * Validates a content tree: <root>/courses, <root>/challenges/<course>/<lesson>.yaml, <root>/fixtures.
 * With partial=true, courses may list lessons whose challenge files are absent and challenges may
 * belong to courses that are absent (used for the examples directory).
 */
export function validateTree(root, { partial = false } = {}) {
  const validators = createValidators();
  const problems = [];
  const report = (file, message) => problems.push({ file: relative(process.cwd(), file), message });

  const courses = new Map();
  for (const file of walkYaml(join(root, "courses"))) {
    const { doc, errors } = validateFile(validators, "course", file);
    errors.forEach((m) => report(file, m));
    if (!doc || errors.length) continue;
    if (basename(file, ".yaml") !== doc.slug) report(file, `file name must be ${doc.slug}.yaml`);
    if (courses.has(doc.slug)) report(file, `duplicate course slug ${doc.slug}`);
    courses.set(doc.slug, doc);
  }

  const fixtures = new Set();
  for (const file of walkYaml(join(root, "fixtures"))) {
    const { doc, errors } = validateFile(validators, "fixture", file);
    errors.forEach((m) => report(file, m));
    if (!doc || errors.length) continue;
    fixtures.add(relative(root, file).split(sep).join("/"));
    const seen = new Set();
    for (const r of doc.resources) {
      const key = `${r.kind}/${r.metadata.namespace ?? ""}/${r.metadata.name}`;
      if (seen.has(key)) report(file, `duplicate resource ${key}`);
      seen.add(key);
      if (!CLUSTER_SCOPED.has(r.kind) && !r.metadata.namespace)
        report(file, `${r.kind} ${r.metadata.name} needs metadata.namespace`);
      if (CLUSTER_SCOPED.has(r.kind) && r.metadata.namespace)
        report(file, `${r.kind} ${r.metadata.name} is cluster-scoped`);
    }
  }

  const lessonIds = new Set();
  const challenges = [];
  for (const file of walkYaml(join(root, "challenges"))) {
    const { doc, errors } = validateFile(validators, "challenge", file);
    errors.forEach((m) => report(file, m));
    if (!doc || errors.length) continue;
    challenges.push({ file, doc });
    lessonIds.add(doc.id);
  }

  for (const { file, doc } of challenges) {
    const expectedId = `${doc.course}/${doc.lesson}`;
    if (doc.id !== expectedId) report(file, `id must be ${expectedId}`);
    const rel = relative(join(root, "challenges"), file).split(sep).join("/");
    if (rel !== `${doc.course}/${doc.lesson}.yaml`)
      report(file, `file must be challenges/${doc.course}/${doc.lesson}.yaml`);

    const course = courses.get(doc.course);
    if (course && !course.lessons.includes(doc.lesson))
      report(file, `course ${doc.course} does not list lesson ${doc.lesson}`);
    if (!course && !partial) report(file, `course ${doc.course} not found`);

    if (!fixtures.has(doc.initialFixture)) report(file, `fixture ${doc.initialFixture} not found`);

    for (const p of doc.prerequisites ?? []) {
      if (!lessonIds.has(p) && !partial) report(file, `prerequisite ${p} not found`);
      if (p === doc.id) report(file, "lesson cannot be its own prerequisite");
    }

    const goalIds = new Set();
    for (const goal of doc.goals) {
      if (goalIds.has(goal.id)) report(file, `duplicate goal id ${goal.id}`);
      goalIds.add(goal.id);
      for (const t of targets(goal)) {
        if (CLUSTER_SCOPED.has(t.kind) && t.namespace)
          report(file, `goal ${goal.id}: ${t.kind} is cluster-scoped; remove namespace`);
      }
      if (goal.type === "answer") {
        if (!(goal.id in (doc.solution.answers ?? {})))
          report(file, `goal ${goal.id}: solution.answers must include an answer`);
        if (goal.input === "choice" && !goal.choices.includes(String(goal.expected.static))) {
          report(file, `goal ${goal.id}: expected answer must be one of the choices`);
        }
      }
    }
    for (const id of Object.keys(doc.solution.answers ?? {})) {
      if (!goalIds.has(id)) report(file, `solution.answers.${id} does not match an answer goal`);
    }
  }

  if (!partial) {
    for (const course of courses.values()) {
      for (const lesson of course.lessons) {
        if (!lessonIds.has(`${course.slug}/${lesson}`)) {
          report(join(root, "courses", `${course.slug}.yaml`), `lesson ${lesson} has no challenge file`);
        }
      }
    }
  }

  return problems;
}

/** Type of an example file in an expect-invalid directory, from its name prefix. */
export function typeFromName(file) {
  const name = basename(file);
  for (const type of ["course", "challenge", "fixture"]) if (name.startsWith(`${type}-`)) return type;
  return undefined;
}
