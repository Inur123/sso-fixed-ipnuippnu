"use client";

import * as React from "react";

import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { cn } from "@/lib/utils";

interface UserAvatarProps
  extends Omit<React.ComponentProps<typeof Avatar>, "children"> {
  name: string;
  src?: string | null;
  alt?: string;
  fallback?: React.ReactNode;
  fallbackClassName?: string;
  imageClassName?: string;
  children?: React.ReactNode;
}

function getInitials(name: string) {
  return name
    .trim()
    .split(/\s+/)
    .filter(Boolean)
    .map((part) => part[0])
    .join("")
    .slice(0, 2)
    .toUpperCase();
}

export function UserAvatar({
  name,
  src,
  alt = `Foto profil ${name}`,
  fallback,
  fallbackClassName,
  imageClassName,
  children,
  className,
  ...props
}: UserAvatarProps) {
  const currentSrc = src?.trim() ?? "";
  const [failedSrc, setFailedSrc] = React.useState("");
  const showImage = Boolean(currentSrc) && failedSrc !== currentSrc;

  return (
    <Avatar
      className={cn("isolate overflow-visible bg-muted", className)}
      data-avatar-status={showImage ? "image" : "fallback"}
      {...props}
    >
      {showImage ? (
        // Avatar sengaja dirender sebagai img native agar URL sudah dimuat saat
        // HTML SSR diparsing, tanpa menunggu hydration JavaScript terlebih dulu.
        // eslint-disable-next-line @next/next/no-img-element
        <img
          key={currentSrc}
          src={currentSrc}
          alt={alt}
          className={cn(
            "aspect-square size-full rounded-full object-cover",
            imageClassName,
          )}
          loading="eager"
          decoding="sync"
          fetchPriority="high"
          draggable={false}
          onError={() => setFailedSrc(currentSrc)}
        />
      ) : (
        <AvatarFallback className={fallbackClassName}>
          {fallback ?? getInitials(name)}
        </AvatarFallback>
      )}
      {children}
    </Avatar>
  );
}
