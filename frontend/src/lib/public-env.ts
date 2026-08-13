function requiredPublicValue(name: string, value: string | undefined) {
  const normalized = value?.trim().replace(/\/$/, "");
  if (!normalized) {
    throw new Error(`${name} wajib diatur melalui environment frontend.`);
  }
  return normalized;
}

export const PUBLIC_BACKEND_URL = requiredPublicValue(
  "NEXT_PUBLIC_BACKEND_URL",
  process.env.NEXT_PUBLIC_BACKEND_URL,
);

export const PUBLIC_DOCUMENTATION_URL = requiredPublicValue(
  "NEXT_PUBLIC_DOCUMENTATION_URL",
  process.env.NEXT_PUBLIC_DOCUMENTATION_URL,
);

export const PUBLIC_APP_NAME = requiredPublicValue(
  "NEXT_PUBLIC_APP_NAME",
  process.env.NEXT_PUBLIC_APP_NAME,
);

export const PUBLIC_APP_TAGLINE = requiredPublicValue(
  "NEXT_PUBLIC_APP_TAGLINE",
  process.env.NEXT_PUBLIC_APP_TAGLINE,
);

export const PUBLIC_APP_DESCRIPTION = requiredPublicValue(
  "NEXT_PUBLIC_APP_DESCRIPTION",
  process.env.NEXT_PUBLIC_APP_DESCRIPTION,
);
