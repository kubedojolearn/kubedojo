import type { MetadataRoute } from "next";

const origin = process.env.WEB_ORIGIN ?? "http://localhost:8000";
const publicPaths = ["/", "/learn", "/about", "/faq", "/changelog", "/privacy", "/terms"];

export default function sitemap(): MetadataRoute.Sitemap {
  return publicPaths.map((path) => ({ url: `${origin}${path}` }));
}
