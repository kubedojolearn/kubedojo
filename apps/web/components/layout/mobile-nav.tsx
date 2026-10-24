"use client";

import Link from "next/link";
import { Menu, X } from "lucide-react";
import { useEffect, useId, useRef, useState } from "react";
import { NavLink } from "@/components/layout/nav-link";
import { Button } from "@/components/ui/button";
import { mainNav } from "@/lib/site";

const linkClass =
  "block rounded-md px-3 py-2.5 text-body text-foreground-secondary hover:bg-hover hover:text-foreground";

/** The main navigation below the xl breakpoint, where the inline links don't fit: a panel under the header. */
export function MobileNav({ signedIn }: { signedIn: boolean }) {
  const [open, setOpen] = useState(false);
  const id = useId();
  const button = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      setOpen(false);
      button.current?.focus();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open]);

  const close = () => setOpen(false);

  return (
    <div className="xl:hidden">
      <Button
        ref={button}
        variant="ghost"
        size="icon"
        aria-label={open ? "Close menu" : "Open menu"}
        aria-expanded={open}
        aria-controls={id}
        onClick={() => setOpen((o) => !o)}
      >
        {open ? <X /> : <Menu />}
      </Button>
      <nav
        id={id}
        aria-label="Mobile"
        hidden={!open}
        className="absolute inset-x-0 top-14 max-h-[calc(100svh-3.5rem)] overflow-y-auto border-b border-border-subtle bg-background px-4 py-3 shadow-xl shadow-shade/40"
      >
        <ul className="mx-auto max-w-[1280px] space-y-1">
          {mainNav.map((item) => (
            <li key={item.href}>
              {item.href.startsWith("http") ? (
                <a href={item.href} rel="noreferrer" className={linkClass}>
                  {item.label}
                </a>
              ) : (
                <NavLink
                  href={item.href}
                  onClick={close}
                  className={linkClass}
                  activeClassName="bg-hover text-foreground"
                >
                  {item.label}
                </NavLink>
              )}
            </li>
          ))}
        </ul>
        {!signedIn && (
          <div className="mx-auto mt-3 grid max-w-[1280px] gap-2 border-t border-border-subtle pt-3 sm:hidden">
            <Button asChild>
              <Link href="/learn" onClick={close}>
                Start learning
              </Link>
            </Button>
          </div>
        )}
      </nav>
    </div>
  );
}
