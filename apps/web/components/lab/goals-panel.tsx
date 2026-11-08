"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { CheckCircle2, Circle, Lightbulb, Loader2, PartyPopper, XCircle } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { bff } from "@/lib/bff-client";
import { cn } from "@/lib/utils";

export interface PublicGoal {
  id: string;
  type: "state" | "answer" | "observation";
  description: string;
  prompt?: string;
  input?: "text" | "number" | "choice";
  choices?: string[];
}

interface GoalResult {
  goalId: string;
  passed: boolean;
  message: string;
}

export interface ValidationResult {
  attemptId: string;
  passed: boolean;
  score: number;
  xpAwarded: number;
  goals: GoalResult[];
  labState: string;
  evaluatedAt: string;
  achievements?: { key: string; title: string }[];
}

export interface AnswerStatus {
  goalId: string;
  submissions: number;
  submissionsRemaining: number;
  lastAnswer: string | null;
}

export interface Hint {
  index: number;
  text: string;
  penaltyXp: number;
  hintsUsed: number;
  hintCount: number;
}

export interface LabGoals {
  answers: AnswerStatus[];
  hints: Hint[];
  hintCount: number;
  hintPenaltyXp: number;
  lastAttempt: ValidationResult | null;
}

export interface Reward {
  baseXp: number;
  firstTryBonusXp: number;
  hintPenaltyPerHintXp: number;
}

/** Lesson text with `code` spans (goal descriptions, feedback and hints use them). */
export function InlineCode({ text }: { text: string }) {
  const parts = text.split("`");
  return (
    <>
      {parts.map((part, i) =>
        i % 2 === 1 && i < parts.length - 1 ? (
          <code key={i} className="rounded-[3px] border bg-background px-1 font-mono text-[0.9em]">
            {part}
          </code>
        ) : (
          <span key={i}>{i % 2 === 1 ? `\`${part}` : part}</span>
        ),
      )}
    </>
  );
}

function StatusIcon({ result }: { result?: GoalResult }) {
  if (!result) return <Circle aria-hidden className="mt-0.5 size-4 shrink-0 text-muted-foreground" />;
  return result.passed ? (
    <CheckCircle2 aria-hidden className="mt-0.5 size-4 shrink-0 text-success" />
  ) : (
    <XCircle aria-hidden className="mt-0.5 size-4 shrink-0 text-danger" />
  );
}

/** An answer goal's question and input. Saving an answer doesn't grade it; checking does. */
function AnswerForm({
  goal,
  status,
  disabled,
  onSaved,
  labId,
}: {
  goal: PublicGoal;
  status?: AnswerStatus;
  disabled: boolean;
  onSaved: (s: AnswerStatus) => void;
  labId: string;
}) {
  const [value, setValue] = useState(status?.lastAnswer ?? "");
  const [saving, setSaving] = useState(false);
  const left = status?.submissionsRemaining;
  const none = left === 0;
  const unchanged = value.trim() === (status?.lastAnswer ?? "");

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!value.trim() || saving) return;
    setSaving(true);
    try {
      const res = await bff<AnswerStatus>(`labs/${labId}/answers`, {
        method: "POST",
        body: { goalId: goal.id, answer: value.trim() },
      });
      if (!res.ok) {
        toast.error("Couldn't save your answer", { description: res.message });
        return;
      }
      onSaved(res.data);
      toast.success("Answer saved", { description: "Check your work to see whether it's right." });
    } finally {
      setSaving(false);
    }
  }

  const inputId = `answer-${goal.id}`;
  return (
    <form onSubmit={submit} className="mt-2 grid gap-2">
      {goal.input === "choice" ? (
        <fieldset disabled={disabled || none} className="grid gap-1.5">
          <legend className="mb-1 font-medium">
            <InlineCode text={goal.prompt ?? ""} />
          </legend>
          {(goal.choices ?? []).map((c) => (
            <label key={c} className="flex items-center gap-2">
              <input
                type="radio"
                name={inputId}
                value={c}
                checked={value === c}
                onChange={() => setValue(c)}
                className="accent-primary"
              />
              {c}
            </label>
          ))}
        </fieldset>
      ) : (
        <>
          <label htmlFor={inputId} className="font-medium">
            <InlineCode text={goal.prompt ?? ""} />
          </label>
          <Input
            id={inputId}
            inputMode={goal.input === "number" ? "decimal" : "text"}
            value={value}
            onChange={(e) => setValue(e.target.value)}
            maxLength={256}
            disabled={disabled || none}
            autoComplete="off"
            className="h-8"
          />
        </>
      )}
      <div className="flex items-center gap-2">
        <Button
          type="submit"
          size="sm"
          variant="outline"
          disabled={disabled || none || saving || !value.trim() || unchanged}
        >
          {saving && <Loader2 aria-hidden className="animate-spin" />} Save answer
        </Button>
        {left !== undefined && (
          <span className="text-xs text-muted-foreground">
            {none ? "No answers left in this lab" : `${left} ${left === 1 ? "answer" : "answers"} left`}
          </span>
        )}
      </div>
    </form>
  );
}

