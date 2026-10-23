"use client";

import { useEffect } from "react";

/** Toggles html.reduce-motion, which globals.css uses to turn off animations and transitions. */
export function ReducedMotion({ enabled }: { enabled: boolean }) {
  useEffect(() => {
    document.documentElement.classList.toggle("reduce-motion", enabled);
  }, [enabled]);
  return null;
}
