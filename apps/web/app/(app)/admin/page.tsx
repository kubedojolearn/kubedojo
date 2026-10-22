import type { Metadata } from "next";
import { Shield } from "lucide-react";
import { EmptyState } from "@/components/feedback/states";
import { PageHeader } from "@/components/layout/page-header";

export const metadata: Metadata = { title: "Admin" };

export default function AdminPage() {
  return (
    <>
      <PageHeader title="Admin" description="Publishing and oversight tools for KubeDojo maintainers." />
      <EmptyState
        icon={Shield}
        title="Admin tools aren't available yet"
        description="Admins will publish reviewed lesson versions and read the audit log here. Access is controlled by roles stored on the server."
      />
    </>
  );
}
