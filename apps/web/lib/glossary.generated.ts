// Generated from content/glossary.yaml by scripts/generate-glossary.mjs. Do not edit.

export interface GlossaryTerm {
  id: string;
  term: string;
  definition: string;
  docsUrl?: string;
}

export const glossary: GlossaryTerm[] = [
  {
    id: "api-server",
    term: "API server",
    definition:
      "The control-plane component every request goes through. It validates requests and stores objects in etcd.",
    docsUrl: "https://kubernetes.io/docs/concepts/architecture/#kube-apiserver",
  },
  {
    id: "cluster",
    term: "Cluster",
    definition: "A set of nodes that run containerized applications, managed by a control plane.",
    docsUrl: "https://kubernetes.io/docs/concepts/architecture/",
  },
  {
    id: "configmap",
    term: "ConfigMap",
    definition:
      "An object that stores non-secret configuration as key-value pairs, for Pods to read as environment variables or files.",
    docsUrl: "https://kubernetes.io/docs/concepts/configuration/configmap/",
  },
  {
    id: "container",
    term: "Container",
    definition: "A packaged, isolated process started from an image. Kubernetes runs containers inside Pods.",
  },
  {
    id: "control-plane",
    term: "Control plane",
    definition:
      "The components that decide what should run in the cluster and keep it that way, such as the API server, etcd, the scheduler and controllers.",
    docsUrl: "https://kubernetes.io/docs/concepts/architecture/#control-plane-components",
  },
  {
    id: "controller",
    term: "Controller",
    definition:
      "A loop that watches the cluster, compares the actual state with the desired state, and acts to close the gap.",
    docsUrl: "https://kubernetes.io/docs/concepts/architecture/controller/",
  },
  {
    id: "deployment",
    term: "Deployment",
    definition:
      "An object that keeps a set number of identical Pods running and replaces them gradually when you change their template.",
    docsUrl: "https://kubernetes.io/docs/concepts/workloads/controllers/deployment/",
  },
  {
    id: "desired-state",
    term: "Desired state",
    definition:
      "What you have asked Kubernetes for, stored in objects' spec fields. Controllers work to make the actual state match it.",
  },
  {
    id: "etcd",
    term: "etcd",
    definition: "The consistent key-value database where Kubernetes stores all cluster state.",
    docsUrl: "https://kubernetes.io/docs/concepts/architecture/#etcd",
  },
  {
    id: "event",
    term: "Event",
    definition:
      "A short record of something that happened to an object, such as being scheduled or failing to pull an image. Shown by kubectl describe and kubectl get events.",
  },
  {
    id: "kubectl",
    term: "kubectl",
    definition: "The command-line tool for talking to a Kubernetes cluster's API server.",
    docsUrl: "https://kubernetes.io/docs/reference/kubectl/",
  },
  {
    id: "label",
    term: "Label",
    definition:
      "A key-value pair attached to an object. Selectors use labels to pick out groups of objects, such as the Pods behind a Service.",
    docsUrl: "https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/",
  },
  {
    id: "namespace",
    term: "Namespace",
    definition:
      "A way to divide one cluster into named groups of objects. Names only have to be unique within a namespace.",
    docsUrl: "https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/",
  },
  {
    id: "node",
    term: "Node",
    definition:
      "A machine, physical or virtual, that runs Pods. Each node runs a kubelet that starts and reports on its Pods.",
    docsUrl: "https://kubernetes.io/docs/concepts/architecture/nodes/",
  },
  {
    id: "pod",
    term: "Pod",
    definition:
      "The smallest unit Kubernetes runs. One or more containers that share a network address and are scheduled together.",
    docsUrl: "https://kubernetes.io/docs/concepts/workloads/pods/",
  },
  {
    id: "replicaset",
    term: "ReplicaSet",
    definition:
      "An object that keeps a set number of identical Pods running. Deployments create and manage ReplicaSets for you.",
    docsUrl: "https://kubernetes.io/docs/concepts/workloads/controllers/replicaset/",
  },
  {
    id: "scheduler",
    term: "Scheduler",
    definition:
      "The control-plane component that picks a node for each new Pod, based on the Pod's requirements and each node's free capacity.",
    docsUrl: "https://kubernetes.io/docs/concepts/scheduling-eviction/kube-scheduler/",
  },
  {
    id: "selector",
    term: "Selector",
    definition: "A query over labels, such as app=web, used to pick a group of objects.",
  },
  {
    id: "service",
    term: "Service",
    definition: "A stable network address for a group of Pods chosen by a label selector.",
    docsUrl: "https://kubernetes.io/docs/concepts/services-networking/service/",
  },
];

export const glossaryById = new Map(glossary.map((t) => [t.id, t]));
