import { ShieldCheck } from "lucide-react";

import { cn } from "@/lib/utils";
import { PUBLIC_APP_NAME, PUBLIC_APP_TAGLINE } from "@/lib/public-env";

export function Brand({
  compact = false,
  inverted = false,
  className,
}: {
  compact?: boolean;
  inverted?: boolean;
  className?: string;
}) {
  return (
    <div className={cn("flex items-center gap-3", className)}>
      <span
        className={cn(
          "relative flex size-10 shrink-0 items-center justify-center overflow-hidden rounded-xl shadow-sm ring-1",
          inverted
            ? "bg-primary-foreground/12 text-primary-foreground ring-primary-foreground/20"
            : "bg-primary text-primary-foreground ring-primary/15",
        )}
      >
        <span className="absolute -right-2 -bottom-2 size-5 rounded-full bg-amber-300/70" />
        <ShieldCheck className="relative size-5" aria-hidden="true" />
      </span>
      {!compact && (
        <span className="grid leading-none">
          <span className={cn("text-base font-semibold tracking-tight", inverted && "text-primary-foreground")}>
            {PUBLIC_APP_NAME}
          </span>
          <span className={cn("mt-1 text-xs text-muted-foreground", inverted && "text-primary-foreground/70")}>
            {PUBLIC_APP_TAGLINE}
          </span>
        </span>
      )}
    </div>
  );
}
