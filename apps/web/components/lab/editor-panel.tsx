"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { FilePlus2, Play, Save, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { YamlEditor, yamlDiagnostics } from "@/components/lab/yaml-editor";
import { bff } from "@/lib/bff-client";
import { publishCommand } from "@/lib/lab-bus";
import { cn } from "@/lib/utils";

export interface LabFile {
  path: string;
  content: string;
  sizeBytes: number;
  updatedAt: string;
}

interface CommandResult {
  requestId: string;
  input: string;
  stdout: string;
  stderr: string;
  exitCode: number;
  mutated: boolean;
}

const MAX_FILES = 20;
const MAX_BYTES = 65536;
const NAME = /^[a-z0-9][a-z0-9._-]{0,62}\.ya?ml$/;

const STARTER = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 1
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
    spec:
      containers:
        - name: web
          image: nginx:1.27
`;

const bytes = (s: string) => new TextEncoder().encode(s).length;

/**
 * The lab's YAML editor: virtual files under manifests/, saved with the lab, which kubectl
 * apply -f / create -f / delete -f read. Apply saves the file and runs kubectl apply on it; the
 * command shows up in the terminal like any other.
 */
export function EditorPanel({ labId, active, initial }: { labId: string; active: boolean; initial: LabFile[] }) {
  const router = useRouter();
  const [files, setFiles] = useState<LabFile[]>(initial);
  const [selected, setSelected] = useState<string | null>(initial[0]?.path ?? null);
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const [newName, setNewName] = useState("");
  const [result, setResult] = useState<CommandResult | null>(null);

  const file = files.find((f) => f.path === selected) ?? null;
  const text = file ? (drafts[file.path] ?? file.content) : "";
  const dirty = file !== null && drafts[file.path] !== undefined && drafts[file.path] !== file.content;
  const problems = file ? yamlDiagnostics(text).filter((d) => d.severity === "error").length : 0;

  async function save(): Promise<LabFile | null> {
    if (!file) return null;
    if (!dirty) return file;
    if (bytes(text) > MAX_BYTES) {
      toast.error("This file is too large", { description: "Files can be at most 64 KiB." });
      return null;
    }
    const res = await bff<LabFile>(`labs/${labId}/files`, {
      method: "PUT",
      body: { path: file.path, content: text, baseUpdatedAt: file.updatedAt },
    });
    if (!res.ok) {
      if (res.code === "STALE_VERSION") {
        toast.error("This file changed in another tab", {
          description: "Reload it to see the latest version. Your edits here aren't saved.",
          action: { label: "Reload", onClick: () => reload(file.path) },
        });
      } else {
        toast.error("Couldn't save the file", { description: res.message });
      }
      return null;
    }
    setFiles((fs) => fs.map((f) => (f.path === res.data.path ? res.data : f)));
    setDrafts((d) => {
      const { [res.data.path]: _, ...rest } = d;
      return rest;
    });
    return res.data;
  }

  async function reload(path: string) {
    const res = await bff<LabFile[]>(`labs/${labId}/files`);
    if (!res.ok) {
      toast.error("Couldn't load the files", { description: res.message });
      return;
    }
    setFiles(res.data);
    setDrafts((d) => {
      const { [path]: _, ...rest } = d;
      return rest;
    });
  }

  async function onSave() {
    if (busy) return;
    setBusy(true);
    try {
      if (await save()) toast.success(`Saved ${file?.path}`);
    } finally {
      setBusy(false);
    }
  }

  async function onApply() {
    if (busy || !file) return;
    setBusy(true);
    try {
      const saved = await save();
      if (!saved) return;
      const input = `kubectl apply -f ${saved.path}`;
      const requestId = crypto.randomUUID();
      const res = await bff<CommandResult>(`labs/${labId}/commands`, { method: "POST", body: { requestId, input } });
      if (!res.ok) {
        toast.error("Couldn't run kubectl apply", { description: res.message });
        return;
      }
      setResult(res.data);
      publishCommand({ labId, ...res.data, requestId });
      if (res.data.mutated) router.refresh();
    } finally {
      setBusy(false);
    }
  }

  async function onCreate(e: FormEvent) {
    e.preventDefault();
    const name = newName.trim().toLowerCase();
    const fileName = /\.ya?ml$/.test(name) ? name : `${name}.yaml`;
    if (!NAME.test(fileName)) {
      toast.error("Choose another name", {
        description: "Use lowercase letters, digits, '.', '_' or '-', ending in .yaml.",
      });
      return;
    }
    const path = `manifests/${fileName}`;
    if (files.some((f) => f.path === path)) {
      setSelected(path);
      setNewName("");
      return;
    }
    setBusy(true);
    try {
      const res = await bff<LabFile>(`labs/${labId}/files`, { method: "PUT", body: { path, content: STARTER } });
      if (!res.ok) {
        toast.error("Couldn't create the file", { description: res.message });
        return;
      }
      setFiles((fs) => [...fs, res.data].sort((a, b) => a.path.localeCompare(b.path)));
      setSelected(path);
      setNewName("");
    } finally {
      setBusy(false);
    }
  }

  async function onDelete() {
    if (!file || busy) return;
    if (!window.confirm(`Delete ${file.path}? Objects you already applied stay in the cluster.`)) return;
    setBusy(true);
    try {
      const res = await bff<void>(`labs/${labId}/files?path=${encodeURIComponent(file.path)}`, { method: "DELETE" });
      if (!res.ok) {
        toast.error("Couldn't delete the file", { description: res.message });
        return;
      }
      const rest = files.filter((f) => f.path !== file.path);
      setFiles(rest);
      setSelected(rest[0]?.path ?? null);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="grid gap-3 md:grid-cols-[13rem_minmax(0,1fr)]">
      <div className="flex flex-col gap-3">
        <nav aria-label="Lab files">
          {files.length === 0 ? (
            <p className="text-sm text-muted-foreground">No files yet. Create one to write a manifest.</p>
          ) : (
            <ul className="grid gap-1">
              {files.map((f) => {
                const changed = drafts[f.path] !== undefined && drafts[f.path] !== f.content;
                return (
                  <li key={f.path}>
                    <button
                      type="button"
                      onClick={() => setSelected(f.path)}
                      aria-current={f.path === selected ? "true" : undefined}
                      className={cn(
                        "flex w-full items-center justify-between gap-2 rounded-md px-2 py-1.5 text-left font-mono text-xs",
                        f.path === selected ? "bg-primary-soft text-foreground" : "hover:bg-hover",
                      )}
                    >
                      <span className="truncate">{f.path.replace(/^manifests\//, "")}</span>
                      {changed && <span className="text-warning">unsaved</span>}
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
        </nav>
        {active && files.length < MAX_FILES && (
          <form onSubmit={onCreate} className="flex gap-1.5">
            <Input
              aria-label="New file name"
              placeholder="web.yaml"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              className="h-8 font-mono text-xs"
              maxLength={67}
            />
            <Button
              type="submit"
              size="sm"
              variant="outline"
              disabled={busy || !newName.trim()}
              aria-label="Create file"
            >
              <FilePlus2 aria-hidden />
            </Button>
          </form>
        )}
        <p className="text-xs text-muted-foreground">
          Files live in <code className="font-mono">manifests/</code>. Use them with{" "}
          <code className="font-mono">kubectl apply -f</code>, <code className="font-mono">create -f</code> or{" "}
          <code className="font-mono">delete -f</code>.
        </p>
      </div>

      {file ? (
        <div className="flex min-w-0 flex-col gap-2">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="font-mono text-sm font-medium">{file.path}</h3>
            {dirty && <Badge variant="secondary">Unsaved</Badge>}
            {problems > 0 && (
              <Badge variant="destructive">
                {problems} YAML {problems === 1 ? "error" : "errors"}
              </Badge>
            )}
            {active && (
              <div className="ml-auto flex gap-1.5">
                <Button size="sm" variant="ghost" onClick={onDelete} disabled={busy} aria-label={`Delete ${file.path}`}>
                  <Trash2 aria-hidden />
                </Button>
                <Button size="sm" variant="outline" onClick={onSave} disabled={busy || !dirty}>
                  <Save aria-hidden /> Save
                </Button>
                <Button size="sm" onClick={onApply} disabled={busy}>
                  <Play aria-hidden /> Apply
                </Button>
              </div>
            )}
          </div>
          <div className="h-[min(55svh,28rem)]">
            <YamlEditor
              key={file.path}
              label={`Editor for ${file.path}`}
              value={text}
              readOnly={!active}
              onChange={(v) => setDrafts((d) => ({ ...d, [file.path]: v }))}
              onSave={onSave}
            />
          </div>
          <p className="text-xs text-muted-foreground">
            Tab indents. To leave the editor with the keyboard, press Escape, then Tab. Ctrl+S (⌘S) saves.
          </p>
          {result && (
            <div
              role="status"
              className="rounded-lg bg-terminal p-3 font-mono text-xs whitespace-pre-wrap text-terminal-foreground"
            >
              <div className="text-terminal-muted">$ {result.input}</div>
              {result.stdout && <div>{result.stdout.replace(/\n$/, "")}</div>}
              {result.stderr && <div className="text-terminal-error">{result.stderr.replace(/\n$/, "")}</div>}
            </div>
          )}
        </div>
      ) : (
        <p className="text-sm text-muted-foreground">
          Pick a file, or create one. Manifests you write here are saved with your lab.
        </p>
      )}
    </div>
  );
}
