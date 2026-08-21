import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));
const frontendDirectory = path.resolve(scriptDirectory, "..");
const repositoryDirectory = path.resolve(frontendDirectory, "..");
const productionEnvPath = path.join(frontendDirectory, ".env.production");

function parseEnvironment(source) {
  const result = {};
  for (const [index, rawLine] of source.split(/\r?\n/).entries()) {
    const line = rawLine.trim();
    if (!line || line.startsWith("#")) continue;

    const match = line.match(/^([A-Z][A-Z0-9_]*)=(.*)$/);
    if (!match) {
      throw new Error(
        `Format .env.production tidak valid pada baris ${index + 1}.`,
      );
    }

    let value = match[2].trim();
    if (
      value.length >= 2 &&
      ((value.startsWith('"') && value.endsWith('"')) ||
        (value.startsWith("'") && value.endsWith("'")))
    ) {
      value = value.slice(1, -1);
    }
    result[match[1]] = value;
  }
  return result;
}

function requireHttpsUrl(name, value) {
  if (!value) throw new Error(`${name} wajib diatur di .env.production.`);

  const parsed = new URL(value);
  const localHosts = new Set(["localhost", "127.0.0.1", "::1"]);
  if (parsed.protocol !== "https:" || localHosts.has(parsed.hostname)) {
    throw new Error(`${name} harus menggunakan URL HTTPS non-localhost.`);
  }
  return value.replace(/\/$/, "");
}

const productionEnvironment = parseEnvironment(
  readFileSync(productionEnvPath, "utf8"),
);
const requiredPublicVariables = [
  "NEXT_PUBLIC_BACKEND_URL",
  "NEXT_PUBLIC_DOCUMENTATION_URL",
  "NEXT_PUBLIC_APP_NAME",
  "NEXT_PUBLIC_APP_TAGLINE",
  "NEXT_PUBLIC_APP_DESCRIPTION",
  "NEXT_PUBLIC_ORGANIZATION_NAME",
];

for (const name of requiredPublicVariables) {
  if (!productionEnvironment[name]?.trim()) {
    throw new Error(`${name} wajib diatur di .env.production.`);
  }
}

const backendUrl = requireHttpsUrl(
  "NEXT_PUBLIC_BACKEND_URL",
  productionEnvironment.NEXT_PUBLIC_BACKEND_URL,
);
const documentationUrl = requireHttpsUrl(
  "NEXT_PUBLIC_DOCUMENTATION_URL",
  productionEnvironment.NEXT_PUBLIC_DOCUMENTATION_URL,
);

const nextBinary = path.join(
  frontendDirectory,
  "node_modules",
  "next",
  "dist",
  "bin",
  "next",
);
const build = spawnSync(process.execPath, [nextBinary, "build", "--webpack"], {
  cwd: frontendDirectory,
  env: {
    ...process.env,
    ...productionEnvironment,
    NODE_ENV: "production",
    NEXT_PUBLIC_BACKEND_URL: backendUrl,
    NEXT_PUBLIC_DOCUMENTATION_URL: documentationUrl,
  },
  stdio: "inherit",
});
if (build.status !== 0) process.exit(build.status ?? 1);

const validator = path.join(
  repositoryDirectory,
  "deploy",
  "verify-frontend-artifact.sh",
);
const validation = spawnSync(
  "bash",
  [validator, frontendDirectory, backendUrl, documentationUrl],
  {
    cwd: repositoryDirectory,
    stdio: "inherit",
  },
);
process.exit(validation.status ?? 1);
