import type { Metadata } from "next";
import { Suspense } from "react";
import { AdminsOnly } from "@/components/admin/admins-only";
import { CardListSkeleton, LoadingStatus } from "@/components/feedback/loading";
import { PageHeader } from "@/components/layout/page-header";
import { Badge } from "@/components/ui/badge";
import { getAdminContent } from "@/lib/admin";

export const metadata: Metadata = { title: "Content" };

const dateFormat = new Intl.DateTimeFormat("en-GB", {
  day: "numeric",
  month: "short",
  year: "numeric",
  timeZone: "UTC",
});

async function ContentTable() {
  const lessons = await getAdminContent();
  if (!lessons) return <AdminsOnly />;
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
                  <ul className="grid gap-1.5">
                    {l.versions.map((v) => (
                      <li key={v.id} className="flex flex-wrap items-center gap-2">
                        <span className="font-mono text-xs">v{v.version}</span>
                        <Badge variant={v.status === "published" ? "default" : "outline"}>{v.status}</Badge>
                        {v.id === l.currentVersionId && <Badge variant="secondary">current</Badge>}
                        <span className="text-xs text-muted-foreground">
                          imported {dateFormat.format(new Date(v.importedAt))}
                          {v.publishedAt && `, published ${dateFormat.format(new Date(v.publishedAt))}`}
                        </span>
                        <code className="font-mono text-[11px] text-muted-foreground">
                          {v.contentSha256.slice(0, 12)}
                        </code>
                      </li>
                    ))}
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

export default function AdminContentPage() {
  return (
    <>
      <PageHeader
        title="Content"
        description={
          <>
            Every lesson and its imported versions. Lessons are written and reviewed in Git; publish with{" "}
            <code className="font-mono text-sm">make content-import</code>. Publishing from here arrives in a later
            release.
          </>
        }
      />
      <Suspense
        fallback={
          <>
            <LoadingStatus label="Loading content" />
            <CardListSkeleton count={1} />
          </>
        }
      >
        <ContentTable />
      </Suspense>
    </>
  );
}
