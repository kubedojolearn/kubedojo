#!/usr/bin/env node
// Builds the web app's logo files from the master artwork in brand/kubedojo-logo-source.png.
// Run after the artwork changes: `pnpm --filter @kubedojo/web brand:assets`. The outputs are committed.
//
//   public/brand/kubedojo-logo.webp     full lockup (mark + wordmark), 2x for ~320 px display
//   public/brand/kubedojo-mark.png      the mark alone on its navy tile, for the header and footer
//   app/icon.png, app/apple-icon.png    browser and home-screen icons (Next.js file conventions)
//   app/favicon.ico                     16/32/48 px icons for browsers that ask for /favicon.ico
import { mkdirSync, writeFileSync } from "node:fs";
import sharp from "sharp";

const root = new URL("..", import.meta.url).pathname;
const source = `${root}brand/kubedojo-logo-source.png`;
// The artwork's background, sampled from the source (also --brand-ink in globals.css).
const ink = { r: 4, g: 13, b: 29, alpha: 1 };

// Measured from the 1254 px source: the mark sits in x 304–949, y 221–828; the wordmark below it.
const markBox = { left: 287, top: 185, width: 680, height: 680 };
const lockupBox = { left: 150, top: 150, width: 954, height: 954 };

const png = (img) => img.png({ compressionLevel: 9, palette: true, quality: 92, effort: 10 });

async function mark(size, padding) {
  const inner = Math.round(size * (1 - 2 * padding));
  const art = await sharp(source).extract(markBox).resize(inner, inner, { kernel: "lanczos3" }).toBuffer();
  return sharp({ create: { width: size, height: size, channels: 4, background: ink } }).composite([
    { input: art, gravity: "center" },
  ]);
}

/** Minimal ICO writer: a directory of embedded PNGs (supported by every current browser). */
function ico(images) {
  const header = Buffer.alloc(6 + 16 * images.length);
  header.writeUInt16LE(0, 0);
  header.writeUInt16LE(1, 2);
  header.writeUInt16LE(images.length, 4);
  let offset = header.length;
  images.forEach(({ size, data }, i) => {
    const e = 6 + 16 * i;
    header.writeUInt8(size >= 256 ? 0 : size, e);
    header.writeUInt8(size >= 256 ? 0 : size, e + 1);
    header.writeUInt16LE(1, e + 4); // planes
    header.writeUInt16LE(32, e + 6); // bits per pixel
    header.writeUInt32LE(data.length, e + 8);
    header.writeUInt32LE(offset, e + 12);
    offset += data.length;
  });
  return Buffer.concat([header, ...images.map((i) => i.data)]);
}

mkdirSync(`${root}public/brand`, { recursive: true });

await sharp(source)
  .extract(lockupBox)
  .resize(640, 640, { kernel: "lanczos3" })
  .webp({ quality: 88, effort: 6 })
  .toFile(`${root}public/brand/kubedojo-logo.webp`);
await png(await mark(192, 0.08)).toFile(`${root}public/brand/kubedojo-mark.png`);
await png(await mark(192, 0.1)).toFile(`${root}app/icon.png`);
// iOS rounds the corners itself and wants an opaque square.
await png(await mark(180, 0.12)).toFile(`${root}app/apple-icon.png`);

const icons = [];
for (const size of [16, 32, 48]) {
  icons.push({ size, data: await (await mark(size, 0.04)).png().toBuffer() });
}
writeFileSync(`${root}app/favicon.ico`, ico(icons));
console.log("brand assets written");
