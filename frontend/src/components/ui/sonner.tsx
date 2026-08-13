"use client"

import { useTheme } from "next-themes"
import { Toaster as Sonner, type ToasterProps } from "sonner"
import { CircleCheckIcon, InfoIcon, TriangleAlertIcon, OctagonXIcon, Loader2Icon } from "lucide-react"

const Toaster = ({ ...props }: ToasterProps) => {
  const { theme = "system" } = useTheme()

  return (
    <Sonner
      theme={theme as ToasterProps["theme"]}
      className="toaster group"
      closeButton
      icons={{
        success: (
          <CircleCheckIcon className="size-4" />
        ),
        info: (
          <InfoIcon className="size-4" />
        ),
        warning: (
          <TriangleAlertIcon className="size-4" />
        ),
        error: (
          <OctagonXIcon className="size-4" />
        ),
        loading: (
          <Loader2Icon className="size-4 animate-spin" />
        ),
      }}
      style={
        {
          "--normal-bg": "var(--background)",
          "--normal-text": "var(--foreground)",
          "--normal-border": "var(--border)",
          "--success-bg": "var(--background)",
          "--success-text": "var(--foreground)",
          "--success-border": "var(--border)",
          "--info-bg": "var(--background)",
          "--info-text": "var(--foreground)",
          "--info-border": "var(--border)",
          "--warning-bg": "var(--background)",
          "--warning-text": "var(--foreground)",
          "--warning-border": "var(--border)",
          "--error-bg": "var(--background)",
          "--error-text": "var(--foreground)",
          "--error-border": "var(--border)",
          "--border-radius": "var(--radius)",
        } as React.CSSProperties
      }
      toastOptions={{
        classNames: {
          toast: "cn-toast !border-border !bg-background !text-foreground shadow-xl",
          title: "!text-foreground",
          description: "!text-muted-foreground",
          icon: "!text-foreground",
          closeButton: "!border-border !bg-background !text-foreground hover:!bg-muted",
          actionButton: "!bg-foreground !text-background",
          cancelButton: "!bg-muted !text-foreground",
          success: "!border-border !bg-background !text-foreground",
          info: "!border-border !bg-background !text-foreground",
          warning: "!border-border !bg-background !text-foreground",
          error: "!border-border !bg-background !text-foreground",
        },
      }}
      {...props}
    />
  )
}

export { Toaster }