/**
 * The lab's goals: what to do, answers for answer goals, hints, and the check that grades the
 * lab. Grading happens on the server; the panel never sees expected answers.
 */
export function GoalsPanel({
  labId,
  active,
  goals,
  reward,
  initial,
}: {
  labId: string;
  active: boolean;
  goals: PublicGoal[];
  reward: Reward | null;
  initial: LabGoals;
}) {
  const router = useRouter();
  const [answers, setAnswers] = useState(initial.answers);
  const [hints, setHints] = useState(initial.hints);
  const [result, setResult] = useState<ValidationResult | null>(initial.lastAttempt);
  const [checking, setChecking] = useState(false);
  const [revealing, setRevealing] = useState(false);

  const byGoal = new Map(result?.goals.map((g) => [g.goalId, g]));
  const nextHint = hints.length;
  const penalty = initial.hintPenaltyXp;

  async function check() {
    if (checking) return;
    setChecking(true);
    try {
      const res = await bff<ValidationResult>(`labs/${labId}/validate`, {
        method: "POST",
        idempotencyKey: crypto.randomUUID(),
      });
      if (!res.ok) {
        toast.error("Couldn't check your work", { description: res.message });
        return;
      }
      setResult(res.data);
      if (res.data.passed) {
        router.refresh();
      }
    } finally {
      setChecking(false);
    }
  }

  async function reveal() {
    if (revealing) return;
    setRevealing(true);
    try {
      const res = await bff<Hint>(`labs/${labId}/hints/${nextHint}`, { method: "POST" });
      if (!res.ok) {
        toast.error("Couldn't show the hint", { description: res.message });
        return;
      }
      setHints((hs) => [...hs, res.data]);
    } finally {
      setRevealing(false);
    }
  }

  const passed = result?.passed === true;
  return (
    <div className="grid gap-5">
      <ol className="grid gap-3">
        {goals.map((g, i) => {
          const r = byGoal.get(g.id);
          return (
            <li key={g.id} className="flex gap-2">
              <StatusIcon result={r} />
              <div className="min-w-0 flex-1">
                <p>
                  <span className="sr-only">
                    Goal {i + 1}
                    {r ? (r.passed ? ", done: " : ", not done yet: ") : ": "}
                  </span>
                  <InlineCode text={g.description} />
                </p>
                {r && !r.passed && (
                  <p className="mt-1 text-xs text-muted-foreground">
                    <InlineCode text={r.message} />
                  </p>
                )}
                {g.type === "answer" && (
                  <AnswerForm
                    goal={g}
                    labId={labId}
                    disabled={!active}
                    status={answers.find((a) => a.goalId === g.id)}
                    onSaved={(s) => setAnswers((as) => [...as.filter((a) => a.goalId !== s.goalId), s])}
                  />
                )}
                {g.type === "observation" && !r?.passed && (
                  <p className="mt-1 text-xs text-muted-foreground">Checked from the commands you run.</p>
                )}
              </div>
            </li>
          );
        })}
      </ol>

      <div aria-live="polite" className="grid gap-2">
        {result &&
          (passed ? (
            <div className="rounded-lg border border-success/40 bg-success/10 p-3">
              <p className="flex items-center gap-2 font-semibold">
                <PartyPopper aria-hidden className="size-4" /> Every goal passes. Lab complete!
              </p>
              <p className="mt-1">
                {result.xpAwarded > 0
                  ? `You earned ${result.xpAwarded} XP.`
                  : "You've already earned this lesson's XP, so this pass adds none."}
              </p>
              {(result.achievements ?? []).map((a) => (
                <p key={a.key} className="mt-1">
                  Achievement unlocked: <strong>{a.title}</strong>
                </p>
              ))}
            </div>
          ) : (
            <p className="rounded-lg border bg-muted/40 p-3">
              {result.score}% of the goals pass. Fix the ones marked, then check again.
            </p>
          ))}
        {active && (
          <Button onClick={check} disabled={checking}>
            {checking && <Loader2 aria-hidden className="animate-spin" />} Check my work
          </Button>
        )}
        {reward && active && (
          <p className="text-xs text-muted-foreground">
            Pass to earn {reward.baseXp} XP
            {reward.firstTryBonusXp > 0 && `, plus ${reward.firstTryBonusXp} if your first check passes`}.
          </p>
        )}
      </div>

      {initial.hintCount > 0 && (
        <section aria-labelledby="hints-heading" className="grid gap-2">
          <h3 id="hints-heading" className="flex items-center gap-1.5 font-bold">
            <Lightbulb aria-hidden className="size-4" /> Hints
          </h3>
          {hints.length > 0 && (
            <ol className="grid list-decimal gap-1.5 pl-5">
              {hints.map((h) => (
                <li key={h.index}>
                  <InlineCode text={h.text} />
                </li>
              ))}
            </ol>
          )}
          {active && nextHint < initial.hintCount && (
            <Button
              variant="outline"
              size="sm"
              onClick={reveal}
              disabled={revealing}
              className={cn("justify-self-start")}
            >
              Show hint {nextHint + 1} of {initial.hintCount}
              {penalty > 0 && ` (−${penalty} XP)`}
            </Button>
          )}
        </section>
      )}
    </div>
  );
}
