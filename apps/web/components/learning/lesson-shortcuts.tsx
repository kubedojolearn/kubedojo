"use client";

import { useRouter } from "next/navigation";
import { useKeyboardShortcuts } from "@/hooks/use-keyboard-shortcuts";

/** `[` and `]` move to the previous and next lesson. */
export function LessonShortcuts({ previous, next }: { previous?: string; next?: string }) {
  const router = useRouter();
  useKeyboardShortcuts([
    { keys: ["["], description: "Previous lesson", action: () => previous && router.push(previous) },
    { keys: ["]"], description: "Next lesson", action: () => next && router.push(next) },
  ]);
  return null;
}
