import { connection } from "next/server";
import { getBuildInfo } from "@/lib/build-info";

/** Liveness probe for the web tier. Reports build metadata only; checks no dependencies. */
export async function GET() {
  // Render at request time so the response reflects the running process, not the build.
  await connection();
  return Response.json({ status: "ok", ...getBuildInfo() }, { headers: { "Cache-Control": "no-store" } });
}
