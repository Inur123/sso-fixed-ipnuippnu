"use client";

import { useState } from "react";
import { BadgeCheck, MailWarning, Save, ShieldCheck } from "lucide-react";
import { toast } from "sonner";

import { useAuth } from "@/components/auth-provider";
import { PageHeader } from "@/components/page-header";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";
import { apiFetch, getErrorMessage, type User } from "@/lib/api";

export default function ProfilPage() {
  const { user, setUser } = useAuth();
  const [form, setForm] = useState<{ name: string; phone: string; bio: string; gender: User["gender"]; avatar: string }>(() => ({ name: user?.name ?? "", phone: user?.phone ?? "", bio: user?.bio ?? "", gender: user?.gender ?? "", avatar: user?.avatar ?? "" }));
  const [saving, setSaving] = useState(false);

  if (!user) return null;
  const initials = user.name.split(" ").map((part) => part[0]).join("").slice(0, 2).toUpperCase();

  async function saveProfile(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    try {
      const data = await apiFetch<{ user: User }>("/api/profile", { method: "PATCH", body: JSON.stringify(form) });
      setUser(data.user);
      toast.success("Profil berhasil diperbarui.");
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
            <Avatar className="size-24 ring-4 ring-primary/8"><AvatarImage src={user.avatar} alt={user.name} /><AvatarFallback className="text-2xl">{initials}</AvatarFallback></Avatar>
            <h2 className="mt-4 font-semibold">{user.name}</h2><p className="text-sm text-muted-foreground">{user.email}</p>
            <div className="mt-4 flex flex-wrap justify-center gap-2"><Badge variant={user.role === "super_admin" ? "default" : "secondary"}><ShieldCheck />{user.role === "super_admin" ? "Super admin" : "Anggota"}</Badge><Badge variant={user.email_verified ? "outline" : "destructive"} className={user.email_verified ? "text-primary" : ""}>{user.email_verified ? <BadgeCheck /> : <MailWarning />}{user.email_verified ? "Email terverifikasi" : "Belum verifikasi"}</Badge></div>
            <p className="mt-5 break-all text-xs text-muted-foreground">ID: {user.id}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle>Informasi pribadi</CardTitle><CardDescription>Email dan role hanya dapat dikelola melalui kebijakan sistem.</CardDescription></CardHeader>
          <CardContent>
            <form className="grid gap-5 sm:grid-cols-2" onSubmit={saveProfile}>
              <div className="space-y-2 sm:col-span-2"><Label htmlFor="name">Nama lengkap</Label><Input id="name" value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} minLength={2} maxLength={120} required /></div>
              <div className="space-y-2"><Label htmlFor="phone">Nomor telepon</Label><Input id="phone" type="tel" value={form.phone} onChange={(event) => setForm({ ...form, phone: event.target.value })} maxLength={30} /></div>
              <div className="space-y-2"><Label>Gender</Label><Select value={form.gender || undefined} onValueChange={(gender: User["gender"]) => setForm({ ...form, gender })}><SelectTrigger className="w-full"><SelectValue placeholder="Pilih gender" /></SelectTrigger><SelectContent><SelectItem value="male">Laki-laki</SelectItem><SelectItem value="female">Perempuan</SelectItem><SelectItem value="other">Lainnya</SelectItem></SelectContent></Select></div>
              <div className="space-y-2 sm:col-span-2"><Label htmlFor="avatar">URL avatar</Label><Input id="avatar" type="url" value={form.avatar} onChange={(event) => setForm({ ...form, avatar: event.target.value })} placeholder="https://..." /></div>
              <div className="space-y-2 sm:col-span-2"><Label htmlFor="bio">Bio</Label><Textarea id="bio" value={form.bio} onChange={(event) => setForm({ ...form, bio: event.target.value })} maxLength={500} rows={4} /></div>
              <div className="sm:col-span-2"><Button className="w-full sm:w-auto" disabled={saving}>{saving ? <Spinner /> : <Save />}{saving ? "Menyimpan..." : "Simpan perubahan"}</Button></div>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
