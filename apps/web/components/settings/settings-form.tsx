"use client";

import { useRouter } from "next/navigation";
import { useTheme } from "next-themes";
import { useId, useMemo, useState, type FormEvent } from "react";
import { toast } from "sonner";
import type { Schemas } from "@kubedojo/api-client";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

type Me = Schemas["Me"];
type FieldErrors = Record<string, string>;

function Toggle({
  id,
  label,
  description,
  checked,
  onChange,
}: {
  id: string;
  label: string;
  description: string;
  checked: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <div className="flex items-start gap-3">
      <input
        id={id}
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        className="mt-1 size-4 accent-[var(--primary)]"
        aria-describedby={`${id}-desc`}
      />
      <div>
        <Label htmlFor={id}>{label}</Label>
        <p id={`${id}-desc`} className="text-sm text-muted-foreground">
          {description}
        </p>
      </div>
    </div>
  );
}

function FieldError({ id, message }: { id: string; message?: string }) {
  if (!message) return null;
  return (
    <p id={id} className="text-sm text-danger">
      {message}
    </p>
  );
}

/** Account settings. Saves through the BFF; the API validates and owns every value. */
export function SettingsForm({ me }: { me: Me }) {
  const router = useRouter();
  const { setTheme } = useTheme();
  const ids = useId();
  const [displayName, setDisplayName] = useState(me.displayName);
  const [handle, setHandle] = useState(me.handle);
  const [settings, setSettings] = useState(me.settings);
  const [errors, setErrors] = useState<FieldErrors>({});
  const [saving, setSaving] = useState(false);
  const timeZones = useMemo(() => {
    const zones = typeof Intl.supportedValuesOf === "function" ? Intl.supportedValuesOf("timeZone") : [];
    return zones.includes(settings.timeZone) ? zones : [settings.timeZone, ...zones];
  }, [settings.timeZone]);

  async function save(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    setErrors({});
    const body: Record<string, unknown> = { settings };
    if (displayName !== me.displayName) body.displayName = displayName;
    if (handle !== me.handle) body.handle = handle;
    try {
      const res = await fetch("/api/bff/v1/users/me", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      if (!res.ok) {
        const payload = (await res.json().catch(() => null)) as {
          error?: { message?: string; details?: Array<{ field?: string; message: string }> };
        } | null;
        const fieldErrors: FieldErrors = {};
        for (const d of payload?.error?.details ?? []) if (d.field) fieldErrors[d.field] = d.message;
        setErrors(fieldErrors);
        toast.error("Settings not saved", { description: payload?.error?.message ?? "Please try again." });
        return;
      }
      setTheme(settings.theme);
      toast.success("Settings saved");
      router.refresh();
    } catch {
      toast.error("Settings not saved", { description: "Check your connection and try again." });
    } finally {
      setSaving(false);
    }
  }

  const err = (field: string) => errors[field];
  const errId = (field: string) => `${ids}-${field.replace(/\W/g, "-")}-error`;

  return (
    <form onSubmit={save} className="grid max-w-3xl gap-6" noValidate>
      {Object.keys(errors).length > 0 && (
        <div role="alert" className="rounded-lg border border-danger/40 bg-danger/5 p-4 text-sm">
          <p className="font-medium">Please fix the highlighted fields.</p>
        </div>
      )}
      <Card>
        <CardHeader>
          <CardTitle>
            <h2>Profile</h2>
          </CardTitle>
          <CardDescription>Shown on your profile and, if you opt in, the leaderboard.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4">
          <div className="grid gap-2">
            <Label htmlFor={`${ids}-display`}>Display name</Label>
            <Input
              id={`${ids}-display`}
              value={displayName}
              maxLength={80}
              onChange={(e) => setDisplayName(e.target.value)}
              aria-invalid={Boolean(err("displayName"))}
              aria-describedby={err("displayName") ? errId("displayName") : undefined}
            />
            <FieldError id={errId("displayName")} message={err("displayName")} />
          </div>
          <div className="grid gap-2">
            <Label htmlFor={`${ids}-handle`}>Handle</Label>
            <Input
              id={`${ids}-handle`}
              value={handle}
              maxLength={39}
              autoCapitalize="none"
              spellCheck={false}
              onChange={(e) => setHandle(e.target.value)}
              aria-invalid={Boolean(err("handle"))}
              aria-describedby={`${ids}-handle-help${err("handle") ? ` ${errId("handle")}` : ""}`}
            />
            <p id={`${ids}-handle-help`} className="text-sm text-muted-foreground">
              Letters, numbers and single hyphens. Your profile address is /profile/{handle || "…"}.
            </p>
            <FieldError id={errId("handle")} message={err("handle")} />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>
            <h2>Preferences</h2>
          </CardTitle>
        </CardHeader>
        <CardContent className="grid gap-5">
          <fieldset className="grid gap-2">
            <legend className="text-sm font-medium">Theme</legend>
            <div className="flex flex-wrap gap-4">
              {(["system", "dark", "light"] as const).map((t) => (
                <label key={t} className="flex items-center gap-2 text-sm capitalize">
                  <input
                    type="radio"
                    name="theme"
                    value={t}
                    checked={settings.theme === t}
                    onChange={() => setSettings({ ...settings, theme: t })}
                  />
                  {t}
                </label>
              ))}
            </div>
          </fieldset>
          <Toggle
            id={`${ids}-motion`}
            label="Reduce motion"
            description="Turn off animations and transitions."
            checked={settings.reducedMotion}
            onChange={(v) => setSettings({ ...settings, reducedMotion: v })}
          />
          <div className="grid gap-2">
            <Label htmlFor={`${ids}-tz`}>Time zone</Label>
            <select
              id={`${ids}-tz`}
              value={settings.timeZone}
              onChange={(e) => setSettings({ ...settings, timeZone: e.target.value })}
              className="h-9 rounded-md border border-input bg-background px-2 text-small"
              aria-describedby={`${ids}-tz-help${err("settings.timeZone") ? ` ${errId("settings.timeZone")}` : ""}`}
            >
              {timeZones.map((z) => (
                <option key={z} value={z}>
                  {z}
                </option>
              ))}
            </select>
            <p id={`${ids}-tz-help`} className="text-sm text-muted-foreground">
              Streaks count calendar days in this time zone.
            </p>
            <FieldError id={errId("settings.timeZone")} message={err("settings.timeZone")} />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>
            <h2>Privacy</h2>
          </CardTitle>
          <CardDescription>Everything is private until you choose otherwise.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-5">
          <Toggle
            id={`${ids}-public`}
            label="Public profile"
            description="Anyone can see your handle, display name, XP, completed lessons and achievements."
            checked={settings.profilePublic}
            onChange={(v) => setSettings({ ...settings, profilePublic: v })}
          />
          <Toggle
            id={`${ids}-leaderboard`}
            label="Appear on the leaderboard"
            description="List your handle and XP on the public leaderboard."
            checked={settings.leaderboardOptIn}
            onChange={(v) => setSettings({ ...settings, leaderboardOptIn: v })}
          />
          <Toggle
            id={`${ids}-analytics`}
            label="Opt out of learning analytics"
            description="We record which lessons you start and finish (never what you type) to improve lessons."
            checked={settings.analyticsOptOut}
            onChange={(v) => setSettings({ ...settings, analyticsOptOut: v })}
          />
        </CardContent>
      </Card>

      <div>
        <Button type="submit" disabled={saving}>
          {saving ? "Saving…" : "Save settings"}
        </Button>
      </div>
    </form>
  );
}
