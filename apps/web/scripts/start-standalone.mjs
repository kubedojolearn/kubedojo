#!/usr/bin/env node
// Runs the production standalone server exactly as the Docker image does (apps/web/Dockerfile):
// copies static assets next to server.js, then starts it. Usage: node scripts/start-standalone.mjs [port]
import { cpSync, existsSync } from "node:fs";
import { spawn } from "node:child_process";

const root = new URL("..", import.meta.url).pathname;
const standalone = `${root}.next/standalone/apps/web`;
if (!existsSync(`${standalone}/server.js`)) {
  console.error("No standalone build found. Run `pnpm build` first.");
  process.exit(1);
}
cpSync(`${root}.next/static`, `${standalone}/.next/static`, { recursive: true });
// public/ is optional (icons live in app/), and git does not keep it when empty.
if (existsSync(`${root}public`)) cpSync(`${root}public`, `${standalone}/public`, { recursive: true });

const port = process.argv[2] ?? process.env.PORT ?? "3000";
const child = spawn(process.execPath, ["server.js"], {
  cwd: standalone,
  stdio: "inherit",
  // Not $HOSTNAME: shells and Docker set it to the machine name, which would bind to the wrong interface.
  env: { ...process.env, PORT: port, HOSTNAME: process.env.WEB_BIND_HOST ?? "127.0.0.1" },
});
for (const sig of ["SIGINT", "SIGTERM"]) process.on(sig, () => child.kill(sig));
child.on("exit", (code) => process.exit(code ?? 0));
