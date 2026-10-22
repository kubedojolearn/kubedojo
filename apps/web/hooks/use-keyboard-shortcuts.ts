"use client";

import { useEffect, useRef } from "react";

export interface Shortcut {
  /** Key sequence, e.g. ["?"] or ["g", "d"]. Compared against KeyboardEvent.key. */
  keys: string[];
  description: string;
  action: () => void;
}

const SEQUENCE_TIMEOUT_MS = 1000;

/** True when the event target accepts text, so shortcuts must not fire. */
export function isTypingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  if (target.isContentEditable) return true;
  const tag = target.tagName;
  if (tag === "TEXTAREA" || tag === "SELECT") return true;
  if (tag === "INPUT") {
    const type = (target as HTMLInputElement).type;
    return !["button", "checkbox", "radio", "submit", "reset", "range", "color", "file"].includes(type);
  }
  // xterm.js and Monaco render into elements with these roles.
  return target.closest('[role="textbox"], .xterm, .monaco-editor') !== null;
}

/**
 * Single-key and two-key sequence shortcuts. Ignored while typing, with modifier keys held
 * (except Shift, needed for "?"), or when another handler already called preventDefault.
 */
export function useKeyboardShortcuts(shortcuts: Shortcut[]) {
  const latest = useRef(shortcuts);
  useEffect(() => {
    latest.current = shortcuts;
  });

  useEffect(() => {
    let buffer: string[] = [];
    let timer: ReturnType<typeof setTimeout> | undefined;

    function onKeyDown(e: KeyboardEvent) {
      if (e.defaultPrevented || e.altKey || e.ctrlKey || e.metaKey || isTypingTarget(e.target)) return;
      buffer = [...buffer, e.key].slice(-2);
      clearTimeout(timer);
      timer = setTimeout(() => (buffer = []), SEQUENCE_TIMEOUT_MS);

      for (const s of latest.current) {
        const tail = buffer.slice(-s.keys.length);
        if (tail.length === s.keys.length && tail.every((k, i) => k === s.keys[i])) {
          e.preventDefault();
          buffer = [];
          s.action();
          return;
        }
      }
    }

    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("keydown", onKeyDown);
      clearTimeout(timer);
    };
  }, []);
}
