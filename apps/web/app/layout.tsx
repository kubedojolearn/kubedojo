import type { Metadata, Viewport } from "next";
import { Bricolage_Grotesque, IBM_Plex_Sans, JetBrains_Mono } from "next/font/google";
import { KeyboardShortcuts } from "@/components/layout/keyboard-shortcuts";
import { SkipLink } from "@/components/layout/skip-link";
import { ThemeProvider } from "@/components/theme/theme-provider";
import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { site } from "@/lib/site";
import { cn } from "@/lib/utils";
import "./globals.css";

const plexSans = IBM_Plex_Sans({ subsets: ["latin"], variable: "--font-plex-sans" });
// Headings only; the variable font covers every weight and optical size we use.
const bricolage = Bricolage_Grotesque({ subsets: ["latin"], variable: "--font-bricolage", axes: ["opsz"] });
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
    { media: "(prefers-color-scheme: dark)", color: "#040D1D" },
    { media: "(prefers-color-scheme: light)", color: "#F4F2EC" },
  ],
  colorScheme: "dark light",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html
      lang="en"
      suppressHydrationWarning
      className={cn("h-full antialiased", plexSans.variable, bricolage.variable, jetbrainsMono.variable)}
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
