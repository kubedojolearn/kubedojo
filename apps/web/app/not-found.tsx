import Link from "next/link";
import { Compass } from "lucide-react";
import { EmptyState } from "@/components/feedback/states";
import { SiteFooter } from "@/components/layout/site-footer";
import { SiteHeader } from "@/components/layout/site-header";
import { Container } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";

export default function NotFound() {
  return (
    <>
      <SiteHeader />
      <main id="main" tabIndex={-1} className="flex-1 outline-none">
        <Container className="py-24">
          <h1 className="sr-only">Page not found</h1>
          <EmptyState
            icon={Compass}
            title="We couldn't find that page"
            description={
              <>
                <code className="font-mono">kubectl get page</code> returned NotFound. The link may be out of date, or
                the page may not exist yet.
              </>
            }
            action={
              <Button asChild>
                <Link href="/">Go home</Link>
              </Button>
            }
          />
        </Container>
      </main>
      <SiteFooter />
    </>
  );
}
