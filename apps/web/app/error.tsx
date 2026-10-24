"use client";

import { useEffect } from "react";
import { ErrorState } from "@/components/feedback/states";
import { Button } from "@/components/ui/button";

/** Route-level error boundary. Shows a generic message; details stay in the console and server logs. */
export default function RouteError({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  return (
    <main id="main" tabIndex={-1} className="mx-auto w-full max-w-2xl flex-1 px-4 py-24 outline-none">
      <h1 className="sr-only">Something went wrong</h1>
      <ErrorState
        title="Something went wrong"
        description={
          <>
            This page hit an unexpected error. Try again; if it keeps happening, please report it.
            {error.digest && <span className="mt-2 block font-mono text-xs">Reference: {error.digest}</span>}
          </>
        }
        action={<Button onClick={reset}>Try again</Button>}
      />
    </main>
  );
}
