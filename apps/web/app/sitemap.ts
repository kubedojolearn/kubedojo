import type { MetadataRoute } from "next";
import { serverEnv } from "@/lib/server-env";

const publicPaths = ["/", "/learn", "/about", "/faq", "/changelog", "/privacy", "/terms"];

export default function sitemap(): MetadataRoute.Sitemap {
  return publicPaths.map((path) => ({ url: `${serverEnv.webOrigin}${path}` }));
}
