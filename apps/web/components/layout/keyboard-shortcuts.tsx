"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Kbd, KbdGroup } from "@/components/ui/kbd";
import { useKeyboardShortcuts, type Shortcut } from "@/hooks/use-keyboard-shortcuts";

/** Global shortcuts and the "?" help dialog that lists them. Mounted once in the root layout. */
export function KeyboardShortcuts() {
  const router = useRouter();
  const [open, setOpen] = useState(false);

  const shortcuts: Shortcut[] = [
    { keys: ["?"], description: "Show keyboard shortcuts", action: () => setOpen(true) },
    { keys: ["g", "h"], description: "Go to home", action: () => router.push("/") },
    { keys: ["g", "l"], description: "Go to the curriculum", action: () => router.push("/learn") },
    { keys: ["g", "d"], description: "Go to your dashboard", action: () => router.push("/dashboard") },
    { keys: ["g", "s"], description: "Go to settings", action: () => router.push("/settings") },
  ];
  useKeyboardShortcuts(shortcuts);

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Keyboard shortcuts</DialogTitle>
          <DialogDescription>Shortcuts are disabled while you are typing in a field or the terminal.</DialogDescription>
        </DialogHeader>
        <dl className="grid gap-3">
          {shortcuts.map((s) => (
            <div key={s.keys.join("+")} className="flex items-center justify-between gap-4">
              <dt className="text-sm">{s.description}</dt>
              <dd>
                <KbdGroup>
                  {s.keys.map((k, i) => (
                    <span key={i} className="inline-flex items-center gap-1">
                      {i > 0 && <span className="text-xs text-muted-foreground">then</span>}
                      <Kbd>{k}</Kbd>
                    </span>
                  ))}
                </KbdGroup>
              </dd>
            </div>
          ))}
        </dl>
      </DialogContent>
    </Dialog>
  );
}
