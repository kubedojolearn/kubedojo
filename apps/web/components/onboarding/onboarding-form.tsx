"use client";

import { useRouter } from "next/navigation";
import { useId, useState, type FormEvent } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

const levels = [
  { value: "new", label: "New to Kubernetes", hint: "I know what a container is, not much more." },
  { value: "some", label: "Some experience", hint: "I've used kubectl but things still surprise me." },
  { value: "comfortable", label: "Comfortable", hint: "I want practice and the trickier cases." },
] as const;

const goals = [
  { value: "fundamentals", label: "Understand the fundamentals" },
  { value: "troubleshooting", label: "Get better at fixing things" },
  { value: "work-project", label: "Use Kubernetes at work" },
  { value: "certification-prep", label: "Prepare for a certification" },
  { value: "curiosity", label: "Just curious" },
] as const;

/**
 * Three optional questions. Everything can be skipped; skipping still marks onboarding complete
 * so it isn't shown again.
 */
export function OnboardingForm({ handle, next }: { handle: string; next: string }) {
  const router = useRouter();
  const ids = useId();
  const [level, setLevel] = useState<string>();
  const [goal, setGoal] = useState<string>();
  const [newHandle, setNewHandle] = useState(handle);
  const [handleError, setHandleError] = useState<string>();
  const [saving, setSaving] = useState(false);

  async function save(body: Record<string, unknown>) {
    setSaving(true);
    setHandleError(undefined);
    try {
      const res = await fetch("/api/bff/v1/users/me", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...body, completeOnboarding: true }),
      });
      if (!res.ok) {
        const payload = (await res.json().catch(() => null)) as {
          error?: { details?: Array<{ field?: string; message: string }> };
        } | null;
        const h = payload?.error?.details?.find((d) => d.field === "handle");
        if (h) setHandleError(h.message);
        else toast.error("Couldn't save", { description: "Please try again." });
        return;
      }
      router.push(next);
      router.refresh();
    } catch {
      toast.error("Couldn't save", { description: "Check your connection and try again." });
    } finally {
      setSaving(false);
    }
  }

  function submit(e: FormEvent) {
    e.preventDefault();
    const body: Record<string, unknown> = {};
    const onboarding: Record<string, string> = {};
    if (level) onboarding.experienceLevel = level;
    if (goal) onboarding.learningGoal = goal;
    if (Object.keys(onboarding).length) body.onboarding = onboarding;
    if (newHandle.trim() && newHandle.trim() !== handle) body.handle = newHandle.trim();
    void save(body);
  }

  return (
    <form onSubmit={submit} className="grid gap-8" noValidate>
      <fieldset className="grid gap-3">
        <legend className="mb-2 font-medium">How much Kubernetes have you used?</legend>
        {levels.map((l) => (
          <label
            key={l.value}
            className="flex cursor-pointer items-start gap-3 rounded-lg border p-3 has-checked:border-primary has-checked:bg-primary/10"
          >
            <input
              type="radio"
              name="level"
              value={l.value}
              checked={level === l.value}
              onChange={() => setLevel(l.value)}
              className="mt-1"
            />
            <span>
              <span className="block text-sm font-medium">{l.label}</span>
              <span className="block text-sm text-muted-foreground">{l.hint}</span>
            </span>
          </label>
        ))}
      </fieldset>

      <fieldset className="grid gap-2">
        <legend className="mb-2 font-medium">What brings you here?</legend>
        {goals.map((g) => (
          <label key={g.value} className="flex items-center gap-3 text-sm">
            <input
              type="radio"
              name="goal"
              value={g.value}
              checked={goal === g.value}
              onChange={() => setGoal(g.value)}
            />
            {g.label}
          </label>
        ))}
      </fieldset>

      <div className="grid gap-2">
        <Label htmlFor={`${ids}-handle`}>Your handle</Label>
        <Input
          id={`${ids}-handle`}
          value={newHandle}
          onChange={(e) => setNewHandle(e.target.value)}
          maxLength={39}
          autoCapitalize="none"
          spellCheck={false}
          aria-invalid={Boolean(handleError)}
          aria-describedby={`${ids}-handle-help${handleError ? ` ${ids}-handle-error` : ""}`}
        />
        <p id={`${ids}-handle-help`} className="text-sm text-muted-foreground">
          Taken from GitHub; change it if you like. Your profile stays private unless you make it public.
        </p>
        {handleError && (
          <p id={`${ids}-handle-error`} className="text-sm text-danger">
            {handleError}
          </p>
        )}
      </div>

      <div className="flex flex-wrap gap-3">
        <Button type="submit" disabled={saving}>
          {saving ? "Saving…" : "Continue"}
        </Button>
        <Button type="button" variant="ghost" disabled={saving} onClick={() => void save({})}>
          Skip for now
        </Button>
      </div>
    </form>
  );
}
