import "server-only";
import type { Schemas } from "@kubedojo/api-client";
import { apiClient } from "@/lib/api/server";
import { auth } from "@/lib/auth/config";

export type Lab = Schemas["Lab"];
export type Snapshot = Schemas["Snapshot"];
export type SimEvent = Schemas["SimEvent"];

const LAB_ID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

/** Whether s looks like a lab ID; anything else is a 404 without asking the API. */
export function isLabId(s: string): boolean {
  return LAB_ID.test(s);
}

async function subject(): Promise<string> {
  const session = await auth();
  if (!session?.subject) throw new Error("not signed in");
  return session.subject;
}

/** The signed-in learner's lab, or null if it doesn't exist or belongs to someone else. */
export async function getLab(labId: string): Promise<Lab | null> {
  const { data, response } = await apiClient(await subject()).GET("/api/v1/labs/{labId}", {
    params: { path: { labId } },
  });
  if (response.status === 404) return null;
  if (!data) throw new Error(`lab request failed (${response.status})`);
  return data.data;
}

/** Every simulated resource in the lab at its current version. */
export async function getSnapshot(labId: string): Promise<Snapshot | null> {
  const { data, response } = await apiClient(await subject()).GET("/api/v1/labs/{labId}/snapshot", {
    params: { path: { labId } },
  });
  if (response.status === 404) return null;
  if (!data) throw new Error(`snapshot request failed (${response.status})`);
  return data.data;
}

/** The lab's most recent simulated events, newest last. */
export async function getRecentEvents(labId: string, limit = 100): Promise<SimEvent[]> {
  const client = apiClient(await subject());
  const events: SimEvent[] = [];
  let cursor: string | undefined;
  // Events are oldest first; walk the pages (bounded) and keep the tail.
  for (let page = 0; page < 10; page++) {
    const { data, response } = await client.GET("/api/v1/labs/{labId}/events", {
      params: { path: { labId }, query: { limit: 100, ...(cursor ? { cursor } : {}) } },
    });
    if (!data) throw new Error(`events request failed (${response.status})`);
    events.push(...data.data);
    if (!data.meta.nextCursor) break;
    cursor = data.meta.nextCursor;
  }
  return events.slice(-limit);
}

export type CommandResult = Schemas["CommandResult"];

/** The lab's most recent commands, oldest first (Secret values are redacted in history). */
export async function getCommandHistory(labId: string, limit = 50): Promise<CommandResult[]> {
  const { data, response } = await apiClient(await subject()).GET("/api/v1/labs/{labId}/commands", {
    params: { path: { labId }, query: { limit } },
  });
  if (!data) throw new Error(`command history request failed (${response.status})`);
  return [...data.data].reverse();
}

/** The learner's active labs, most recently used first. */
export async function listActiveLabs(limit = 5): Promise<Lab[]> {
  const { data, response } = await apiClient(await subject()).GET("/api/v1/labs", {
    params: { query: { state: "active", limit } },
  });
  if (!data) throw new Error(`lab list request failed (${response.status})`);
  return data.data;
}
