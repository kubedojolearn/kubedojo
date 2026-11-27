import type { Metadata } from "next";
import { Suspense } from "react";
import { AdminsOnly } from "@/components/admin/admins-only";
import { AdminContentTable } from "@/components/admin/content-table";
import { CardListSkeleton, LoadingStatus } from "@/components/feedback/loading";
import { PageHeader } from "@/components/layout/page-header";
import { getAdminContent } from "@/lib/admin";

export const metadata: Metadata = { title: "Content" };

async function ContentTable() {
  const lessons = await getAdminContent();
  if (!lessons) return <AdminsOnly />;
  return <AdminContentTable initialLessons={lessons} />;
}

export default function AdminContentPage() {
  return (
    <>
      <PageHeader
        title="Content"
        description={
          <>
            Every lesson and its imported versions. Lessons are written and reviewed in Git and imported with{" "}
            <code className="font-mono text-sm">make content-import</code>; validate, publish or unpublish imported
            versions here. Every change is audited.
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
