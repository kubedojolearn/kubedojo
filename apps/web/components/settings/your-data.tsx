"use client";

import { Download, Trash2 } from "lucide-react";
import { useId, useState, type FormEvent } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { signOutAction } from "@/lib/auth/actions";
import { bff } from "@/lib/bff-client";

function saveJson(data: unknown, filename: string) {
  const url = URL.createObjectURL(new Blob([JSON.stringify(data, null, 2)], { type: "application/json" }));
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

/** Data export and account deletion (Settings → Your data). */
export function YourData({ handle }: { handle: string }) {
  const ids = useId();
  const [exporting, setExporting] = useState(false);
  const [confirm, setConfirm] = useState("");
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const matches = confirm.trim().toLowerCase() === handle.toLowerCase();

  async function exportData() {
    setExporting(true);
    const res = await bff<unknown>("users/me/export");
    setExporting(false);
    if (!res.ok) {
      toast.error(res.status === 429 ? "You've exported a lot recently. Try again in an hour." : res.message);
      return;
    }
    saveJson(res.data, `kubedojo-${handle}-${new Date().toISOString().slice(0, 10)}.json`);
    toast.success("Your data is downloading.");
  }

  async function deleteAccount(e: FormEvent) {
    e.preventDefault();
    if (!matches) return;
    setDeleting(true);
    setError(null);
    const res = await bff<void>("users/me/deletion", {
      method: "POST",
      body: { confirmHandle: confirm.trim() },
      idempotencyKey: crypto.randomUUID(),
    });
    // 401 after a retry means the first request already deleted the account.
    if (res.ok || res.status === 401) {
      toast.success("Your account is deleted. Thanks for learning with KubeDojo.");
      await signOutAction();
      return;
    }
    setDeleting(false);
    setError(res.message);
  }

  return (
    <section aria-labelledby={`${ids}-heading`} className="mt-10 max-w-3xl rounded-lg border bg-card p-6">
      <h2 id={`${ids}-heading`} className="text-lg font-bold">
        Your data
      </h2>
      <div className="mt-5 grid gap-6 sm:grid-cols-2">
        <div>
          <h3 className="font-semibold">Export</h3>
          <p className="mt-1 text-sm text-muted-foreground">
            A JSON file with your profile, settings, progress, XP, achievements, activity and lab history. Secret values
            in command output stay redacted.
          </p>
          <Button variant="outline" className="mt-3" onClick={exportData} disabled={exporting}>
            <Download /> {exporting ? "Preparing…" : "Download my data"}
          </Button>
        </div>
        <div>
          <h3 className="font-semibold">Delete account</h3>
          <p className="mt-1 text-sm text-muted-foreground">
            Your account disappears immediately and is unlinked from GitHub. Everything else is erased within 30 days.
            This can&apos;t be undone.
          </p>
          <Dialog
            onOpenChange={(open) => {
              if (!open) {
                setConfirm("");
                setError(null);
              }
            }}
          >
            <DialogTrigger asChild>
              <Button variant="destructive" className="mt-3">
                <Trash2 /> Delete my account…
              </Button>
            </DialogTrigger>
            <DialogContent>
              <form onSubmit={deleteAccount} className="grid gap-4">
                <DialogHeader>
                  <DialogTitle>Delete your account?</DialogTitle>
                  <DialogDescription>
                    Your progress, XP, achievements and labs will be erased. You can sign up again later with the same
                    GitHub account, but you&apos;ll start from scratch.
                  </DialogDescription>
                </DialogHeader>
                <div className="grid gap-1.5">
                  <Label htmlFor={`${ids}-confirm`}>
                    Type <span className="font-mono">{handle}</span> to confirm
                  </Label>
                  <Input
                    id={`${ids}-confirm`}
                    value={confirm}
                    onChange={(e) => setConfirm(e.target.value)}
                    autoComplete="off"
                    spellCheck={false}
                    aria-invalid={error ? true : undefined}
                    aria-describedby={error ? `${ids}-error` : undefined}
                  />
                  {error && (
                    <p id={`${ids}-error`} role="alert" className="text-sm text-danger">
                      {error}
                    </p>
                  )}
                </div>
                <DialogFooter>
                  <DialogClose asChild>
                    <Button type="button" variant="outline">
                      Keep my account
                    </Button>
                  </DialogClose>
                  <Button type="submit" variant="destructive" disabled={!matches || deleting}>
                    {deleting ? "Deleting…" : "Delete account"}
                  </Button>
                </DialogFooter>
              </form>
            </DialogContent>
          </Dialog>
        </div>
      </div>
    </section>
  );
}
