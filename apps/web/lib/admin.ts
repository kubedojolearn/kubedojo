import "server-only";
import type { Schemas } from "@kubedojo/api-client";
import { apiClient } from "@/lib/api/server";
import { auth } from "@/lib/auth/config";

export type AdminLessonContent = Schemas["AdminLessonContent"];
export type AuditEntry = Schemas["AuditEntry"];

/** null when the user isn't signed in or isn't an admin (the API checks the role on every call). */
export async function getAdminContent(): Promise<AdminLessonContent[] | null> {
  const session = await auth();
  if (!session?.subject) return null;
  const { data, response } = await apiClient(session.subject).GET("/api/v1/admin/content");
  if (response.status === 401 || response.status === 403) return null;
  if (!data) throw new Error(`admin content request failed (${response.status})`);
  return data.data;
}

export async function getAuditLog(
  cursor?: string,
): Promise<{ entries: AuditEntry[]; nextCursor: string | null } | null> {
  const session = await auth();
  if (!session?.subject) return null;
  const { data, response } = await apiClient(session.subject).GET("/api/v1/admin/audit", {
    params: { query: { limit: 50, ...(cursor ? { cursor } : {}) } },
  });
  if (response.status === 401 || response.status === 403) return null;
  if (!data) throw new Error(`audit request failed (${response.status})`);
  return { entries: data.data, nextCursor: data.meta.nextCursor ?? null };
}
