// SQL is larger than the encrypted R2 archive. Validate the actual response
// length, never the archive's metadata size, before offering the file to save.
export async function readBackupSQL(response: Response): Promise<Blob> {
  if (!response.ok) {
    const payload = await response.json().catch(() => null);
    throw new Error(payload?.message ?? "Unduhan belum dapat diproses.");
  }
  const expectedSize = Number(response.headers.get("X-Backup-SQL-Size") ?? response.headers.get("Content-Length"));
  if (!response.headers.get("Content-Type")?.startsWith("application/sql") || !Number.isSafeInteger(expectedSize) || expectedSize <= 0) {
    throw new Error("Respons unduhan SQL tidak valid. Silakan coba lagi.");
  }
  const blob = await response.blob();
  if (blob.size !== expectedSize) {
    throw new Error("Unduhan SQL terputus. Silakan coba lagi.");
  }
  return blob;
}
