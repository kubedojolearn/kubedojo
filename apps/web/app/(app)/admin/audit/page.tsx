import type { Metadata } from "next";
import Link from "next/link";
import { Suspense } from "react";
import { ScrollText } from "lucide-react";
import { AdminsOnly } from "@/components/admin/admins-only";
import { CardListSkeleton, LoadingStatus } from "@/components/feedback/loading";
import { EmptyState } from "@/components/feedback/states";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { getAuditLog } from "@/lib/admin";

export const metadata: Metadata = { title: "Audit log" };

type Props = PageProps<"/admin/audit">;

const timeFormat = new Intl.DateTimeFormat("en-GB", {
  day: "numeric",
  month: "short",
  hour: "2-digit",
  minute: "2-digit",
  timeZone: "UTC",
});

async function AuditTable({ searchParams }: Props) {
  const raw = (await searchParams).cursor;
  const cursor = typeof raw === "string" && raw.length <= 512 ? raw : undefined;
  const page = await getAuditLog(cursor);
  if (!page) return <AdminsOnly />;
  if (page.entries.length === 0) {
    return (
      <EmptyState
        icon={ScrollText}
        title="No audit entries yet"
        description="Admin and account actions are recorded here."
      />
    );
  }
  return (
    <>
      <ol className="divide-y rounded-lg border bg-card text-sm">
        {page.entries.map((e) => (
          <li key={e.id} className="grid gap-1 px-4 py-3 sm:grid-cols-[9rem_14rem_1fr]">
            <time dateTime={e.createdAt} className="text-xs text-muted-foreground tabular-nums">
              {timeFormat.format(new Date(e.createdAt))} UTC
            </time>
            <span className="font-mono text-xs">{e.action}</span>
            <span className="min-w-0 truncate font-mono text-xs text-muted-foreground">
              {e.entityType}/{e.entityId}
              {e.actorUserId ? ` · by ${e.actorUserId.slice(0, 8)}` : " · system"}
            </span>
          </li>
        ))}
      </ol>
      <div className="mt-4 flex gap-3">
        {cursor && (
          <Button asChild variant="outline">
            <Link href="/admin/audit">Newest</Link>
          </Button>
        )}
        {page.nextCursor && (
          <Button asChild variant="outline">
            <Link href={`/admin/audit?cursor=${encodeURIComponent(page.nextCursor)}`}>Older</Link>
          </Button>
        )}
      </div>
    </>
  );
}

export default function AdminAuditPage(props: Props) {
  return (
    <>
      <PageHeader title="Audit log" description="Publishing, role changes and account deletions, newest first." />
      <Suspense
        fallback={
          <>
            <LoadingStatus label="Loading the audit log" />
            <CardListSkeleton count={1} />
          </>
        }
      >
        <AuditTable {...props} />
      </Suspense>
    </>
  );
}
