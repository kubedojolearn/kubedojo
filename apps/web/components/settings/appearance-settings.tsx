"use client";

import { Monitor, Moon, Sun } from "lucide-react";
import { useTheme } from "next-themes";
import { useSyncExternalStore } from "react";
import { cn } from "@/lib/utils";

const options = [
  { value: "system", label: "System", icon: Monitor },
  { value: "dark", label: "Dark", icon: Moon },
  { value: "light", label: "Light", icon: Sun },
] as const;

const subscribe = () => () => {};

/** Theme picker. Works today because the choice is stored in this browser (synced to the account in Part 08). */
export function AppearanceSettings() {
  const { theme, setTheme } = useTheme();
  const mounted = useSyncExternalStore(
    subscribe,
    () => true,
    () => false,
  );
  const current = mounted ? (theme ?? "system") : undefined;

  return (
    <fieldset>
      <legend className="text-sm font-medium">Theme</legend>
      <div className="mt-3 grid gap-3 sm:grid-cols-3">
        {options.map((o) => (
          <label
            key={o.value}
            className={cn(
              "flex cursor-pointer items-center gap-3 rounded-lg border p-3 text-sm transition-colors has-focus-visible:outline-2 has-focus-visible:outline-offset-2 has-focus-visible:outline-ring",
              current === o.value && "border-primary bg-primary/10",
            )}
          >
            <input
              type="radio"
              name="theme"
              value={o.value}
              checked={current === o.value}
              onChange={() => setTheme(o.value)}
              className="sr-only"
            />
            <o.icon aria-hidden className="size-4" />
            {o.label}
          </label>
        ))}
      </div>
    </fieldset>
  );
}
