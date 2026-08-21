"use client";

import { useRef, useState } from "react";
import { BadgeCheck, Camera, ChevronDown, MailWarning, Save, ShieldCheck, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { useAuth } from "@/components/auth-provider";
import { AvatarCropDialog } from "@/components/avatar-crop-dialog";
import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";
import { UserAvatar } from "@/components/user-avatar";
import { apiFetch, getErrorMessage, type User } from "@/lib/api";

type ProfileForm = {
  name: string;
  phone: string;
  bio: string;
  gender: User["gender"];
};

function getProfileForm(user: User): ProfileForm {
  return {
    name: user.name,
    phone: user.phone,
    bio: user.bio,
    gender: user.gender,
  };
}

function getProfileSource(user: User) {
  return JSON.stringify([
    user.id,
    user.name,
    user.phone,
    user.bio,
    user.gender,
  ]);
}

export default function ProfilPage() {
  const { user, setUser } = useAuth();
  const [formState, setFormState] = useState<{
    source: string;
    form: ProfileForm;
  } | null>(() => user ? {
    source: getProfileSource(user),
    form: getProfileForm(user),
  } : null);
  const [saving, setSaving] = useState(false);

  // Avatar pending state
  const [pendingAvatar, setPendingAvatar] = useState<File | null>(null);
  const [pendingPreview, setPendingPreview] = useState<string | null>(null);
  const [pendingDelete, setPendingDelete] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Crop dialog state
  const [cropSrc, setCropSrc] = useState<string | null>(null);
  const [cropOpen, setCropOpen] = useState(false);

  if (!user) return null;
  const profileSource = getProfileSource(user);
  const form = formState?.source === profileSource
    ? formState.form
    : getProfileForm(user);

  function updateForm(nextForm: ProfileForm) {
    setFormState({ source: profileSource, form: nextForm });
  }

  const displayAvatar = pendingPreview ?? (pendingDelete ? undefined : user.avatar || undefined);

  function handleFileSelect(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;

    if (file.size > 10 * 1024 * 1024) {
      toast.error("Ukuran file terlalu besar. Pilih file yang lebih kecil.");
      return;
    }
    const allowed = ["image/jpeg", "image/png", "image/webp"];
    if (!allowed.includes(file.type)) {
      toast.error("Format file harus JPEG, PNG, atau WebP.");
      return;
    }

    // Buka dialog crop
    const objectUrl = URL.createObjectURL(file);
    setCropSrc(objectUrl);
    setCropOpen(true);
  }

  function handleCropResult(croppedFile: File) {
    // Bersihkan preview lama & crop source
    if (pendingPreview) URL.revokeObjectURL(pendingPreview);
    if (cropSrc) URL.revokeObjectURL(cropSrc);

    setPendingAvatar(croppedFile);
    setPendingPreview(URL.createObjectURL(croppedFile));
    setPendingDelete(false);
    setCropSrc(null);
    setCropOpen(false);
  }

  function handleCropCancel() {
    if (cropSrc) URL.revokeObjectURL(cropSrc);
    setCropSrc(null);
    setCropOpen(false);
  }

  function handleMarkDeleteAvatar() {
    if (pendingPreview) URL.revokeObjectURL(pendingPreview);
    setPendingAvatar(null);
    setPendingPreview(null);
    setPendingDelete(true);
  }

  function handleCancelAvatarChanges() {
    if (pendingPreview) URL.revokeObjectURL(pendingPreview);
    setPendingAvatar(null);
    setPendingPreview(null);
    setPendingDelete(false);
  }

  async function saveProfile(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    try {
      let latestUser: User | null = null;

      // 1. Upload avatar baru jika ada pending file
      if (pendingAvatar) {
        const body = new FormData();
        body.append("avatar", pendingAvatar);
        const data = await apiFetch<{ user: User }>("/api/profile/avatar", { method: "POST", body });
        latestUser = data.user;
      }

      // 2. Hapus avatar jika ditandai untuk dihapus
      if (pendingDelete) {
        const data = await apiFetch<{ user: User }>("/api/profile/avatar", { method: "DELETE" });
        latestUser = data.user;
      }

      // 3. Simpan data profil lainnya
      const profileData = await apiFetch<{ user: User }>("/api/profile", { method: "PATCH", body: JSON.stringify(form) });
      latestUser = profileData.user;

      setFormState({
        source: getProfileSource(latestUser),
        form: getProfileForm(latestUser),
      });
      setUser(latestUser);
      toast.success("Profil berhasil diperbarui.");

      // Preload avatar URL baru sebelum clear preview lokal agar tidak kedip.
      const clearPending = () => {
        if (pendingPreview) URL.revokeObjectURL(pendingPreview);
        setPendingAvatar(null);
        setPendingPreview(null);
        setPendingDelete(false);
      };

      if (pendingAvatar && latestUser.avatar) {
        // Tahan blob preview sampai R2 URL selesai dimuat browser.
        const img = new window.Image();
        img.onload = clearPending;
        img.onerror = clearPending;
        img.src = latestUser.avatar;
      } else {
        clearPending();
      }
    } catch (error) {
      toast.error(getErrorMessage(error));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="space-y-7">
      <PageHeader title="Profil" description="Kelola informasi identitas yang dapat dibagikan kepada aplikasi sesuai scope yang Anda setujui." />
      <div className="grid gap-6 lg:grid-cols-[320px_minmax(0,1fr)]">
        <Card className="h-fit">
          <CardContent className="flex flex-col items-center pt-5 text-center">
            {/* Avatar dengan upload overlay */}
            <div className="group relative">
              <UserAvatar
                className="size-24 ring-4 ring-primary/8"
                name={user.name}
                src={displayAvatar}
                fallbackClassName="text-2xl"
              />
              <button
                type="button"
                disabled={saving}
                onClick={() => fileInputRef.current?.click()}
                className="absolute inset-0 flex cursor-pointer items-center justify-center rounded-full bg-black/50 text-white opacity-0 transition-opacity group-hover:opacity-100 disabled:cursor-not-allowed"
                aria-label="Ganti avatar"
              >
                <Camera className="size-6" />
              </button>
              <input
                ref={fileInputRef}
                type="file"
                accept="image/jpeg,image/png,image/webp"
                className="hidden"
                onChange={handleFileSelect}
              />
            </div>

            <p className="mt-2 text-xs text-muted-foreground">
              {pendingAvatar
                ? <span className="text-primary">Avatar baru dipilih — simpan untuk menerapkan</span>
                : pendingDelete
                  ? <span className="text-destructive">Avatar akan dihapus — simpan untuk menerapkan</span>
                  : "Klik foto untuk mengubah avatar"}
            </p>

            <div className="mt-1 flex items-center justify-center gap-1">
              {/* Idle: ada avatar → tampilkan tombol hapus */}
              {!pendingAvatar && !pendingDelete && !!user.avatar && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-7 text-xs text-destructive hover:text-destructive"
                  onClick={handleMarkDeleteAvatar}
                  disabled={saving}
                >
                  <Trash2 className="size-3" />
                  Hapus avatar
                </Button>
              )}
              {/* Pending avatar baru → batalkan pilihan */}
              {pendingAvatar && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-7 text-xs"
                  onClick={handleCancelAvatarChanges}
                  disabled={saving}
                >
                  Batalkan pilihan
                </Button>
              )}
              {/* Pending delete → urungkan hapus */}
              {pendingDelete && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-7 text-xs"
                  onClick={handleCancelAvatarChanges}
                  disabled={saving}
                >
                  Urungkan hapus
                </Button>
              )}
            </div>

            <h2 className="mt-4 font-semibold">{user.name}</h2><p className="text-sm text-muted-foreground">{user.email}</p>
            <div className="mt-4 flex flex-wrap justify-center gap-2"><Badge variant={user.role === "super_admin" ? "default" : "secondary"}><ShieldCheck />{user.role === "super_admin" ? "Super admin" : "Anggota"}</Badge><Badge variant={user.email_verified ? "outline" : "destructive"} className={user.email_verified ? "text-primary" : ""}>{user.email_verified ? <BadgeCheck /> : <MailWarning />}{user.email_verified ? "Email terverifikasi" : "Belum verifikasi"}</Badge></div>
            <p className="mt-5 break-all text-xs text-muted-foreground">ID: {user.id}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle>Informasi pribadi</CardTitle><CardDescription>Email dan role hanya dapat dikelola melalui kebijakan sistem.</CardDescription></CardHeader>
          <CardContent>
            <form className="grid gap-5 sm:grid-cols-2" onSubmit={saveProfile}>
              <div className="space-y-2 sm:col-span-2"><Label htmlFor="name">Nama lengkap</Label><Input id="name" value={form.name} onChange={(event) => updateForm({ ...form, name: event.target.value })} minLength={2} maxLength={120} required /></div>
              <div className="space-y-2"><Label htmlFor="phone">Nomor telepon</Label><Input id="phone" type="tel" value={form.phone} onChange={(event) => updateForm({ ...form, phone: event.target.value })} maxLength={30} /></div>
              <div className="space-y-2">
                <Label htmlFor="gender">Gender</Label>
                <div className="relative">
                  <select
                    id="gender"
                    value={form.gender}
                    onChange={(event) => updateForm({
                      ...form,
                      gender: event.target.value as User["gender"],
                    })}
                    className={`h-10 w-full appearance-none rounded-lg border border-input bg-transparent py-2 pr-9 pl-3 text-sm outline-none transition-colors focus-visible:border-primary disabled:cursor-not-allowed disabled:opacity-50 ${form.gender ? "text-foreground" : "text-muted-foreground"}`}
                  >
                    <option value="">Pilih gender</option>
                    <option value="male">Laki-laki</option>
                    <option value="female">Perempuan</option>
                    <option value="other">Lainnya</option>
                  </select>
                  <ChevronDown
                    className="pointer-events-none absolute top-1/2 right-3 size-4 -translate-y-1/2 text-muted-foreground"
                    aria-hidden="true"
                  />
                </div>
              </div>
              <div className="space-y-2 sm:col-span-2"><Label htmlFor="bio">Bio</Label><Textarea id="bio" value={form.bio} onChange={(event) => updateForm({ ...form, bio: event.target.value })} maxLength={500} rows={4} /></div>
              <div className="sm:col-span-2"><Button className="w-full sm:w-auto" disabled={saving}>{saving ? <Spinner /> : <Save />}{saving ? "Menyimpan..." : "Simpan perubahan"}</Button></div>
            </form>
          </CardContent>
        </Card>
      </div>

      {/* Dialog crop avatar */}
      {cropSrc && (
        <AvatarCropDialog
          open={cropOpen}
          imageSrc={cropSrc}
          onCrop={handleCropResult}
          onCancel={handleCropCancel}
        />
      )}
    </div>
  );
}
