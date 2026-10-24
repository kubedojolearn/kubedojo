import Link from "next/link";
import { SiteFooter } from "@/components/layout/site-footer";
import { SiteHeader } from "@/components/layout/site-header";
import { Container } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";

export default function NotFound() {
  return (
    <>
      <SiteHeader />
      <main id="main" tabIndex={-1} className="flex-1 outline-none">
        <Container className="max-w-3xl py-20 sm:py-28">
          <h1 className="text-h2 font-bold tracking-tight sm:text-h1">Page not found</h1>
          <pre
            tabIndex={0}
            role="group"
            aria-label="Example kubectl error"
            className="mt-6 overflow-x-auto rounded-lg border border-white/10 bg-terminal p-4 font-mono text-sm text-terminal-foreground"
          >
            <span className="text-terminal-prompt">$ </span>kubectl get page{"\n"}
            <span className="text-terminal-error">
              Error from server (NotFound): pages &quot;this-one&quot; not found
            </span>
          </pre>
          <p className="mt-6 max-w-prose text-muted-foreground">
            The link may be out of date, or the lesson may not be published yet.
          </p>
          <div className="mt-6 flex flex-wrap gap-3">
            <Button asChild>
              <Link href="/learn">Browse the curriculum</Link>
            </Button>
            <Button asChild variant="outline">
              <Link href="/">Go home</Link>
            </Button>
          </div>
        </Container>
      </main>
      <SiteFooter />
    </>
  );
}
