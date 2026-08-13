"use client";

import { useEffect, useRef, useState } from "react";
import { AppWindow, Copy, Eye, EyeOff, Pencil, Plus, RefreshCw, ShieldCheck, Trash2, UserPlus, Users } from "lucide-react";
import { toast } from "sonner";

import { OAuthClientCardSkeleton } from "@/components/dashboard-loading-skeleton";
import { PageHeader } from "@/components/page-header";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";
import { apiFetch, getErrorMessage, type OAuthClient, type OAuthClientAssignment } from "@/lib/api";

type ClientForm = {
  name: string;
  description: string;
  redirectUris: string[];
  accessPolicy: OAuthClient["access_policy"];
};

const EMPTY_FORM: ClientForm = { name: "", description: "", redirectUris: [""], accessPolicy: "assigned_only" };

function canRetainClientSecret() {
  return document.visibilityState === "visible" && document.hasFocus();
}

function validateRedirectURIs(values: string[]) {
  const errors: Record<number, string> = {};
  const normalizedValues = values.map((value) => value.trim());
  normalizedValues.forEach((value, index) => {
    if (!value) {
      errors[index] = "Redirect URI wajib diisi.";
      return;
    }
    try {
      const url = new URL(value);
      const isLocalhost = url.hostname === "localhost" || url.hostname === "127.0.0.1";
      if (url.protocol !== "https:" && !(isLocalhost && url.protocol === "http:")) {
        errors[index] = "Gunakan HTTPS. HTTP hanya diizinkan untuk localhost.";
      } else if (url.hash) {
        errors[index] = "Redirect URI tidak boleh memiliki fragmen (#).";
      } else if (normalizedValues.indexOf(value) !== index) {
        errors[index] = "Redirect URI ini sudah ditambahkan.";
      }
    } catch {
      errors[index] = "Masukkan URL valid, misalnya https://app.example.com/callback.";
    }
  });
  return errors;
}

