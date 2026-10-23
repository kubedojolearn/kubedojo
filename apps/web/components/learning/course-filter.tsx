"use client";

import Link from "next/link";
import { useId, useMemo, useState } from "react";
import { EmptyState } from "@/components/feedback/states";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export interface CatalogCourse {
  slug: string;
  title: string;
  summary: string;
  difficulty: string;
  lessonCount: number;
  plannedLessonCount: number;
  estimatedMinutes: number;
}

/** Client-side search and difficulty filter over the (small) published catalog. */
export function CourseFilter({ courses }: { courses: CatalogCourse[] }) {
  const ids = useId();
  const [query, setQuery] = useState("");
  const [level, setLevel] = useState<"all" | "beginner" | "intermediate">("all");
  const shown = useMemo(() => {
    const q = query.trim().toLowerCase();
    return courses.filter(
      (c) =>
        (level === "all" || c.difficulty === level) &&
        (!q || c.title.toLowerCase().includes(q) || c.summary.toLowerCase().includes(q)),
    );
  }, [courses, query, level]);

  return (
    <div className="grid gap-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-end">
        <div className="grid gap-1.5 sm:w-72">
          <Label htmlFor={`${ids}-q`}>Search courses</Label>
          <Input
            id={`${ids}-q`}
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="e.g. networking"
          />
        </div>
        <fieldset className="flex flex-wrap items-center gap-3">
          <legend className="mb-1.5 text-sm font-medium">Level</legend>
          {(["all", "beginner", "intermediate"] as const).map((l) => (
            <label key={l} className="flex items-center gap-1.5 text-sm capitalize">
              <input type="radio" name={`${ids}-level`} value={l} checked={level === l} onChange={() => setLevel(l)} />
              {l}
            </label>
          ))}
        </fieldset>
      </div>
      <p role="status" aria-live="polite" className="text-sm text-muted-foreground">
        {shown.length} of {courses.length} courses
      </p>
      {shown.length === 0 ? (
        <EmptyState title="No courses match" description="Try a different search or level." headingLevel={2} />
      ) : (
        <ul className="grid gap-4 md:grid-cols-2">
          {shown.map((c) => (
            <li key={c.slug}>
              <Card className="h-full transition-colors focus-within:border-ring hover:border-primary/60">
                <CardHeader>
                  <div className="flex items-center justify-between gap-2">
                    <Badge variant="outline" className="capitalize">
                      {c.difficulty}
                    </Badge>
                    <span className="text-xs text-muted-foreground">~{c.estimatedMinutes} min</span>
                  </div>
                  <CardTitle className="text-lg">
                    <h2>
                      <Link
                        href={`/learn/${c.slug}`}
                        className="after:absolute after:inset-0 focus-visible:outline-none"
                      >
                        {c.title}
                      </Link>
                    </h2>
                  </CardTitle>
                  <CardDescription>{c.summary}</CardDescription>
                </CardHeader>
                <CardContent className="mt-auto text-sm text-muted-foreground">
                  {c.lessonCount} of {c.plannedLessonCount} lessons available
                </CardContent>
              </Card>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
