import { Shield } from "lucide-react";
import { EmptyState } from "@/components/feedback/states";

export function AdminsOnly() {
  return (
    <EmptyState
      icon={Shield}
      title="Admins only"
      description="This page is for KubeDojo maintainers. Roles are stored on the server and changed with the operator CLI."
    />
  );
}
