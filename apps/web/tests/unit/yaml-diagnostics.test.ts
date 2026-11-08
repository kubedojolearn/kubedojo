import { describe, expect, it } from "vitest";
import { yamlDiagnostics } from "@/components/lab/yaml-editor";

describe("yamlDiagnostics", () => {
  it("accepts valid multi-document YAML", () => {
    expect(yamlDiagnostics("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: a\n---\nkind: x\n")).toEqual([]);
  });

  it("points at a syntax error inside the text", () => {
    const text = "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n data: oops\n";
    const [d] = yamlDiagnostics(text);
    expect(d?.severity).toBe("error");
    expect(d!.from).toBeGreaterThanOrEqual(0);
    expect(d!.to).toBeLessThanOrEqual(text.length);
  });

  it("reports tab indentation", () => {
    expect(yamlDiagnostics("a:\n\tb: 1\n").some((d) => d.severity === "error")).toBe(true);
  });
});
