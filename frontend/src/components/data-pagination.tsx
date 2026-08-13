"use client";

import { ChevronLeft, ChevronRight, MoreHorizontal } from "lucide-react";

import { Button } from "@/components/ui/button";

type PageItem = number | "ellipsis-start" | "ellipsis-end";

function pageItems(page: number, totalPages: number): PageItem[] {
  if (totalPages <= 7) return Array.from({ length: totalPages }, (_, index) => index + 1);
  if (page <= 4) return [1, 2, 3, 4, 5, "ellipsis-end", totalPages];
  if (page >= totalPages - 3) return [1, "ellipsis-start", totalPages - 4, totalPages - 3, totalPages - 2, totalPages - 1, totalPages];
  return [1, "ellipsis-start", page - 1, page, page + 1, "ellipsis-end", totalPages];
}

export function DataPagination({
  page,
  totalPages,
  total,
  itemLabel,
  onPageChange,
}: {
  page: number;
  totalPages: number;
  total: number;
  itemLabel: string;
  onPageChange: (page: number) => void;
}) {
  const safeTotalPages = Math.max(1, totalPages);
  return (
    <div className="flex flex-col gap-3 border-t p-4 sm:flex-row sm:items-center sm:justify-between">
      <p className="text-xs text-muted-foreground">Halaman {page} dari {safeTotalPages} · {total} {itemLabel}</p>
      <nav className="flex min-w-0 items-center justify-between gap-1 sm:justify-end" aria-label="Navigasi halaman">
        <Button variant="outline" size="icon-sm" disabled={page <= 1} onClick={() => onPageChange(page - 1)} aria-label="Halaman sebelumnya"><ChevronLeft /></Button>
        <div className="flex min-w-0 items-center gap-1">
          {pageItems(page, safeTotalPages).map((item) => typeof item === "number" ? (
            <Button
              key={item}
              variant={item === page ? "default" : "outline"}
              size="icon-sm"
              onClick={() => onPageChange(item)}
              aria-label={`Halaman ${item}`}
              aria-current={item === page ? "page" : undefined}
            >
              {item}
            </Button>
          ) : (
            <span key={item} className="flex size-8 items-center justify-center text-muted-foreground" aria-hidden="true"><MoreHorizontal className="size-4" /></span>
          ))}
        </div>
        <Button variant="outline" size="icon-sm" disabled={page >= safeTotalPages} onClick={() => onPageChange(page + 1)} aria-label="Halaman berikutnya"><ChevronRight /></Button>
      </nav>
    </div>
  );
}
