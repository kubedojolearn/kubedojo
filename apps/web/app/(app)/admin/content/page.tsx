import type { Metadata } from "next";
import { FileStack } from "lucide-react";
import { EmptyState } from "@/components/feedback/states";
import { PageHeader } from "@/components/layout/page-header";

export const metadata: Metadata = { title: "Content" };

export default function AdminContentPage() {
  return (
    <>
      <PageHeader title="Content" description="Imported lesson versions and their publication status." />
      <EmptyState
        icon={FileStack}
        title="No content imported yet"
        description="Lessons are written and reviewed in the Git repository. After deployment, each new version appears here as a draft for an admin to publish."
      />
    </>
  );
}
