import type { Metadata, Viewport } from "next";
import { Inter, JetBrains_Mono } from "next/font/google";
import { KeyboardShortcuts } from "@/components/layout/keyboard-shortcuts";
import { SkipLink } from "@/components/layout/skip-link";
import { ThemeProvider } from "@/components/theme/theme-provider";
import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { site } from "@/lib/site";
import { cn } from "@/lib/utils";
import "./globals.css";

// One variable family for headings and body, like GitDojo.
const inter = Inter({ subsets: ["latin"], variable: "--font-inter" });
// Mono is only used below the fold or in the lab; don't let it compete with the body font for bandwidth.
const jetbrainsMono = JetBrains_Mono({ subsets: ["latin"], variable: "--font-jetbrains-mono", preload: false });

export const metadata: Metadata = {
  // Absolute URLs for social cards. Set WEB_ORIGIN at build time for production.
  metadataBase: new URL(process.env.WEB_ORIGIN ?? "http://localhost:8000"),
  title: { default: `${site.name} — ${site.tagline}`, template: `%s · ${site.name}` },
  description: site.description,
  applicationName: site.name,
  openGraph: { type: "website", siteName: site.name, title: site.name, description: site.description, url: "/" },
  twitter: { card: "summary_large_image", title: site.name, description: site.description },
};

export const viewport: Viewport = {
  themeColor: [
    { media: "(prefers-color-scheme: dark)", color: "#0B0D10" },
    { media: "(prefers-color-scheme: light)", color: "#F4F6F9" },
  ],
  colorScheme: "dark light",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html
      lang="en"
      suppressHydrationWarning
      className={cn("h-full antialiased", inter.variable, jetbrainsMono.variable)}
    >
      <body className="flex min-h-full flex-col">
        <ThemeProvider>
          <TooltipProvider>
            <SkipLink />
            {children}
            <KeyboardShortcuts />
            <Toaster />
          </TooltipProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
