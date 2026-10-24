import { ImageResponse } from "next/og";
import { readFile } from "node:fs/promises";
import { join } from "node:path";

export const alt = "KubeDojo — Master Kubernetes, one command at a time.";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

export default async function OpenGraphImage() {
  const mark = await readFile(join(process.cwd(), "app/icon.svg"), "utf8");
  const markSrc = `data:image/svg+xml;base64,${Buffer.from(mark).toString("base64")}`;
  return new ImageResponse(
    <div
      style={{
        width: "100%",
        height: "100%",
        display: "flex",
        flexDirection: "column",
        justifyContent: "space-between",
        padding: 72,
        background: "linear-gradient(135deg, #09111F 0%, #111C2E 60%, #12305f 100%)",
        color: "#E6EDF7",
      }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: 20 }}>
        <img src={markSrc} width={88} height={88} alt="" />
        <div style={{ display: "flex", fontSize: 52, fontWeight: 700 }}>
          Kube<span style={{ color: "#22D3EE" }}>Dojo</span>
        </div>
      </div>
      <div style={{ display: "flex", flexDirection: "column", gap: 20 }}>
        <div style={{ fontSize: 68, fontWeight: 700, lineHeight: 1.1, maxWidth: 980 }}>
          Master Kubernetes, one command at a time.
        </div>
        <div style={{ fontSize: 30, color: "#9AABC4", maxWidth: 980 }}>
          Hands-on lessons with a simulated cluster, a live cluster map and instant feedback. Free and open source.
        </div>
      </div>
    </div>,
    size,
  );
}
