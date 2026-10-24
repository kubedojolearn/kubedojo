import "server-only";
import { cache } from "react";
import type { Schemas } from "@kubedojo/api-client";
import { apiClient } from "@/lib/api/server";
import { auth } from "@/lib/auth/config";

export type Progress = Schemas["Progress"];
export type Achievement = Schemas["Achievement"];
export type ActivityItem = Schemas["ActivityItem"];
export type PublicProfile = Schemas["PublicProfile"];

async function subject(): Promise<string> {
  const session = await auth();
  if (!session?.subject) throw new Error("not signed in");
  return session.subject;
}

/** The signed-in learner's progress (first page of lessons plus dashboard pointers). */
export const getProgress = cache(async (): Promise<Progress> => {
  const { data, response } = await apiClient(await subject()).GET("/api/v1/users/me/progress", {
    params: { query: { limit: 100 } },
  });
  if (!data) throw new Error(`progress request failed (${response.status})`);
  return data.data;
});

export async function getAchievements(): Promise<Achievement[]> {
  const { data, response } = await apiClient(await subject()).GET("/api/v1/users/me/achievements");
  if (!data) throw new Error(`achievements request failed (${response.status})`);
  return data.data;
}

export async function getRecentActivity(limit = 5): Promise<ActivityItem[]> {
  const { data, response } = await apiClient(await subject()).GET("/api/v1/users/me/activity", {
    params: { query: { limit } },
  });
  if (!data) throw new Error(`activity request failed (${response.status})`);
  return data.data;
}

/** A public profile, or null if it doesn't exist or isn't public. */
export async function getPublicProfile(handle: string): Promise<PublicProfile | null> {
  const { data, response } = await apiClient().GET("/api/v1/profiles/{handle}", { params: { path: { handle } } });
  if (response.status === 404) return null;
  if (!data) throw new Error(`profile request failed (${response.status})`);
  return data.data;
}

export type LeaderboardEntry = Schemas["LeaderboardEntry"];

/** One page of the public, opt-in leaderboard. */
export async function getLeaderboard(
  cursor?: string,
  limit = 25,
): Promise<{ entries: LeaderboardEntry[]; nextCursor: string | null }> {
  const { data, response } = await apiClient().GET("/api/v1/leaderboard", {
    params: { query: { limit, ...(cursor ? { cursor } : {}) } },
  });
  if (!data) throw new Error(`leaderboard request failed (${response.status})`);
  return { entries: data.data, nextCursor: data.meta.nextCursor ?? null };
}
