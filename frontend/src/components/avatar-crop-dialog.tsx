"use client";

import { useRef, useCallback } from "react";
import Cropper, { type ReactCropperElement } from "react-cropper";
import { Crop, ZoomIn, ZoomOut, RotateCw } from "lucide-react";
import "cropperjs/dist/cropper.css";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

interface AvatarCropDialogProps {
  open: boolean;
  imageSrc: string;
  onCrop: (croppedFile: File) => void;
  onCancel: () => void;
}

export function AvatarCropDialog({ open, imageSrc, onCrop, onCancel }: AvatarCropDialogProps) {
  const cropperRef = useRef<ReactCropperElement>(null);

  const handleCrop = useCallback(() => {
    const cropper = cropperRef.current?.cropper;
    if (!cropper) return;

    const canvas = cropper.getCroppedCanvas({
      width: 512,
      height: 512,
      imageSmoothingEnabled: true,
      imageSmoothingQuality: "high",
    });

    canvas.toBlob(
      (blob: Blob | null) => {
        if (!blob) return;
        const file = new File([blob], "avatar.webp", { type: "image/webp" });
        onCrop(file);
      },
      "image/webp",
      0.9,
    );
  }, [onCrop]);

  return (
    <Dialog open={open} onOpenChange={(isOpen) => { if (!isOpen) onCancel(); }}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Sesuaikan avatar</DialogTitle>
          <DialogDescription>Geser dan perbesar untuk mengatur area foto yang diinginkan.</DialogDescription>
        </DialogHeader>

        <div className="overflow-hidden rounded-lg bg-muted">
          <Cropper
            ref={cropperRef}
            src={imageSrc}
            style={{ height: 320, width: "100%" }}
            aspectRatio={1}
            viewMode={1}
            guides={false}
            center
            background={false}
            responsive
            autoCropArea={1}
            checkOrientation={false}
          />
        </div>

        {/* Toolbar */}
        <div className="flex items-center justify-center gap-1">
          <Button
            type="button"
            variant="outline"
            size="icon-sm"
            onClick={() => cropperRef.current?.cropper?.zoom(0.1)}
            aria-label="Perbesar"
          >
            <ZoomIn className="size-4" />
          </Button>
          <Button
            type="button"
            variant="outline"
            size="icon-sm"
            onClick={() => cropperRef.current?.cropper?.zoom(-0.1)}
            aria-label="Perkecil"
          >
            <ZoomOut className="size-4" />
          </Button>
          <Button
            type="button"
            variant="outline"
            size="icon-sm"
            onClick={() => cropperRef.current?.cropper?.rotate(90)}
            aria-label="Putar"
          >
            <RotateCw className="size-4" />
          </Button>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onCancel}>Batal</Button>
          <Button onClick={handleCrop}><Crop className="size-4" />Terapkan</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
