import "server-only";
import { connection } from "next/server";
import type { Schemas } from "@kubedojo/api-client";
import { apiClient } from "@/lib/api/server";

export type CourseSummary = Schemas["CourseSummary"];
export type CourseDetail = Schemas["CourseDetail"];
export type LessonDetail = Schemas["LessonDetail"];

/** Thrown when the API says the course or lesson doesn't exist or isn't published. */
export class NotPublishedError extends Error {}

/*
 * Public catalog reads. They run at request time (connection() opts out of prerendering, since
 * the API decides what is published), so pages render them inside <Suspense>. No identity is
 * sent: the catalog is public.
 */

export async function listCourses(): Promise<CourseSummary[]> {
  await connection();
  const { data, response } = await apiClient().GET("/api/v1/catalog/courses");
  if (!data) throw new Error(`catalog request failed (${response.status})`);
  return data.data;
}

export async function getCourse(slug: string): Promise<CourseDetail> {
  await connection();
  const { data, response } = await apiClient().GET("/api/v1/catalog/courses/{courseSlug}", {
    params: { path: { courseSlug: slug } },
  });
  if (response.status === 404) throw new NotPublishedError(slug);
  if (!data) throw new Error(`course request failed (${response.status})`);
  return data.data;
}

export async function getLesson(courseSlug: string, lessonSlug: string): Promise<LessonDetail> {
  await connection();
  const { data, response } = await apiClient().GET("/api/v1/catalog/courses/{courseSlug}/lessons/{lessonSlug}", {
    params: { path: { courseSlug, lessonSlug } },
  });
  if (response.status === 404) throw new NotPublishedError(`${courseSlug}/${lessonSlug}`);
  if (!data) throw new Error(`lesson request failed (${response.status})`);
  return data.data;
}

/** Slugs that can be passed to the API: anything else is a 404 without a network call. */
export function isSlug(s: string): boolean {
  return /^[a-z0-9]+(-[a-z0-9]+)*$/.test(s) && s.length <= 64;
}
