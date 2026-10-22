import type { Metadata } from "next";
import { InDevelopmentBadge } from "@/components/feedback/in-development";
import { PageHeader } from "@/components/layout/page-header";
import { AppearanceSettings } from "@/components/settings/appearance-settings";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export const metadata: Metadata = { title: "Settings" };

function Unavailable() {
  return <InDevelopmentBadge label="Available after sign-in" />;
}

export default function SettingsPage() {
  return (
    <>
      <PageHeader title="Settings" />
      <div className="grid max-w-3xl gap-6">
        <Card>
          <CardHeader>
            <CardTitle>
              <h2>Appearance</h2>
            </CardTitle>
            <CardDescription>Saved in this browser.</CardDescription>
          </CardHeader>
          <CardContent>
            <AppearanceSettings />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex flex-wrap items-center gap-2">
              <h2>Profile</h2> <Unavailable />
            </CardTitle>
            <CardDescription>Your handle and display name, shown on your profile and the leaderboard.</CardDescription>
          </CardHeader>
          <CardContent>
            <fieldset disabled className="grid gap-4 opacity-70">
              <legend className="sr-only">Profile (not available yet)</legend>
              <div className="grid gap-2">
                <Label htmlFor="handle">Handle</Label>
                <Input id="handle" name="handle" placeholder="your-github-username" />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="display-name">Display name</Label>
                <Input id="display-name" name="displayName" />
              </div>
            </fieldset>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex flex-wrap items-center gap-2">
              <h2>Privacy</h2> <Unavailable />
            </CardTitle>
            <CardDescription>
              Profiles are private and you are off the leaderboard until you choose otherwise. Learning analytics can be
              turned off at any time.
            </CardDescription>
          </CardHeader>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex flex-wrap items-center gap-2">
              <h2>Your data</h2> <Unavailable />
            </CardTitle>
            <CardDescription>Export everything KubeDojo stores about you, or delete your account.</CardDescription>
          </CardHeader>
        </Card>
      </div>
    </>
  );
}
