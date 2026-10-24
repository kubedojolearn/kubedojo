"use client";

import { useId, type ComponentProps, type ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

/**
 * A control that is visible but not available yet. Uses aria-disabled (not `disabled`) so it stays
 * focusable and the reason is reachable by keyboard and screen readers; activation does nothing.
 */
export function UnavailableButton({
  reason,
  className,
  children,
  ...props
}: Omit<ComponentProps<typeof Button>, "onClick" | "disabled" | "asChild"> & { reason: string; children: ReactNode }) {
  const reasonId = useId();
  return (
    <Tooltip>
      <span id={reasonId} hidden>
        {reason}
      </span>
      <TooltipTrigger asChild>
        <Button
          {...props}
          aria-disabled="true"
          aria-describedby={reasonId}
          onClick={(e) => e.preventDefault()}
          className={cn("cursor-not-allowed opacity-60 hover:bg-transparent", className)}
        >
          {children}
        </Button>
      </TooltipTrigger>
      <TooltipContent>{reason}</TooltipContent>
    </Tooltip>
  );
}
