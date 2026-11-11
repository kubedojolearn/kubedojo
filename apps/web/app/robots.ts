import type { MetadataRoute } from "next";
import { serverEnv } from "@/lib/server-env";

export default function robots(): MetadataRoute.Robots {
  return {
    rules: [
      { userAgent: "*", allow: "/", disallow: ["/api/", "/dashboard", "/settings", "/lab/", "/admin", "/login"] },
    ],
    sitemap: `${serverEnv.webOrigin}/sitemap.xml`,
  };
}
