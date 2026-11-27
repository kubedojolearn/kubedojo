"use client";

import { useState } from "react";
import { CheckCircle2, Loader2, ShieldCheck, XCircle } from "lucide-react";
import { toast } from "sonner";
import type { Schemas } from "@kubedojo/api-client";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { bff } from "@/lib/bff-client";

type AdminLessonContent = Schemas["AdminLessonContent"];
type ContentValidationReport = Schemas["ContentValidationReport"];

const dateFormat = new Intl.DateTimeFormat("en-GB", {
  day: "numeric",
  month: "short",
  year: "numeric",
  timeZone: "UTC",
});

export function AdminContentTable({ initialLessons }: { initialLessons: AdminLessonContent[] }) {
  const [lessons, setLessons] = useState<AdminLessonContent[]>(initialLessons);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [reports, setReports] = useState<Record<string, ContentValidationReport>>({});

  async function handleValidate(versionId: string, label: string) {
    setBusyId(versionId);
    const res = await bff<ContentValidationReport>("admin/content/validate", {
      method: "POST",
      body: { challengeVersionId: versionId },
    });
    setBusyId(null);
    if (!res.ok) {
      toast.error(`Validation failed for ${label}: ${res.message}`);
      return;
    }
    setReports((prev) => ({ ...prev, [versionId]: res.data }));
    if (res.data.valid) {
      toast.success(`${label} passed validation (${res.data.solutionReplay.goals.length} goals).`);
    } else {
      toast.error(`${label} failed validation (${res.data.errors.length} error(s)).`);
    }
  }

  async function handlePublishAction(versionId: string, action: "publish" | "unpublish", label: string) {
    setBusyId(versionId);
    const res = await bff<AdminLessonContent>("admin/content/publish", {
      method: "POST",
      body: { challengeVersionId: versionId, action },
      idempotencyKey: crypto.randomUUID(),
    });
    setBusyId(null);
    if (!res.ok) {
      toast.error(`Could not ${action} ${label}: ${res.message}`);
      return;
    }
    setLessons((prev) => prev.map((l) => (l.lessonId === res.data.lessonId ? res.data : l)));
    toast.success(action === "publish" ? `Published ${label}.` : `Unpublished ${label}.`);
  }

  return (
    <div className="overflow-x-auto rounded-lg border bg-card">
      <table className="w-full text-sm">
        <caption className="sr-only">Lessons and their imported versions</caption>
        <thead className="border-b text-left text-xs text-muted-foreground">
          <tr>
            <th scope="col" className="px-4 py-3 font-medium">
              Lesson
            </th>
            <th scope="col" className="px-4 py-3 font-medium">
              Versions
            </th>
          </tr>
        </thead>
        <tbody className="divide-y">
          {lessons.map((l) => (
            <tr key={l.lessonId} className="align-top">
              <td className="px-4 py-3">
                <p className="font-medium">{l.title}</p>
                <p className="font-mono text-xs text-muted-foreground">{l.lessonId}</p>
              </td>
              <td className="px-4 py-3">
                {l.versions.length === 0 ? (
                  <span className="text-muted-foreground">Not imported</span>
                ) : (
                  <ul className="grid gap-2">
                    {l.versions.map((v) => {
                      const isCurrent = v.id === l.currentVersionId;
                      const isBusy = busyId === v.id;
                      const label = `${l.lessonId} v${v.version}`;
                      const report = reports[v.id];
                      return (
                        <li key={v.id} className="grid gap-1.5">
                          <div className="flex flex-wrap items-center justify-between gap-2">
                            <div className="flex flex-wrap items-center gap-2">
                              <span className="font-mono text-xs">v{v.version}</span>
                              <Badge variant={v.status === "published" ? "default" : "outline"}>{v.status}</Badge>
                              {isCurrent && <Badge variant="secondary">current</Badge>}
                              <span className="text-xs text-muted-foreground">
                                imported {dateFormat.format(new Date(v.importedAt))}
                                {v.publishedAt && `, published ${dateFormat.format(new Date(v.publishedAt))}`}
                              </span>
                              <code className="font-mono text-[11px] text-muted-foreground">
                                {v.contentSha256.slice(0, 12)}
                              </code>
                            </div>
                            <div className="flex items-center gap-1.5">
                              <Button
                                type="button"
                                size="sm"
                                variant="outline"
                                disabled={busyId !== null}
                                onClick={() => handleValidate(v.id, label)}
                                aria-label={`Validate ${label}`}
                              >
                                {isBusy ? (
                                  <Loader2 className="size-3.5 animate-spin" aria-hidden="true" />
                                ) : (
                                  <ShieldCheck className="size-3.5" aria-hidden="true" />
                                )}
                                Validate
                              </Button>
                              {isCurrent ? (
                                <Button
                                  type="button"
                                  size="sm"
                                  variant="outline"
                                  disabled={busyId !== null}
                                  onClick={() => handlePublishAction(v.id, "unpublish", label)}
                                  aria-label={`Unpublish ${label}`}
                                >
                                  Unpublish
                                </Button>
                              ) : (
                                <Button
                                  type="button"
                                  size="sm"
                                  disabled={busyId !== null}
                                  onClick={() => handlePublishAction(v.id, "publish", label)}
                                  aria-label={`Publish ${label}`}
                                >
                                  Publish
                                </Button>
                              )}
                            </div>
                          </div>
                          {report && (
                            <div
                              role="status"
                              className="rounded-md border bg-muted/40 px-3 py-2 text-xs text-muted-foreground"
                            >
                              <div className="flex items-center gap-1.5 font-medium text-foreground">
                                {report.valid ? (
                                  <CheckCircle2 className="size-3.5 text-emerald-500" aria-hidden="true" />
                                ) : (
                                  <XCircle className="size-3.5 text-destructive" aria-hidden="true" />
                                )}
                                <span>
                                  {report.valid
                                    ? `Valid — reference solution passed all ${report.solutionReplay.goals.length} goal(s).`
                                    : `Invalid — ${report.errors.length} error(s).`}
                                </span>
                              </div>
                              {report.errors.length > 0 && (
                                <ul className="mt-1.5 list-disc space-y-0.5 pl-4 text-destructive">
                                  {report.errors.map((e, i) => (
                                    <li key={`${e.path}-${i}`}>
                                      <code className="font-mono">{e.path}</code>: {e.message}
                                    </li>
                                  ))}
                                </ul>
                              )}
                            </div>
                          )}
                        </li>
                      );
                    })}
                  </ul>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
