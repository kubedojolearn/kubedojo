import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { Suspense } from "react";
import { Lock, Trophy } from "lucide-react";
import { LoadingStatus } from "@/components/feedback/loading";
import { EmptyState } from "@/components/feedback/states";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { getCurrentUser } from "@/lib/auth/current-user";
import { getPublicProfile } from "@/lib/learner";

type Props = PageProps<"/profile/[handle]">;

export const metadata: Metadata = { title: "Profile", robots: { index: false } };

const joined = new Intl.DateTimeFormat("en-GB", { month: "long", year: "numeric", timeZone: "UTC" });

async function ProfileView({ params }: Props) {
  const { handle } = await params;
  if (!/^[A-Za-z0-9-]{1,39}$/.test(handle)) notFound();
  const profile = await getPublicProfile(handle);
  if (!profile) {
    // Private profiles are indistinguishable from missing ones, except to their owner.
    const me = await getCurrentUser();
    if (me?.handle.toLowerCase() !== handle.toLowerCase()) notFound();
    return (
      <>
        <h1 className="text-3xl font-semibold tracking-tight">@{me.handle}</h1>
        <EmptyState
          icon={Lock}
          className="mt-8"
          title="Your profile is private"
          description="Only you can see this page. Make your profile public in settings to share your progress."
          action={
            <Button asChild variant="outline">
              <Link href="/settings">Open settings</Link>
            </Button>
          }
        />
      </>
    );
  }
  return (
    <>
      <header className="flex flex-wrap items-center gap-4">
        <Avatar className="size-16">
          {profile.avatarUrl && <AvatarImage src={profile.avatarUrl} alt="" />}
          <AvatarFallback>{(profile.displayName || profile.handle).slice(0, 2).toUpperCase()}</AvatarFallback>
        </Avatar>
        <div>
          <h1 className="text-3xl font-semibold tracking-tight">{profile.displayName || profile.handle}</h1>
          <p className="text-muted-foreground">
            @{profile.handle} · joined {joined.format(new Date(profile.joinedAt))}
          </p>
        </div>
      </header>
      <dl className="mt-8 grid max-w-md grid-cols-2 gap-4">
        <div className="rounded-xl border bg-card p-4">
          <dt className="text-sm text-muted-foreground">Experience</dt>
          <dd className="text-2xl font-semibold">{profile.totalXp} XP</dd>
        </div>
        <div className="rounded-xl border bg-card p-4">
          <dt className="text-sm text-muted-foreground">Lessons completed</dt>
          <dd className="text-2xl font-semibold">{profile.completedLessons}</dd>
        </div>
      </dl>
      <section aria-labelledby="achievements-heading" className="mt-8">
        <h2 id="achievements-heading" className="text-lg font-semibold">
          Achievements
        </h2>
        {profile.achievements.length === 0 ? (
          <EmptyState icon={Trophy} className="mt-3" title="No achievements yet" headingLevel={3} />
        ) : (
          <ul className="mt-3 flex flex-wrap gap-3">
            {profile.achievements.map((a) => (
              <li key={a.key} className="rounded-lg border bg-card px-4 py-3 text-sm">
                <p className="font-medium">{a.title}</p>
                <p className="text-muted-foreground">{a.description}</p>
              </li>
            ))}
          </ul>
        )}
      </section>
    </>
  );
}

export default function ProfilePage(props: Props) {
  return (
    <Suspense
      fallback={
        <>
          <LoadingStatus label="Loading profile" />
          <Skeleton aria-hidden className="h-24 w-full max-w-md" />
        </>
      }
    >
      <ProfileView {...props} />
    </Suspense>
  );
}
