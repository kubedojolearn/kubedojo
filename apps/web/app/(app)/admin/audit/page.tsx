import type { Metadata } from "next";
import { ScrollText } from "lucide-react";
import { EmptyState } from "@/components/feedback/states";
import { PageHeader } from "@/components/layout/page-header";

export const metadata: Metadata = { title: "Audit log" };

export default function AdminAuditPage() {
  return (
    <>
      <PageHeader title="Audit log" description="Every publishing and role change, newest first." />
      <EmptyState icon={ScrollText} title="No audit entries yet" description="Admin actions will be recorded here." />
    </>
  );
}
