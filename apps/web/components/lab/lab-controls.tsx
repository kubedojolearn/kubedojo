"use client";

import { useRouter } from "next/navigation";
import { useTransition } from "react";
import { LogOut, RotateCcw } from "lucide-react";
import { toast } from "sonner";
import { ConfirmDialog } from "@/components/feedback/confirm-dialog";
import { Button } from "@/components/ui/button";
import { bff } from "@/lib/bff-client";

/** Reset and finish for an active lab. Each click is one idempotent request. */
export function LabControls({ labId }: { labId: string }) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();

  function act(action: "reset" | "finish", success: string) {
    startTransition(async () => {
      const res = await bff(`labs/${labId}/${action}`, { method: "POST", idempotencyKey: crypto.randomUUID() });
      if (!res.ok) {
        toast.error(action === "reset" ? "Couldn't reset the lab" : "Couldn't end the lab", {
          description: res.message,
        });
      } else {
        toast.success(success);
      }
      router.refresh();
    });
  }

  return (
    <div className="flex flex-wrap gap-2">
      <ConfirmDialog
        trigger={
          <Button variant="outline" disabled={pending}>
            <RotateCcw /> Reset
          </Button>
        }
        title="Reset this lab?"
        description="Your simulated cluster goes back to how the lab started. Your command history is kept."
        confirmLabel="Reset lab"
        destructive
        onConfirm={() => act("reset", "Lab reset to its starting state")}
      />
      <ConfirmDialog
        trigger={
          <Button variant="outline" disabled={pending}>
            <LogOut /> End lab
          </Button>
        }
        title="End this lab?"
        description="The lab becomes read-only and doesn't count as completed. You can start the lesson again any time."
        confirmLabel="End lab"
        destructive
        onConfirm={() => act("finish", "Lab ended")}
      />
    </div>
  );
}
