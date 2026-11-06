import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Vercel builds Next.js into its native function output. Docker uses the standalone bundle.
  ...(process.env.VERCEL ? {} : { output: "standalone" as const }),
  // Trace files from the monorepo root so workspace packages are included in the standalone output.
  outputFileTracingRoot: new URL("../..", import.meta.url).pathname,
  poweredByHeader: false,
  // Baseline security headers. A nonce-based Content-Security-Policy is added with authentication
  // (Part 06), when the proxy that generates nonces exists.
  async headers() {
    return [
      {
        source: "/:path*",
        headers: [
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
          { key: "X-Frame-Options", value: "DENY" },
          { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=(), payment=(), usb=()" },
          { key: "Cross-Origin-Opener-Policy", value: "same-origin" },
        ],
      },
    ];
  },
  reactStrictMode: true,
  cacheComponents: true,
  partialPrefetching: true,
  turbopack: {
    rules: {
      "*.css": {
        loaders: ["@tailwindcss/turbopack"],
        as: "*.css",
      },
    },
  },
};

export default nextConfig;
