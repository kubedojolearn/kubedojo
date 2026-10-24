import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";
import { Slot } from "radix-ui";

const buttonVariants = cva(
  "group/button inline-flex shrink-0 items-center justify-center gap-2 rounded-md border border-transparent bg-clip-padding font-medium whitespace-nowrap transition-colors duration-150 ease-out outline-none select-none focus-visible:ring-3 focus-visible:ring-ring/25 disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default: "bg-primary text-primary-foreground hover:bg-primary/90 active:bg-primary/80",
        outline:
          "border-border-strong bg-card text-foreground hover:bg-hover active:bg-elevated aria-expanded:bg-hover",
        secondary: "bg-elevated text-foreground hover:bg-hover aria-expanded:bg-hover",
        ghost:
          "text-foreground-secondary hover:bg-hover hover:text-foreground aria-expanded:bg-hover aria-expanded:text-foreground",
        destructive:
          "border-danger/30 bg-danger/8 text-danger dark:bg-danger/12 hover:bg-danger/20 focus-visible:ring-danger/25",
        link: "text-link underline-offset-4 hover:underline",
      },
      size: {
        default: "h-9 px-4 text-small",
        xs: "h-6 gap-1 rounded-sm px-2 text-caption [&_svg:not([class*='size-'])]:size-3",
        sm: "h-8 gap-1.5 px-3 text-small",
        lg: "h-10 px-5 text-body [&_svg:not([class*='size-'])]:size-[18px]",
        hero: "h-12 px-6 text-body [&_svg:not([class*='size-'])]:size-5",
        icon: "size-8",
        "icon-xs": "size-6 rounded-sm [&_svg:not([class*='size-'])]:size-3",
        "icon-sm": "size-7",
        "icon-lg": "size-9",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  },
);

function Button({
  className,
  variant = "default",
  size = "default",
  asChild = false,
  ...props
}: React.ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean;
  }) {
  const Comp = asChild ? Slot.Root : "button";

  return (
    <Comp
      data-slot="button"
      data-variant={variant}
      data-size={size}
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  );
}

export { Button, buttonVariants };
