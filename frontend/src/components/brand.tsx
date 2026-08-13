import Image from "next/image";

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
      <Image
        src="/images/logo-sso.png"
        alt={`Logo ${PUBLIC_APP_NAME}`}
        width={44}
        height={44}
        priority
        className="size-11 shrink-0 object-contain"
      />
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
