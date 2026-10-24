import type { Metadata } from "next";
import Link from "next/link";
import { Suspense } from "react";
import { AdminsOnly } from "@/components/admin/admins-only";
import { LoadingStatus } from "@/components/feedback/loading";
import { PageHeader } from "@/components/layout/page-header";
import { getAdminContent } from "@/lib/admin";

export const metadata: Metadata = { title: "Admin" };

async function Overview() {
  const lessons = await getAdminContent();
  if (!lessons) return <AdminsOnly />;
  const versions = lessons.flatMap((l) => l.versions);
  const stats = [
    { label: "Lessons", value: lessons.length },
    { label: "Published", value: lessons.filter((l) => l.currentVersionId).length },
    { label: "Draft versions", value: versions.filter((v) => v.status === "draft").length },
  ];
  return (
    <div className="grid gap-8">
      <dl className="grid max-w-xl grid-cols-3 divide-x rounded-lg border bg-card">
        {stats.map((s) => (
          <div key={s.label} className="p-5">
            <dt className="text-xs text-muted-foreground">{s.label}</dt>
            <dd className="mt-1 text-2xl font-bold tabular-nums">{s.value}</dd>
          </div>
        ))}
      </dl>
      <ul className="grid max-w-xl gap-2 text-sm">
        <li>
          <Link href="/admin/content" className="text-link underline-offset-4 hover:underline">
            Content versions
          </Link>{" "}
          <span className="text-muted-foreground">— every lesson and what&apos;s published</span>
        </li>
        <li>
          <Link href="/admin/audit" className="text-link underline-offset-4 hover:underline">
            Audit log
          </Link>{" "}
          <span className="text-muted-foreground">— who changed what, and when</span>
        </li>
      </ul>
    </div>
  );
}

export default function AdminPage() {
  return (
    <>
      <PageHeader title="Admin" description="Publishing and oversight tools for KubeDojo maintainers." />
      <Suspense fallback={<LoadingStatus label="Loading" />}>
        <Overview />
      </Suspense>
    </>
  );
}
