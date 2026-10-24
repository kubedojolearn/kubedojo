"use client";

import { useRouter } from "next/navigation";
import { BookOpen, FileText, Search, TextSearch } from "lucide-react";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import { Kbd } from "@/components/ui/kbd";
import { glossary } from "@/lib/glossary.generated";
import { cn } from "@/lib/utils";

export interface SearchLesson {
  href: string;
  title: string;
  course: string;
  summary: string;
}

const pages = [
  { href: "/dashboard", title: "Dashboard" },
  { href: "/learn", title: "Curriculum" },
  { href: "/paths", title: "Learning path" },
  { href: "/leaderboard", title: "Leaderboard" },
  { href: "/settings", title: "Settings" },
  { href: "/learn/glossary", title: "Glossary" },
];

/**
 * ⌘K / Ctrl+K search over published lessons, glossary terms and app pages. `lessons` is null
 * while the index loads (or if the catalog is down); terms and pages still work.
 */
export function SiteSearch({ lessons, className }: { lessons: SearchLesson[] | null; className?: string }) {
  const router = useRouter();
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key.toLowerCase() === "k" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        setOpen((o) => !o);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const go = (href: string) => {
    setOpen(false);
    router.push(href);
  };

  return (
    <>
      <Button
        variant="outline"
        onClick={() => setOpen(true)}
        className={cn("justify-start gap-2 font-normal text-muted-foreground", className)}
      >
        <Search />
        <span>Search lessons and terms</span>
        <Kbd className="ml-auto">⌘K</Kbd>
      </Button>
      <CommandDialog
        open={open}
        onOpenChange={setOpen}
        className="sm:max-w-xl"
        title="Search KubeDojo"
        description="Find a lesson, a glossary term or a page."
      >
        <CommandInput placeholder="Try “pending”, “service” or “settings”" />
        <CommandList>
          <CommandEmpty>Nothing matches. Lessons that aren&apos;t published yet don&apos;t show up here.</CommandEmpty>
          {lessons && lessons.length > 0 && (
            <CommandGroup heading="Lessons">
              {lessons.map((l) => (
                <CommandItem key={l.href} value={`${l.title} ${l.course} ${l.summary}`} onSelect={() => go(l.href)}>
                  <BookOpen />
                  <span className="grid">
                    <span>{l.title}</span>
                    <span className="text-xs text-muted-foreground">{l.course}</span>
                  </span>
                </CommandItem>
              ))}
            </CommandGroup>
          )}
          <CommandGroup heading="Glossary">
            {glossary.map((t) => (
              <CommandItem
                key={t.id}
                value={`${t.term} ${t.definition}`}
                onSelect={() => go(`/learn/glossary#${t.id}`)}
              >
                <TextSearch />
                {t.term}
              </CommandItem>
            ))}
          </CommandGroup>
          <CommandGroup heading="Pages">
            {pages.map((p) => (
              <CommandItem key={p.href} value={p.title} onSelect={() => go(p.href)}>
                <FileText />
                {p.title}
              </CommandItem>
            ))}
          </CommandGroup>
        </CommandList>
      </CommandDialog>
    </>
  );
}
