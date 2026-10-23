"use client";

import { useEffect } from "react";

/**
 * Tells the API the signed-in learner opened this lesson (powers "continue learning"). Fire and
 * forget: failure only means the dashboard pointer isn't updated.
 */
export function LessonOpened({ courseSlug, lessonSlug }: { courseSlug: string; lessonSlug: string }) {
  useEffect(() => {
    void fetch("/api/bff/v1/users/me/progress/opened", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ courseSlug, lessonSlug }),
      keepalive: true,
    }).catch(() => {});
  }, [courseSlug, lessonSlug]);
  return null;
}