export default function AplikasiPage() {
  const [clients, setClients] = useState<OAuthClient[]>([]);
  const [loading, setLoading] = useState(true);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingClient, setEditingClient] = useState<OAuthClient | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [clientSecrets, setClientSecrets] = useState<Record<string, string>>({});
  const [visibleSecrets, setVisibleSecrets] = useState<Record<string, boolean>>({});
  const [loadingSecretID, setLoadingSecretID] = useState<string | null>(null);
  const [regeneratingSecretID, setRegeneratingSecretID] = useState<string | null>(null);
  const [form, setForm] = useState<ClientForm>(EMPTY_FORM);
  const [redirectURIErrors, setRedirectURIErrors] = useState<Record<number, string>>({});
  const [accessClient, setAccessClient] = useState<OAuthClient | null>(null);
  const [assignments, setAssignments] = useState<OAuthClientAssignment[]>([]);
  const [assignmentIdentifier, setAssignmentIdentifier] = useState("");
  const [accessLoading, setAccessLoading] = useState(false);
  const [assignmentSubmitting, setAssignmentSubmitting] = useState(false);
  const secretMemoryGeneration = useRef(0);

  useEffect(() => {
    let active = true;
    apiFetch<{ clients: OAuthClient[] }>("/api/clients")
      .then((data) => { if (active) setClients(data.clients); })
      .catch((error) => { if (active) toast.error(getErrorMessage(error)); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, []);

  useEffect(() => {
    const clearSecrets = () => {
      secretMemoryGeneration.current += 1;
      setClientSecrets({});
      setVisibleSecrets({});
    };
    const handleVisibilityChange = () => {
      if (document.visibilityState === "hidden") clearSecrets();
    };
    document.addEventListener("visibilitychange", handleVisibilityChange);
    window.addEventListener("blur", clearSecrets);
    return () => {
      document.removeEventListener("visibilitychange", handleVisibilityChange);
      window.removeEventListener("blur", clearSecrets);
    };
  }, []);

  function openCreateDialog() {
    setEditingClient(null);
    setForm(EMPTY_FORM);
    setRedirectURIErrors({});
    setDialogOpen(true);
  }

  function openEditDialog(client: OAuthClient) {
    setEditingClient(client);
    setForm({
      name: client.name,
      description: client.description,
      redirectUris: client.redirect_uris.length ? [...client.redirect_uris] : [""],
      accessPolicy: client.access_policy || "assigned_only",
    });
    setRedirectURIErrors({});
    setDialogOpen(true);
  }

  function handleDialogOpenChange(open: boolean) {
    if (!open && submitting) return;
    setDialogOpen(open);
    if (!open) {
      setEditingClient(null);
      setForm(EMPTY_FORM);
      setRedirectURIErrors({});
    }
  }

  function closeAndResetDialog() {
    setDialogOpen(false);
    setEditingClient(null);
    setForm(EMPTY_FORM);
    setRedirectURIErrors({});
  }

  async function submitClient(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const errors = validateRedirectURIs(form.redirectUris);
    if (Object.keys(errors).length > 0) {
      setRedirectURIErrors(errors);
      toast.error("Periksa kembali Redirect URI aplikasi.");
      return;
    }

    setSubmitting(true);
    const memoryGeneration = secretMemoryGeneration.current;
    try {
      const payload = {
        name: form.name.trim(),
        description: form.description.trim(),
        redirect_uris: form.redirectUris.map((item) => item.trim()),
        access_policy: form.accessPolicy,
      };
      const path = editingClient ? `/api/clients/${encodeURIComponent(editingClient.client_id)}` : "/api/clients";
      const data = await apiFetch<{ client: OAuthClient }>(path, {
        method: editingClient ? "PATCH" : "POST",
        body: JSON.stringify(payload),
      });

      if (editingClient) {
        setClients((items) => items.map((item) => item.client_id === data.client.client_id ? data.client : item));
        toast.success("Data aplikasi berhasil diperbarui.");
      } else {
        const { client_secret: clientSecret, ...clientData } = data.client;
        const client = { ...clientData, secret_available: Boolean(clientSecret || clientData.secret_available) };
        setClients((items) => [client, ...items]);
        if (clientSecret && memoryGeneration === secretMemoryGeneration.current && canRetainClientSecret()) {
          setClientSecrets((secrets) => ({ ...secrets, [client.client_id]: clientSecret }));
          setVisibleSecrets((visible) => ({ ...visible, [client.client_id]: true }));
        }
        toast.success("Aplikasi OAuth berhasil didaftarkan.");
      }
      closeAndResetDialog();
    } catch (error) {
      toast.error(getErrorMessage(error));
    } finally {
      setSubmitting(false);
    }
  }

  function updateRedirectURI(index: number, value: string) {
    const next = form.redirectUris.map((uri, uriIndex) => uriIndex === index ? value : uri);
    setForm((current) => ({ ...current, redirectUris: next }));
    setRedirectURIErrors((current) => Object.keys(current).length ? validateRedirectURIs(next) : current);
  }

  function addRedirectURI() {
    setForm((current) => ({ ...current, redirectUris: [...current.redirectUris, ""] }));
  }

  function removeRedirectURI(index: number) {
    if (form.redirectUris.length === 1) return;
    const next = form.redirectUris.filter((_, uriIndex) => uriIndex !== index);
    setForm((current) => ({ ...current, redirectUris: next }));
    setRedirectURIErrors((current) => Object.keys(current).length ? validateRedirectURIs(next) : {});
  }

  async function deleteClient(client: OAuthClient) {
    try {
      await apiFetch(`/api/clients/${encodeURIComponent(client.client_id)}`, { method: "DELETE" });
      setClients((items) => items.filter((item) => item.client_id !== client.client_id));
      setClientSecrets((secrets) => { const next = { ...secrets }; delete next[client.client_id]; return next; });
      setVisibleSecrets((visible) => { const next = { ...visible }; delete next[client.client_id]; return next; });
      toast.success("Aplikasi dan seluruh akses terkait berhasil dihapus.");
    } catch (error) {
      toast.error(getErrorMessage(error));
    }
  }

  async function copy(value: string) {
    try {
      await navigator.clipboard.writeText(value);
      toast.success("Disalin ke clipboard.");
    } catch {
      toast.error("Tidak dapat menyalin ke clipboard.");
    }
  }

  async function toggleClientSecret(client: OAuthClient) {
    const clientID = client.client_id;
    if (visibleSecrets[clientID]) {
      setClientSecrets((secrets) => { const next = { ...secrets }; delete next[clientID]; return next; });
      setVisibleSecrets((visible) => { const next = { ...visible }; delete next[clientID]; return next; });
      return;
    }
    if (!client.secret_available) {
      toast.error("Secret lama tidak dapat dipulihkan. Regenerate client secret untuk membuat yang baru.");
      return;
    }

    setLoadingSecretID(clientID);
    const memoryGeneration = secretMemoryGeneration.current;
    try {
      const data = await apiFetch<{ client_secret: string }>(`/api/clients/${encodeURIComponent(clientID)}/secret`);
      if (memoryGeneration === secretMemoryGeneration.current && canRetainClientSecret()) {
        setClientSecrets((secrets) => ({ ...secrets, [clientID]: data.client_secret }));
        setVisibleSecrets((visible) => ({ ...visible, [clientID]: true }));
      }
    } catch (error) {
      toast.error(getErrorMessage(error));
    } finally {
      setLoadingSecretID(null);
    }
  }

  async function regenerateClientSecret(client: OAuthClient) {
    const clientID = client.client_id;
    setClientSecrets((secrets) => { const next = { ...secrets }; delete next[clientID]; return next; });
    setVisibleSecrets((visible) => { const next = { ...visible }; delete next[clientID]; return next; });
    setRegeneratingSecretID(clientID);
    const memoryGeneration = secretMemoryGeneration.current;
    try {
      const data = await apiFetch<{ client_secret: string; secret_version: number; message?: string }>(`/api/clients/${encodeURIComponent(clientID)}/secret/regenerate`, {
        method: "POST",
        body: JSON.stringify({ expected_version: client.secret_version }),
      });
      setClients((items) => items.map((item) => item.client_id === clientID ? { ...item, secret_available: true, secret_version: data.secret_version } : item));
      if (memoryGeneration === secretMemoryGeneration.current && canRetainClientSecret()) {
        setClientSecrets((secrets) => ({ ...secrets, [clientID]: data.client_secret }));
        setVisibleSecrets((visible) => ({ ...visible, [clientID]: true }));
      }
      toast.success(data.message || "Client secret berhasil di-regenerate.");
    } catch (error) {
      toast.error(getErrorMessage(error));
    } finally {
      setRegeneratingSecretID(null);
    }
  }

  async function openAccessDialog(client: OAuthClient) {
    setAccessClient(client);
    setAssignmentIdentifier("");
    setAccessLoading(true);
    try {
      const data = await apiFetch<{ client: OAuthClient; assignments: OAuthClientAssignment[] }>(`/api/clients/${encodeURIComponent(client.client_id)}/assignments`);
      setAssignments(data.assignments);
      setClients((items) => items.map((item) => item.client_id === client.client_id ? { ...item, ...data.client } : item));
      setAccessClient(data.client);
    } catch (error) {
      toast.error(getErrorMessage(error));
      setAccessClient(null);
    } finally {
      setAccessLoading(false);
    }
  }

  async function saveAssignment(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const identifier = assignmentIdentifier.trim();
    if (!accessClient || !identifier) return;
    setAssignmentSubmitting(true);
    try {
      const data = await apiFetch<{ assignment: OAuthClientAssignment }>(`/api/clients/${encodeURIComponent(accessClient.client_id)}/assignments`, {
        method: "POST",
        body: JSON.stringify({ identifier }),
      });
      const existed = assignments.some((item) => item.user_id === data.assignment.user_id);
      setAssignments((items) => existed ? items.map((item) => item.user_id === data.assignment.user_id ? data.assignment : item) : [...items, data.assignment]);
      if (!existed) {
        setClients((items) => items.map((item) => item.client_id === accessClient.client_id ? { ...item, assignment_count: item.assignment_count + 1 } : item));
      }
      setAssignmentIdentifier("");
      toast.success("Akses pengguna berhasil disimpan.");
    } catch (error) {
      toast.error(getErrorMessage(error));
    } finally {
      setAssignmentSubmitting(false);
    }
  }

  async function removeAssignment(assignment: OAuthClientAssignment) {
    if (!accessClient) return;
    try {
      await apiFetch(`/api/clients/${encodeURIComponent(accessClient.client_id)}/assignments/${encodeURIComponent(assignment.user_id)}`, { method: "DELETE" });
      setAssignments((items) => items.filter((item) => item.user_id !== assignment.user_id));
      setClients((items) => items.map((item) => item.client_id === accessClient.client_id ? { ...item, assignment_count: Math.max(0, item.assignment_count - 1) } : item));
      toast.success("Akses pengguna berhasil dicabut.");
    } catch (error) {
      toast.error(getErrorMessage(error));
    }
  }

  return (
    <div className="space-y-7">
      <PageHeader
        title="Aplikasi OAuth"
        description="Buat dan kelola kredensial aplikasi milik Anda."
        action={<Button onClick={openCreateDialog}><Plus />Tambah aplikasi</Button>}
      />

      <Dialog open={dialogOpen} onOpenChange={handleDialogOpenChange}>
        <DialogContent className="max-h-[calc(100svh-2rem)] overflow-hidden p-0 sm:max-w-lg">
          <form onSubmit={submitClient} className="flex min-h-0 flex-col">
            <DialogHeader className="border-b px-5 py-5 pr-14 sm:px-6">
              <DialogTitle className="text-lg">{editingClient ? "Edit aplikasi" : "Tambah aplikasi"}</DialogTitle>
              <DialogDescription>
                {editingClient ? "Perbarui nama, deskripsi, atau alamat callback aplikasi." : "Isi identitas aplikasi dan alamat callback yang akan menerima kode login."}
              </DialogDescription>
            </DialogHeader>

            <div className="grid min-w-0 gap-5 overflow-y-auto px-5 py-5 sm:px-6">
              <div className="space-y-2">
                <Label htmlFor="client-name">Nama aplikasi</Label>
                <Input id="client-name" value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} minLength={2} maxLength={120} placeholder="Contoh: Portal Komisariat" required autoFocus />
              </div>
              <div className="space-y-2">
                <Label htmlFor="client-description">Deskripsi <span className="font-normal text-muted-foreground">(opsional)</span></Label>
                <Textarea id="client-description" value={form.description} onChange={(event) => setForm({ ...form, description: event.target.value })} maxLength={500} rows={3} placeholder="Kegunaan singkat aplikasi" />
              </div>
              <div className="space-y-2">
                <Label htmlFor="access-policy">Siapa yang dapat masuk</Label>
                <Select value={form.accessPolicy} onValueChange={(value: OAuthClient["access_policy"]) => setForm({ ...form, accessPolicy: value })}>
                  <SelectTrigger id="access-policy" className="w-full"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="assigned_only">Hanya pengguna yang ditugaskan</SelectItem>
                    <SelectItem value="all_active_users">Semua akun aktif</SelectItem>
                  </SelectContent>
                </Select>
                <p className="text-xs leading-relaxed text-muted-foreground">Pilih assignment untuk aplikasi internal atau sensitif. Pemilik aplikasi otomatis mendapat akses.</p>
              </div>
              <fieldset className="min-w-0 space-y-3">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <Label asChild><legend>Redirect URI</legend></Label>
                  <Button type="button" variant="outline" size="sm" onClick={addRedirectURI}><Plus />Tambah URI</Button>
                </div>
                <div className="space-y-3">
                  {form.redirectUris.map((uri, index) => (
                    <div key={index} className="space-y-1.5">
                      <div className="flex min-w-0 items-start gap-2">
                        <Input
                          id={`redirect-uri-${index}`}
                          type="url"
                          inputMode="url"
                          autoCapitalize="none"
                          autoCorrect="off"
                          spellCheck={false}
                          value={uri}
                          onChange={(event) => updateRedirectURI(index, event.target.value)}
                          placeholder={index === 0 ? "http://localhost:4000/callback" : "https://app.example.com/callback"}
                          aria-label={`Redirect URI ${index + 1}`}
                          aria-invalid={Boolean(redirectURIErrors[index])}
                          aria-describedby={redirectURIErrors[index] ? `redirect-uri-error-${index}` : undefined}
                          required
                        />
                        {form.redirectUris.length > 1 && (
                          <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            className="shrink-0 text-destructive hover:bg-destructive/10 hover:text-destructive"
                            onClick={() => removeRedirectURI(index)}
                            aria-label={`Hapus Redirect URI ${index + 1}`}
                          >
                            <Trash2 />
                          </Button>
                        )}
                      </div>
                      {redirectURIErrors[index] && <p id={`redirect-uri-error-${index}`} className="text-xs text-destructive">{redirectURIErrors[index]}</p>}
                    </div>
                  ))}
                </div>
                <p className="text-xs leading-relaxed text-muted-foreground">Gunakan HTTPS di produksi. HTTP hanya dapat digunakan untuk localhost.</p>
              </fieldset>
            </div>

            <DialogFooter className="mx-0 mb-0 shrink-0 border-t px-5 py-4 sm:px-6">
              <Button type="button" variant="outline" onClick={() => handleDialogOpenChange(false)}>Batal</Button>
              <Button disabled={submitting}>{submitting && <Spinner />}{submitting ? "Menyimpan..." : editingClient ? "Simpan perubahan" : "Tambah aplikasi"}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(accessClient)} onOpenChange={(open) => { if (!open && !assignmentSubmitting) { setAccessClient(null); setAssignmentIdentifier(""); } }}>
        <DialogContent className="max-h-[calc(100svh-2rem)] overflow-hidden p-0 sm:max-w-2xl">
          <DialogHeader className="border-b px-5 py-5 pr-14 sm:px-6">
            <DialogTitle className="text-lg">Akses aplikasi {accessClient?.name}</DialogTitle>
            <DialogDescription>Atur siapa yang boleh menyelesaikan login SSO ke aplikasi ini.</DialogDescription>
          </DialogHeader>
          <div className="grid min-h-0 gap-5 overflow-y-auto px-5 py-5 sm:px-6">
            <div className="rounded-lg border bg-muted/20 p-4 text-sm">
              <div className="flex items-center gap-2 font-medium"><ShieldCheck className="size-4 text-primary" />{accessClient?.access_policy === "all_active_users" ? "Semua akun aktif dapat masuk" : "Default deny: hanya assignment aktif dapat masuk"}</div>
              <p className="mt-1 text-xs leading-relaxed text-muted-foreground">SSO hanya menentukan siapa yang boleh masuk. Role dan permission bisnis dikelola sendiri oleh aplikasi tujuan.</p>
            </div>

            <form onSubmit={saveAssignment} className="grid gap-4 rounded-lg border p-4">
              <div className="flex items-center gap-2 font-medium"><UserPlus className="size-4" />Tambahkan akses pengguna</div>
              <div className="space-y-2">
                <Label htmlFor="assignment-identifier">ID pengguna atau email</Label>
                <Input id="assignment-identifier" value={assignmentIdentifier} onChange={(event) => setAssignmentIdentifier(event.target.value)} placeholder="UUID pengguna atau anggota@example.com" autoComplete="off" required />
                <p className="text-xs leading-relaxed text-muted-foreground">Pencocokan harus persis. Hanya akun aktif dengan email terverifikasi yang dapat ditambahkan.</p>
              </div>
              <div className="flex flex-wrap justify-end gap-2"><Button type="button" variant="outline" onClick={() => setAssignmentIdentifier("")}>Bersihkan</Button><Button disabled={!assignmentIdentifier.trim() || assignmentSubmitting}>{assignmentSubmitting && <Spinner />}Tambahkan akses</Button></div>
            </form>

            <div className="space-y-3">
              <div><p className="font-medium">Pengguna yang ditugaskan</p><p className="text-xs text-muted-foreground">Perubahan assignment langsung mencabut token dan authorization code lama pengguna tersebut.</p></div>
              {accessLoading && <div className="flex justify-center py-8"><Spinner /></div>}
              {!accessLoading && assignments.length === 0 && <div className="rounded-lg border border-dashed p-8 text-center text-sm text-muted-foreground">Belum ada assignment.</div>}
              {assignments.map((assignment) => (
                <div key={assignment.id} className="flex flex-col gap-3 rounded-lg border p-4 sm:flex-row sm:items-center sm:justify-between">
                  <div className="min-w-0">
                    <p className="truncate font-medium">{assignment.name}</p><p className="truncate text-xs text-muted-foreground">{assignment.email}</p>
                    <p className="mt-1 truncate font-mono text-[11px] text-muted-foreground">{assignment.user_id}</p>
                  </div>
                  <AlertDialog>
                    <AlertDialogTrigger asChild><Button type="button" size="sm" variant="outline" className="text-destructive hover:text-destructive"><Trash2 />Cabut</Button></AlertDialogTrigger>
                    <AlertDialogContent><AlertDialogHeader><AlertDialogTitle>Cabut akses pengguna?</AlertDialogTitle><AlertDialogDescription>{assignment.name} tidak dapat login lagi ke aplikasi ini. Token dan authorization code aktifnya juga dicabut.</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel>Batal</AlertDialogCancel><AlertDialogAction variant="destructive" onClick={() => removeAssignment(assignment)}>Cabut akses</AlertDialogAction></AlertDialogFooter></AlertDialogContent>
                  </AlertDialog>
                </div>
              ))}
            </div>
          </div>
        </DialogContent>
      </Dialog>

      <div className="grid gap-4 md:grid-cols-2">
        {loading && [1, 2].map((item) => <OAuthClientCardSkeleton key={item} />)}
        {!loading && clients.length === 0 && (
          <Card className="md:col-span-2"><CardContent><Empty><EmptyHeader><EmptyMedia variant="icon"><AppWindow /></EmptyMedia><EmptyTitle>Belum ada aplikasi</EmptyTitle><EmptyDescription>Tambahkan aplikasi pertama untuk memulai integrasi SSO.</EmptyDescription></EmptyHeader></Empty></CardContent></Card>
        )}
        {clients.map((client) => (
          <Card key={client.client_id}>
            <CardHeader>
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0"><CardTitle className="truncate">{client.name}</CardTitle><CardDescription className="mt-1 line-clamp-2">{client.description || "Tanpa deskripsi"}</CardDescription><div className="mt-2 flex flex-wrap gap-1.5"><Badge variant={client.access_policy === "assigned_only" ? "default" : "secondary"}>{client.access_policy === "assigned_only" ? "Assignment wajib" : "Semua akun aktif"}</Badge><Badge variant="outline">{client.assignment_count ?? 0} pengguna</Badge></div></div>
                <div className="flex shrink-0 gap-1">
                  <Button variant="ghost" size="icon" onClick={() => openEditDialog(client)} aria-label={`Edit ${client.name}`}><Pencil /></Button>
                  <AlertDialog>
                    <AlertDialogTrigger asChild><Button variant="ghost" size="icon" aria-label={`Hapus ${client.name}`}><Trash2 /></Button></AlertDialogTrigger>
                    <AlertDialogContent><AlertDialogHeader><AlertDialogTitle>Hapus aplikasi?</AlertDialogTitle><AlertDialogDescription>Aplikasi, authorization code, access token, dan refresh token terkait akan dihapus. Tindakan ini tidak dapat dibatalkan.</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel>Batal</AlertDialogCancel><AlertDialogAction variant="destructive" onClick={() => deleteClient(client)}>Hapus aplikasi</AlertDialogAction></AlertDialogFooter></AlertDialogContent>
                  </AlertDialog>
                </div>
              </div>
            </CardHeader>
            <CardContent className="space-y-5">
              <Button variant="outline" className="w-full" onClick={() => openAccessDialog(client)}><Users />Kelola akses pengguna</Button>
              <div>
                <p className="text-xs font-medium text-muted-foreground">CLIENT ID</p>
                <div className="mt-1 flex min-w-0 items-center gap-2"><code className="min-w-0 flex-1 truncate rounded-md bg-muted p-2 text-xs">{client.client_id}</code><Button size="icon" variant="outline" onClick={() => copy(client.client_id)} aria-label="Salin client ID"><Copy /></Button></div>
              </div>
              <div>
                <p className="text-xs font-medium text-muted-foreground">CLIENT SECRET</p>
                <div className="mt-1 flex min-w-0 items-center gap-2">
                  <code className="min-w-0 flex-1 truncate rounded-md bg-muted p-2 text-xs" aria-label={visibleSecrets[client.client_id] ? "Client secret terlihat" : "Client secret disembunyikan"}>{visibleSecrets[client.client_id] && clientSecrets[client.client_id] ? clientSecrets[client.client_id] : "••••••••••••••••••••••••••••••••"}</code>
                  <Button size="icon" variant="outline" onClick={() => toggleClientSecret(client)} disabled={loadingSecretID === client.client_id || regeneratingSecretID === client.client_id || !client.secret_available} aria-label={visibleSecrets[client.client_id] ? "Sembunyikan client secret" : "Lihat client secret"}>{loadingSecretID === client.client_id ? <Spinner /> : visibleSecrets[client.client_id] ? <EyeOff /> : <Eye />}</Button>
                  <Button size="icon" variant="outline" onClick={() => copy(clientSecrets[client.client_id])} disabled={!clientSecrets[client.client_id]} aria-label="Salin client secret"><Copy /></Button>
                </div>
                {!client.secret_available && <p className="mt-2 text-xs leading-relaxed text-muted-foreground">Secret lama tidak dapat dipulihkan. Regenerate untuk membuat secret baru.</p>}
                <AlertDialog>
                  <AlertDialogTrigger asChild><Button variant="outline" size="sm" className="mt-3 w-full sm:w-auto" disabled={loadingSecretID === client.client_id || regeneratingSecretID === client.client_id}><RefreshCw />Regenerate secret</Button></AlertDialogTrigger>
                  <AlertDialogContent><AlertDialogHeader><AlertDialogTitle>Regenerate client secret?</AlertDialogTitle><AlertDialogDescription>Secret lama langsung tidak berlaku. Semua token dan authorization code aplikasi ini akan dicabut sehingga aplikasi harus dihubungkan ulang.</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel>Batal</AlertDialogCancel><AlertDialogAction variant="destructive" disabled={regeneratingSecretID === client.client_id} onClick={() => regenerateClientSecret(client)}>{regeneratingSecretID === client.client_id && <Spinner />}Regenerate dan cabut akses lama</AlertDialogAction></AlertDialogFooter></AlertDialogContent>
                </AlertDialog>
              </div>
              <div>
                <p className="text-xs font-medium text-muted-foreground">REDIRECT URI</p>
                <div className="mt-2 space-y-1.5">{client.redirect_uris.map((uri) => <p key={uri} className="break-all rounded-md border bg-muted/30 px-2.5 py-2 text-xs">{uri}</p>)}</div>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
}
