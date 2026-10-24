#!/usr/bin/env node
// Creates .env from .env.example, replacing every GENERATED value with a fresh development secret.
//
//   node scripts/gen-env.mjs                     create .env if it does not exist
//   node scripts/gen-env.mjs --print-keypair ID  print a new Ed25519 keypair for rotation
import { generateKeyPairSync, randomBytes } from "node:crypto";
import { existsSync, readFileSync, writeFileSync } from "node:fs";

export function ed25519Keypair(kid) {
  const { privateKey, publicKey } = generateKeyPairSync("ed25519");
  return {
    privateKey: privateKey.export({ format: "der", type: "pkcs8" }).toString("base64"),
    publicKeyEntry: `${kid}:${publicKey.export({ format: "der", type: "spki" }).toString("base64")}`,
  };
}

const args = process.argv.slice(2);
if (args[0] === "--print-keypair") {
  const kid = args[1];
  if (!kid || !/^[A-Za-z0-9._-]{1,32}$/.test(kid)) {
    console.error("usage: node scripts/gen-env.mjs --print-keypair <kid>   (kid: 1-32 of A-Z a-z 0-9 . _ -)");
    process.exit(2);
  }
  const { privateKey, publicKeyEntry } = ed25519Keypair(kid);
  console.log(`BFF_SIGNING_KID=${kid}`);
  console.log(`BFF_SIGNING_PRIVATE_KEY=${privateKey}`);
  console.log(`# Append to API_JWT_PUBLIC_KEYS (comma-separated):`);
  console.log(publicKeyEntry);
  process.exit(0);
}

if (existsSync(".env")) {
  console.log(".env already exists; leaving it unchanged.");
  process.exit(0);
}

const template = readFileSync(".env.example", "utf8");
const values = new Map(
  template
    .split("\n")
    .filter((l) => /^[A-Z0-9_]+=/.test(l))
    .map((l) => [l.slice(0, l.indexOf("=")), l.slice(l.indexOf("=") + 1)]),
);
const kid = values.get("BFF_SIGNING_KID") || "dev-1";
const dbPassword = randomBytes(18).toString("base64url");
const { privateKey, publicKeyEntry } = ed25519Keypair(kid);

const generated = {
  POSTGRES_PASSWORD: dbPassword,
  DATABASE_URL: (values.get("DATABASE_URL") ?? "").replace("GENERATED", dbPassword),
  TEST_DATABASE_URL: (values.get("TEST_DATABASE_URL") ?? "").replace("GENERATED", dbPassword),
  AUTH_SECRET: randomBytes(32).toString("base64"),
  BFF_SIGNING_PRIVATE_KEY: privateKey,
  API_JWT_PUBLIC_KEYS: publicKeyEntry,
};

const out = template
  .split("\n")
  .map((line) => {
    const key = line.slice(0, line.indexOf("="));
    return /^[A-Z0-9_]+=/.test(line) && key in generated ? `${key}=${generated[key]}` : line;
  })
  .join("\n");
if (out.split("\n").some((l) => /^[A-Z0-9_]+=.*GENERATED/.test(l))) {
  console.error("internal error: a GENERATED placeholder was not replaced");
  process.exit(1);
}
writeFileSync(".env", out, { mode: 0o600 });
console.log("Created .env with fresh development secrets (mode 600).");
