import "server-only";

function requiredServerValue(name: string, value: string | undefined) {
  const normalized = value?.trim();
  if (!normalized) {
    throw new Error(`${name} wajib diatur melalui environment frontend.`);
  }
  return normalized;
}

export const BACKEND_SESSION_COOKIE_NAME = requiredServerValue(
  "BACKEND_SESSION_COOKIE_NAME",
  process.env.BACKEND_SESSION_COOKIE_NAME,
);
