"use client";

import { useRouter } from "next/navigation";
import { useTransition } from "react";
import { FlaskConical } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { bff } from "@/lib/bff-client";

/** Starts (or reopens) the learner's lab for this lesson and opens it. */
export function StartLabButton({ challengeVersionId }: { challengeVersionId: string }) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();

  function start() {
    startTransition(async () => {
      const res = await bff<{ id: string }>("labs", { method: "POST", body: { challengeVersionId } });
      if (!res.ok) {
        toast.error("Couldn't start the lab", { description: res.message });
        if (res.code === "CONFLICT") router.refresh();
        return;
      }
      router.push(`/lab/${res.data.id}`);
    });
  }

  return (
    <Button onClick={start} disabled={pending} aria-busy={pending}>
      <FlaskConical /> {pending ? "Starting…" : "Start lab"}
    </Button>
  );
}
