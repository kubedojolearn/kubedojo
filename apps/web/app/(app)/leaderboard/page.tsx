import type { Metadata } from "next";
import Link from "next/link";
import { unstable_rethrow } from "next/navigation";
import { Suspense } from "react";
import { Trophy } from "lucide-react";
import { CardListSkeleton, LoadingStatus } from "@/components/feedback/loading";
import { EmptyState, ErrorState } from "@/components/feedback/states";
import { PageHeader } from "@/components/layout/page-header";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { getCurrentUser } from "@/lib/auth/current-user";
import { getLeaderboard } from "@/lib/learner";

export const metadata: Metadata = { title: "Leaderboard" };

type Props = PageProps<"/leaderboard">;

/** Tells a signed-in learner whether they're listed, and how to change that. */
async function OptInNote() {
  const me = await getCurrentUser();
  if (!me) return null;
  return (
    <p className="mb-6 rounded-md border bg-card px-4 py-3 text-sm text-muted-foreground">
      {me.settings.leaderboardOptIn ? <>You&apos;re listed as @{me.handle}. </> : <>You&apos;re not listed. </>}
      <Link href="/settings" className="text-link underline underline-offset-4">
        {me.settings.leaderboardOptIn ? "Leave the leaderboard" : "Join the leaderboard"}
      </Link>{" "}
      in settings.
    </p>
  );
}

async function Board({ searchParams }: Props) {
  const raw = (await searchParams).cursor;
  const cursor = typeof raw === "string" && raw.length <= 512 ? raw : undefined;
  let page;
  try {
    page = await getLeaderboard(cursor);
  } catch (err) {
    unstable_rethrow(err);
    return (
      <ErrorState
        title="Couldn't load the leaderboard"
        description="Please try again in a moment."
        action={
          <Button asChild variant="outline">
            <Link href="/leaderboard">Try again</Link>
          </Button>
        }
      />
    );
  }
  if (page.entries.length === 0) {
    return (
      <EmptyState
        icon={Trophy}
        title="No one on the leaderboard yet"
        description="The leaderboard is opt-in. Choose to appear here in your settings."
      />
    );
  }
  return (
    <>
      <div className="overflow-x-auto rounded-lg border bg-card">
        <table className="w-full text-sm">
          <caption className="sr-only">Learners ranked by experience points</caption>
          <thead className="border-b text-left text-xs text-muted-foreground">
            <tr>
              <th scope="col" className="w-16 px-4 py-3 font-medium">
                Rank
              </th>
              <th scope="col" className="px-4 py-3 font-medium">
                Learner
              </th>
              <th scope="col" className="px-4 py-3 text-right font-medium">
                XP
              </th>
              <th scope="col" className="hidden px-4 py-3 text-right font-medium sm:table-cell">
                Lessons
              </th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {page.entries.map((e) => (
              <tr key={e.handle}>
                <td className="px-4 py-3 font-mono tabular-nums">{e.rank}</td>
                <td className="px-4 py-3">
                  <span className="flex items-center gap-3">
                    <Avatar className="size-7">
                      {e.avatarUrl && <AvatarImage src={e.avatarUrl} alt="" />}
                      <AvatarFallback className="text-[10px]">
                        {(e.displayName || e.handle).slice(0, 2).toUpperCase()}
                      </AvatarFallback>
                    </Avatar>
                    <span className="grid">
                      <span className="font-medium">{e.displayName || e.handle}</span>
                      <span className="text-xs text-muted-foreground">@{e.handle}</span>
                    </span>
                  </span>
                </td>
                <td className="px-4 py-3 text-right font-semibold tabular-nums">{e.totalXp}</td>
                <td className="hidden px-4 py-3 text-right tabular-nums sm:table-cell">{e.completedLessons}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="mt-4 flex gap-3">
        {cursor && (
          <Button asChild variant="outline">
            <Link href="/leaderboard">Back to the top</Link>
          </Button>
        )}
        {page.nextCursor && (
          <Button asChild variant="outline">
            <Link href={`/leaderboard?cursor=${encodeURIComponent(page.nextCursor)}`}>Next page</Link>
          </Button>
        )}
      </div>
    </>
  );
}

export default function LeaderboardPage(props: Props) {
  return (
    <>
      <PageHeader title="Leaderboard" description="Learners ranked by XP. Only people who opt in are listed." />
      <Suspense fallback={null}>
        <OptInNote />
      </Suspense>
      <Suspense
        fallback={
          <>
            <LoadingStatus label="Loading the leaderboard" />
            <CardListSkeleton count={1} />
          </>
        }
      >
        <Board {...props} />
      </Suspense>
    </>
  );
}
