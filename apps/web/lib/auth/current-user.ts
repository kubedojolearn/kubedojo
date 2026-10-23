import "server-only";
import { cache } from "react";
import type { Schemas } from "@kubedojo/api-client";
import { apiClient } from "@/lib/api/server";
import { auth } from "@/lib/auth/config";

export type CurrentUser = Schemas["Me"];

/**
 * The signed-in user as the API sees it, or null. Profile and role always come from the API
 * (which reads PostgreSQL), never from the session cookie. Reads request data, so call it inside
 * a <Suspense> boundary (Cache Components). Deduplicated per request.
 */
export const getCurrentUser = cache(async (): Promise<CurrentUser | null> => {
  const session = await auth();
  if (!session?.subject) return null;
  const { data, response } = await apiClient(session.subject).GET("/api/v1/users/me");
  if (response.status === 401) return null;
  if (!data) throw new Error(`users/me failed with ${response.status}`);
  return data.data;
});
