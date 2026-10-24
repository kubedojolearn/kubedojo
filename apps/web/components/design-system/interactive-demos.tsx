"use client";

import { toast } from "sonner";
import { ConfirmDialog } from "@/components/feedback/confirm-dialog";
import { Button } from "@/components/ui/button";

export function ToastDemo() {
  return (
    <div className="flex flex-wrap gap-2">
      <Button variant="outline" onClick={() => toast.success("Lesson complete", { description: "+125 XP" })}>
        Success toast
      </Button>
      <Button variant="outline" onClick={() => toast.error("Couldn't reach the lab", { description: "Reconnecting…" })}>
        Error toast
      </Button>
      <Button variant="outline" onClick={() => toast("Hint revealed", { description: "−10 XP on completion" })}>
        Neutral toast
      </Button>
    </div>
  );
}

export function ConfirmDemo() {
  return (
    <ConfirmDialog
      trigger={<Button variant="destructive">Reset lab…</Button>}
      title="Reset this lab?"
      description="Your simulated cluster goes back to its starting state. Your files and command history are kept."
      confirmLabel="Reset lab"
      destructive
      onConfirm={() => toast("Lab reset")}
    />
  );
}
