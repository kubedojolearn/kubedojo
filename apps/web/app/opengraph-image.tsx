import { ImageResponse } from "next/og";
import { readFile } from "node:fs/promises";
import { join } from "node:path";

export const alt = "KubeDojo — Master Kubernetes, one command at a time.";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

export default async function OpenGraphImage() {
  const mark = await readFile(join(process.cwd(), "public/brand/kubedojo-mark.png"));
  const markSrc = `data:image/png;base64,${mark.toString("base64")}`;
  return new ImageResponse(
    <div
      style={{
        width: "100%",
        height: "100%",
        display: "flex",
        flexDirection: "column",
        justifyContent: "space-between",
        padding: 72,
        background: "#0B0D10",
        color: "#F4F7FB",
        borderTop: "12px solid #4A67E0",
      }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: 20 }}>
        <img src={markSrc} width={96} height={96} alt="" style={{ borderRadius: 12 }} />
        <div style={{ display: "flex", fontSize: 52, fontWeight: 700 }}>
          Kube<span style={{ color: "#6C8CFF" }}>Dojo</span>
        </div>
      </div>
      <div style={{ display: "flex", flexDirection: "column", gap: 20 }}>
        <div style={{ fontSize: 68, fontWeight: 700, lineHeight: 1.1, maxWidth: 980 }}>
          Master Kubernetes, one command at a time.
        </div>
        <div style={{ fontSize: 30, color: "#AEB6C2", maxWidth: 980 }}>
          Short lessons and real kubectl against a simulated cluster. Free and open source.
        </div>
      </div>
    </div>,
    size,
  );
}
