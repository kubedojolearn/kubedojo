"use client";

import {
  animate,
  motion,
  useInView,
  useMotionTemplate,
  useMotionValue,
  useReducedMotion,
  useSpring,
  type Variants,
} from "motion/react";
import { useEffect, useRef, useSyncExternalStore, type ComponentProps, type PointerEvent, type ReactNode } from "react";
import { cn } from "@/lib/utils";

/**
 * Landing-page motion. Every effect here is decorative: content is in the DOM from the first
 * render, and both the OS setting (prefers-reduced-motion) and the account setting
 * (html.reduce-motion) show the final state at once. A <noscript> rule on the page reveals
 * [data-reveal] elements when JavaScript is off.
 */

const EASE_OUT = [0.22, 1, 0.36, 1] as const;

/** True when the OS or the account asks for reduced motion. */
export function useCalmMotion(): boolean {
  const os = useReducedMotion();
  const account = useSyncExternalStore(subscribeToHtmlClass, readAccountSetting, () => false);
  return Boolean(os) || account;
}

function subscribeToHtmlClass(onChange: () => void) {
  const observer = new MutationObserver(onChange);
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });
  return () => observer.disconnect();
}

function readAccountSetting() {
  return document.documentElement.classList.contains("reduce-motion");
}

const revealVariants: Variants = {
  hidden: { opacity: 0, y: 18, filter: "blur(6px)" },
  shown: { opacity: 1, y: 0, filter: "blur(0px)" },
};

type RevealProps = Omit<ComponentProps<typeof motion.div>, "variants" | "initial" | "whileInView" | "animate"> & {
  delay?: number;
  /** Animate on mount (above the fold) instead of when scrolled into view. */
  onMount?: boolean;
};

/** Fades and lifts its children into place the first time they scroll into view. */
export function Reveal({ delay = 0, onMount = false, transition, children, ...props }: RevealProps) {
  const calm = useCalmMotion();
  return (
    <motion.div
      data-reveal
      variants={revealVariants}
      initial={calm ? false : "hidden"}
      {...(calm || onMount ? { animate: "shown" } : { whileInView: "shown", viewport: { once: true, amount: 0.2 } })}
      transition={calm ? { duration: 0 } : { duration: 0.6, ease: EASE_OUT, delay, ...transition }}
      {...props}
    >
      {children}
    </motion.div>
  );
}

/** A container whose <RevealItem> children enter one after another. */
export function RevealGroup({
  stagger = 0.08,
  delay = 0,
  onMount = false,
  as = "div",
  className,
  children,
}: {
  stagger?: number;
  delay?: number;
  onMount?: boolean;
  as?: "div" | "ul" | "ol" | "dl";
  className?: string;
  children: ReactNode;
}) {
  const calm = useCalmMotion();
  const Comp = motion[as];
  return (
    <Comp
      initial={calm ? false : "hidden"}
      {...(calm || onMount ? { animate: "shown" } : { whileInView: "shown", viewport: { once: true, amount: 0.15 } })}
      variants={{
        hidden: {},
        shown: { transition: calm ? {} : { staggerChildren: stagger, delayChildren: delay } },
      }}
      className={className}
    >
      {children}
    </Comp>
  );
}

/** One child of a <RevealGroup>. */
export function RevealItem({
  as = "div",
  className,
  children,
}: {
  as?: "div" | "li" | "span" | "p";
  className?: string;
  children: ReactNode;
}) {
  const calm = useCalmMotion();
  const Comp = motion[as];
  return (
    <Comp
      data-reveal
      variants={revealVariants}
      transition={calm ? { duration: 0 } : { duration: 0.55, ease: EASE_OUT }}
      className={className}
    >
      {children}
    </Comp>
  );
}

/** A line that draws itself across (or down, on small screens) when scrolled into view. */
export function DrawLine({ className }: { className?: string }) {
  const calm = useCalmMotion();
  return (
    <motion.span
      aria-hidden
      data-reveal
      initial={calm ? false : { scaleX: 0, scaleY: 0 }}
      {...(calm
        ? { animate: { scaleX: 1, scaleY: 1 } }
        : { whileInView: { scaleX: 1, scaleY: 1 }, viewport: { once: true, amount: 0.5 } })}
      transition={calm ? { duration: 0 } : { duration: 1.1, ease: EASE_OUT, delay: 0.15 }}
      className={cn("origin-top-left", className)}
    />
  );
}

/** A card with a soft light that follows the pointer. Purely visual; children keep their semantics. */
export function Spotlight({ className, children }: { className?: string; children: ReactNode }) {
  const calm = useCalmMotion();
  const x = useMotionValue(-400);
  const y = useMotionValue(-400);
  const background = useMotionTemplate`radial-gradient(260px circle at ${x}px ${y}px, var(--glow-strong), transparent 70%)`;
  const move = (e: PointerEvent<HTMLDivElement>) => {
    const r = e.currentTarget.getBoundingClientRect();
    x.set(e.clientX - r.left);
    y.set(e.clientY - r.top);
  };
  return (
    <div onPointerMove={calm ? undefined : move} className={cn("group/spot relative", className)}>
      {children}
      {calm ? null : (
        <motion.div
          aria-hidden
          className="pointer-events-none absolute inset-0 z-10 rounded-[inherit] opacity-0 transition-opacity duration-200 group-hover/spot:opacity-100"
          style={{ background }}
        />
      )}
    </div>
  );
}

/**
 * A number that counts up from zero the first time it scrolls into view. The server renders the
 * final value, so it is correct without JavaScript and for screen readers.
 */
export function CountUp({ value, className }: { value: number; className?: string }) {
  const calm = useCalmMotion();
  const ref = useRef<HTMLSpanElement>(null);
  const inView = useInView(ref, { once: true, amount: 0.6 });
  useEffect(() => {
    const el = ref.current;
    if (!el || !inView || calm || value === 0) return;
    const controls = animate(0, value, {
      duration: 1.2,
      ease: EASE_OUT,
      onUpdate: (v) => {
        el.textContent = String(Math.round(v));
      },
    });
    return () => controls.stop();
  }, [inView, calm, value]);
  return (
    <span ref={ref} className={cn("tabular-nums", className)}>
      {value}
    </span>
  );
}

/** Tilts its child a few degrees toward the pointer, on a spring. */
export function Tilt({ className, children, max = 4 }: { className?: string; children: ReactNode; max?: number }) {
  const calm = useCalmMotion();
  const rx = useSpring(0, { stiffness: 150, damping: 18 });
  const ry = useSpring(0, { stiffness: 150, damping: 18 });
  const move = (e: PointerEvent<HTMLDivElement>) => {
    if (e.pointerType !== "mouse") return;
    const r = e.currentTarget.getBoundingClientRect();
    ry.set(((e.clientX - r.left) / r.width - 0.5) * 2 * max);
    rx.set(-((e.clientY - r.top) / r.height - 0.5) * 2 * max);
  };
  const leave = () => {
    rx.set(0);
    ry.set(0);
  };
  if (calm) return <div className={className}>{children}</div>;
  return (
    <div className={cn("[perspective:1400px]", className)} onPointerMove={move} onPointerLeave={leave}>
      <motion.div style={{ rotateX: rx, rotateY: ry, transformStyle: "preserve-3d" }}>{children}</motion.div>
    </div>
  );
}
