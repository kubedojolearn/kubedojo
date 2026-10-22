#!/usr/bin/env node
// Renders app/icon.svg into app/favicon.ico (16, 32, 48 px) and app/apple-icon.png (180 px).
// Run after changing the mark: `node scripts/generate-icons.mjs` (needs Playwright's Chromium).
import { readFileSync, writeFileSync } from "node:fs";
import { chromium } from "@playwright/test";

const root = new URL("..", import.meta.url).pathname;
const svg = readFileSync(`${root}app/icon.svg`, "utf8");

const browser = await chromium.launch();
const page = await browser.newPage({ deviceScaleFactor: 1 });

async function render(size, { padding = 0, background } = {}) {
  await page.setViewportSize({ width: size, height: size });
  const inner = size - padding * 2;
  await page.setContent(
    `<html><body style="margin:0;background:${background ?? "transparent"}">` +
      `<div style="width:${size}px;height:${size}px;display:grid;place-items:center">` +
      svg.replace("<svg ", `<svg width="${inner}" height="${inner}" `) +
      `</div></body></html>`,
  );
  return page.screenshot({ omitBackground: !background, type: "png" });
}

// ICO container with embedded PNG images (supported by all current browsers).
function ico(images) {
  const header = Buffer.alloc(6);
  header.writeUInt16LE(0, 0);
  header.writeUInt16LE(1, 2);
  header.writeUInt16LE(images.length, 4);
  const entries = [];
  let offset = 6 + 16 * images.length;
  for (const { size, data } of images) {
    const e = Buffer.alloc(16);
    e.writeUInt8(size >= 256 ? 0 : size, 0);
    e.writeUInt8(size >= 256 ? 0 : size, 1);
    e.writeUInt8(0, 2);
    e.writeUInt8(0, 3);
    e.writeUInt16LE(1, 4);
    e.writeUInt16LE(32, 6);
    e.writeUInt32LE(data.length, 8);
    e.writeUInt32LE(offset, 12);
    offset += data.length;
    entries.push(e);
  }
  return Buffer.concat([header, ...entries, ...images.map((i) => i.data)]);
}

const sizes = [16, 32, 48];
const images = [];
for (const size of sizes) images.push({ size, data: await render(size) });
writeFileSync(`${root}app/favicon.ico`, ico(images));
// Apple touch icons are shown on a square with rounded corners applied by iOS; fill the background.
writeFileSync(`${root}app/apple-icon.png`, await render(180, { padding: 10, background: "#09111F" }));
await browser.close();
console.log("Wrote app/favicon.ico and app/apple-icon.png");
