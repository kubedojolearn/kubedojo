import type { MetadataRoute } from "next";

const origin = process.env.WEB_ORIGIN ?? "http://localhost:8000";

export default function robots(): MetadataRoute.Robots {
  return {
    rules: [
      { userAgent: "*", allow: "/", disallow: ["/api/", "/dashboard", "/settings", "/lab/", "/admin", "/login"] },
    ],
    sitemap: `${origin}/sitemap.xml`,
  };
}
