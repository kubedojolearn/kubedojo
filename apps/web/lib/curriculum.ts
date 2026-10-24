/**
 * The planned V1 curriculum, mirroring docs/product/course-outline.md. Used by marketing pages
 * until the catalog API serves published lessons (Part 07). Contains no learner data.
 */
export type Difficulty = "beginner" | "intermediate";

export interface PlannedCourse {
  slug: string;
  title: string;
  summary: string;
  difficulty: Difficulty;
  lessons: Array<{ slug: string; title: string }>;
}

export const plannedCourses: PlannedCourse[] = [
  {
    slug: "fundamentals",
    title: "Kubernetes fundamentals",
    summary: "How a cluster is put together, how to find your way around it, and how to describe what you want.",
    difficulty: "beginner",
    lessons: [
      { slug: "cluster-architecture", title: "How a cluster is put together" },
      { slug: "namespace-navigation", title: "Finding your way with namespaces" },
      { slug: "listing-resources", title: "Listing and reading resources" },
      { slug: "labels-and-selectors", title: "Labels and selectors" },
      { slug: "declarative-vs-imperative", title: "Imperative commands vs. declarative manifests" },
    ],
  },
  {
    slug: "workloads",
    title: "Pods and workloads",
    summary: "Run, inspect, scale and roll out applications with Pods and Deployments.",
    difficulty: "beginner",
    lessons: [
      { slug: "launch-a-pod", title: "Launch your first Pod" },
      { slug: "inspect-a-pod", title: "Inspect a running Pod" },
      { slug: "troubleshoot-pending-pod", title: "Why is my Pod Pending?" },
      { slug: "create-a-deployment", title: "Create a Deployment" },
      { slug: "scaling-deployments", title: "Scale a Deployment" },
      { slug: "rollout-and-rollback", title: "Roll out and roll back" },
    ],
  },
  {
    slug: "networking",
    title: "Networking",
    summary: "Give Pods a stable address with Services, and work out why traffic isn't arriving.",
    difficulty: "beginner",
    lessons: [
      { slug: "service-basics", title: "What a Service is" },
      { slug: "expose-a-deployment", title: "Expose a Deployment" },
      { slug: "selectors-and-endpoints", title: "How Services find Pods" },
      { slug: "clusterip-vs-nodeport", title: "ClusterIP vs. NodePort" },
      { slug: "debug-missing-endpoints", title: "Debug a Service with no endpoints" },
    ],
  },
  {
    slug: "configuration",
    title: "Configuration",
    summary: "Keep configuration out of images with ConfigMaps, edit manifests safely, and understand Secrets.",
    difficulty: "intermediate",
    lessons: [
      { slug: "configmaps", title: "Store configuration in a ConfigMap" },
      { slug: "editing-manifests", title: "Edit and re-apply a manifest" },
      { slug: "environment-from-configmaps", title: "Feed environment variables from a ConfigMap" },
      { slug: "secrets-awareness", title: "Secrets are not encryption" },
    ],
  },
  {
    slug: "cluster-operations",
    title: "Cluster operations",
    summary: "Inspect nodes, scopes, labels and scheduling, and read the events that explain what happened.",
    difficulty: "intermediate",
    lessons: [
      { slug: "inspect-nodes", title: "Inspect Nodes" },
      { slug: "namespaces-and-scope", title: "Namespaced vs. cluster-scoped" },
      { slug: "labels-and-annotations", title: "Labels vs. annotations" },
      { slug: "requests-limits-scheduling", title: "Requests, limits and scheduling" },
      { slug: "events-and-diagnostics", title: "Read events like a detective" },
    ],
  },
  {
    slug: "troubleshooting",
    title: "Troubleshooting",
    summary: "Fix the failures you'll meet in real clusters, from image pull errors to a full outage.",
    difficulty: "intermediate",
    lessons: [
      { slug: "image-pull-failure", title: "Fix an image pull failure" },
      { slug: "crashloopbackoff", title: "Fix a CrashLoopBackOff" },
      { slug: "wrong-labels", title: "A Deployment whose Pods nobody can reach" },
      { slug: "replica-mismatch", title: "Desired 4, ready 1" },
      { slug: "capstone-recovery", title: "Capstone: bring the shop back online" },
    ],
  },
];

export const totalPlannedLessons = plannedCourses.reduce((n, c) => n + c.lessons.length, 0);
