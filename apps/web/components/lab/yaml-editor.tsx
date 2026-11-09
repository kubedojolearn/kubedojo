"use client";

import { useEffect, useRef } from "react";
import { defaultKeymap, history, historyKeymap, indentWithTab } from "@codemirror/commands";
import { yaml } from "@codemirror/lang-yaml";
import { bracketMatching, HighlightStyle, indentOnInput, syntaxHighlighting } from "@codemirror/language";
import { linter, lintGutter, type Diagnostic } from "@codemirror/lint";
import { EditorState } from "@codemirror/state";
import {
  drawSelection,
  EditorView,
  highlightActiveLine,
  highlightActiveLineGutter,
  keymap,
  lineNumbers,
  placeholder,
} from "@codemirror/view";
import { tags } from "@lezer/highlight";
import { parseAllDocuments } from "yaml";
import { terminal } from "@/lib/design-tokens";

/** YAML syntax problems, found in the browser as the learner types (the API validates again). */
export function yamlDiagnostics(text: string): Diagnostic[] {
  const out: Diagnostic[] = [];
  for (const doc of parseAllDocuments(text)) {
    for (const problem of [...doc.errors, ...doc.warnings]) {
      const [from, to] = problem.pos;
      out.push({
        from: Math.min(from, text.length),
        to: Math.min(Math.max(to, from), text.length),
        severity: problem.name === "YAMLWarning" ? "warning" : "error",
        message: problem.message.split("\n")[0] ?? problem.message,
      });
    }
  }
  return out;
}

// Dark in both app themes, like the terminal it sits next to.
const theme = EditorView.theme(
  {
    "&": { backgroundColor: terminal.background, color: terminal.foreground, fontSize: "13px", height: "100%" },
    ".cm-content": { fontFamily: "var(--font-mono)", caretColor: terminal.cursor },
    ".cm-cursor, .cm-dropCursor": { borderLeftColor: terminal.cursor },
    "&.cm-focused": { outline: "2px solid var(--color-ring)", outlineOffset: "2px" },
    ".cm-gutters": { backgroundColor: terminal.background, color: terminal.brightBlack, border: "none" },
    ".cm-activeLine, .cm-activeLineGutter": { backgroundColor: "#ffffff0d" },
    "&.cm-focused .cm-selectionBackground, .cm-selectionBackground, ::selection": {
      backgroundColor: terminal.selectionBackground,
    },
    ".cm-placeholder": { color: terminal.brightBlack },
    ".cm-scroller": { overflow: "auto" },
    ".cm-tooltip": { backgroundColor: "#11151b", color: terminal.foreground, border: "1px solid #2a313b" },
  },
  { dark: true },
);

const highlight = HighlightStyle.define([
  { tag: [tags.propertyName, tags.definition(tags.propertyName)], color: terminal.cyan },
  { tag: [tags.string, tags.special(tags.string)], color: terminal.green },
  { tag: [tags.number, tags.bool, tags.null], color: terminal.yellow },
  { tag: [tags.comment, tags.lineComment], color: terminal.brightBlack, fontStyle: "italic" },
  { tag: [tags.meta, tags.keyword, tags.typeName], color: terminal.magenta },
  { tag: [tags.punctuation, tags.separator], color: terminal.foreground },
]);

/**
 * A YAML editor (CodeMirror 6). Tab indents; press Escape, then Tab, to move focus on, as
 * CodeMirror documents for keyboard users. Ctrl/Cmd+S calls onSave.
 */
export function YamlEditor({
  value,
  onChange,
  onSave,
  readOnly,
  label,
}: {
  value: string;
  onChange: (v: string) => void;
  onSave: () => void;
  readOnly: boolean;
  label: string;
}) {
  const host = useRef<HTMLDivElement>(null);
  const view = useRef<EditorView | null>(null);
  // The latest callbacks, read by the editor's listeners without recreating it.
  const callbacks = useRef({ onChange, onSave });
  useEffect(() => {
    callbacks.current = { onChange, onSave };
  });

  useEffect(() => {
    if (!host.current) return;
    const v = new EditorView({
      parent: host.current,
      state: EditorState.create({
        doc: value,
        extensions: [
          lineNumbers(),
          highlightActiveLineGutter(),
          highlightActiveLine(),
          drawSelection(),
          history(),
          indentOnInput(),
          bracketMatching(),
          yaml(),
          syntaxHighlighting(highlight),
          linter((ev) => yamlDiagnostics(ev.state.doc.toString()), { delay: 300 }),
          lintGutter(),
          placeholder("apiVersion: apps/v1\nkind: Deployment\n…"),
          EditorState.tabSize.of(2),
          EditorState.readOnly.of(readOnly),
          EditorView.contentAttributes.of({ "aria-label": label }),
          keymap.of([
            {
              key: "Mod-s",
              preventDefault: true,
              run: () => {
                callbacks.current.onSave();
                return true;
              },
            },
            indentWithTab,
            ...defaultKeymap,
            ...historyKeymap,
          ]),
          EditorView.updateListener.of((u) => {
            if (u.docChanged) callbacks.current.onChange(u.state.doc.toString());
          }),
          theme,
        ],
      }),
    });
    view.current = v;
    return () => {
      v.destroy();
      view.current = null;
    };
    // The editor is recreated only when its file or mode changes (the key on the parent);
    // value changes from typing flow out through onChange, not back in.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [readOnly, label]);

  // A value replaced from outside (reload after a conflict) is pushed into the editor.
  useEffect(() => {
    const v = view.current;
    if (v && v.state.doc.toString() !== value) {
      v.dispatch({ changes: { from: 0, to: v.state.doc.length, insert: value } });
    }
  }, [value]);

  return <div ref={host} className="h-full min-h-0 overflow-hidden rounded-lg" />;
}
