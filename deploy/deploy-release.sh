#!/usr/bin/env bash
set -euo pipefail

TARGET_HOST="${1:-ubuntu@43.157.224.6}"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPOSITORY_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"
FRONTEND_DIR="${REPOSITORY_ROOT}/frontend"
DOCUMENTATION_DIR="${REPOSITORY_ROOT}/documentation"
RELEASE_ID="$(date +%Y%m%d%H%M%S)"
REMOTE_ARCHIVE="/tmp/ipnu-sso-release-${RELEASE_ID}.tar.gz"
STAGING_DIR="$(mktemp -d "/tmp/ipnu-sso-release-${RELEASE_ID}.XXXXXX")"

cleanup() {
  if [[ -n "${STAGING_DIR:-}" && -d "${STAGING_DIR}" ]]; then
    rm -rf -- "${STAGING_DIR}"
  fi
}
trap cleanup EXIT

if [[ ! "${TARGET_HOST}" =~ ^[A-Za-z0-9._-]+@[A-Za-z0-9.-]+$ ]]; then
  echo "Target SSH tidak valid. Gunakan format pengguna@hostname." >&2
  exit 1
fi

for command_name in git go node npm ssh scp tar; do
  if ! command -v "${command_name}" >/dev/null 2>&1; then
    echo "Perintah ${command_name} tidak tersedia." >&2
    exit 1
  fi
done

cd "${REPOSITORY_ROOT}"

if [[ -n "$(git status --porcelain)" ]]; then
  echo "Working tree belum bersih. Commit atau batalkan perubahan sebelum deploy." >&2
  git status --short >&2
  exit 1
fi

current_branch="$(git branch --show-current)"
if [[ -z "${current_branch}" ]]; then
  echo "Deploy tidak boleh dijalankan dari detached HEAD." >&2
  exit 1
fi

upstream_ref="$(git rev-parse --abbrev-ref --symbolic-full-name '@{upstream}' 2>/dev/null || true)"
if [[ -z "${upstream_ref}" ]]; then
  echo "Branch ${current_branch} belum memiliki upstream Git." >&2
  exit 1
fi

echo "[1/7] Memastikan commit lokal sudah tersedia di ${upstream_ref}..."
git fetch --quiet
local_commit="$(git rev-parse HEAD)"
upstream_commit="$(git rev-parse '@{upstream}')"
if [[ "${local_commit}" != "${upstream_commit}" ]]; then
  echo "HEAD lokal belum sama dengan ${upstream_ref}. Push/pull perubahan terlebih dahulu." >&2
  echo "Lokal:    ${local_commit}" >&2
  echo "Upstream: ${upstream_commit}" >&2
  exit 1
fi

echo "[2/7] Memeriksa koneksi dan prasyarat VPS ${TARGET_HOST}..."
ssh -o BatchMode=yes -o ConnectTimeout=10 "${TARGET_HOST}" \
  sudo -n bash -s <<'REMOTE_PREFLIGHT'
set -euo pipefail
test -d /opt/ipnu-sso/releases
test -L /opt/ipnu-sso/current
test -s /etc/ipnu-sso/backend.env
test -s /etc/ipnu-sso/frontend.env
test -s /etc/ipnu-sso/deploy.env
systemctl is-enabled --quiet ipnu-sso-backend.service
systemctl is-enabled --quiet ipnu-sso-frontend.service
systemctl is-active --quiet nginx
REMOTE_PREFLIGHT

echo "[3/7] Menguji dan membangun backend, frontend, serta dokumentasi..."
BACKEND_BINARY="${STAGING_DIR}/sso-backend"
(
  cd "${REPOSITORY_ROOT}/backend"
  go test ./...
  go vet ./...
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o "${BACKEND_BINARY}" .
)
(
  cd "${FRONTEND_DIR}"
  npm ci
  npm run lint -- --max-warnings=0
  npm run build:production
)
(
  cd "${DOCUMENTATION_DIR}"
  npm ci
  npm run typecheck
  npm run build
)

BUILD_ID="$(tr -d '\r\n' < "${FRONTEND_DIR}/.next/BUILD_ID")"
if [[ -z "${BUILD_ID}" ]]; then
  echo "BUILD_ID frontend kosong." >&2
  exit 1
fi
if [[ ! -s "${DOCUMENTATION_DIR}/build/index.html" ]]; then
  echo "Artifact dokumentasi tidak memiliki index.html." >&2
  exit 1
fi

echo "[4/7] Menyiapkan artifact release ${RELEASE_ID}..."
install -d \
  "${STAGING_DIR}/backend" \
  "${STAGING_DIR}/frontend/.next" \
  "${STAGING_DIR}/docs" \
  "${STAGING_DIR}/deploy"
install -m 0755 "${BACKEND_BINARY}" "${STAGING_DIR}/backend/sso-backend"
cp -a "${FRONTEND_DIR}/.next/standalone/." "${STAGING_DIR}/frontend/"
cp -a "${FRONTEND_DIR}/.next/static" "${STAGING_DIR}/frontend/.next/static"
cp -a "${FRONTEND_DIR}/public" "${STAGING_DIR}/frontend/public"
cp -a "${DOCUMENTATION_DIR}/build/." "${STAGING_DIR}/docs/"
cp \
  "${SCRIPT_DIR}/verify-frontend-artifact.sh" \
  "${SCRIPT_DIR}/url-config.sh" \
  "${STAGING_DIR}/deploy/"

# Build Next.js di macOS dapat menelusuri modul native macOS. Runtime Linux
# dipasang eksplisit, dengan versi dan SHA-512 yang dikunci package-lock.json.
node --input-type=module - "${FRONTEND_DIR}" "${STAGING_DIR}" <<'NATIVE_RUNTIME'
import { createHash } from 'node:crypto';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';
const [source, staging] = process.argv.slice(2);
const lock = JSON.parse(readFileSync(join(source, 'package-lock.json'), 'utf8'));
const sharp = JSON.parse(readFileSync(join(staging, 'frontend/node_modules/sharp/package.json'), 'utf8'));
for (const name of ['sharp-linux-x64', 'sharp-libvips-linux-x64']) {
  const pkg = `@img/${name}`;
  const entry = lock.packages[`node_modules/${pkg}`];
  if (!entry || entry.version !== sharp.optionalDependencies[pkg] ||
      !entry.resolved.startsWith(`https://registry.npmjs.org/${pkg}/-/`) ||
      !entry.integrity.startsWith('sha512-')) {
    throw new Error(`Lockfile tidak cocok dengan native runtime ${pkg}`);
  }
  const response = await fetch(entry.resolved, { signal: AbortSignal.timeout(120000) });
  if (!response.ok) throw new Error(`Download native runtime gagal: ${response.status}`);
  const bytes = Buffer.from(await response.arrayBuffer());
  const integrity = `sha512-${createHash('sha512').update(bytes).digest('base64')}`;
  if (integrity !== entry.integrity) throw new Error(`Integritas ${pkg} tidak cocok`);
  const archive = join(staging, `${name}.tgz`);
  const destination = join(staging, 'frontend/node_modules/@img', name);
  writeFileSync(archive, bytes, { flag: 'wx', mode: 0o600 });
  mkdirSync(destination, { recursive: true });
  const result = spawnSync('tar', ['-xzf', archive, '-C', destination, '--strip-components=1'], { stdio: 'inherit' });
  if (result.status !== 0) throw new Error(`Ekstraksi ${pkg} gagal`);
}
console.log('Native runtime Linux x64 terpasang; versi dan SHA-512 sesuai lockfile.');
NATIVE_RUNTIME

bash "${SCRIPT_DIR}/verify-frontend-artifact.sh" \
  "${STAGING_DIR}/frontend" \
  "https://api.pelajarnumagetan.id" \
  "https://doc.pelajarnumagetan.id"

node --input-type=module - \
  "${STAGING_DIR}/release.json" \
  "${RELEASE_ID}" \
  "${local_commit}" \
  "${current_branch}" \
  "${BUILD_ID}" <<'RELEASE_METADATA'
import { writeFileSync } from 'node:fs';
const [path, releaseID, gitCommit, gitBranch, frontendBuildID] = process.argv.slice(2);
writeFileSync(path, `${JSON.stringify({
  release_id: releaseID,
  git_commit: gitCommit,
  git_branch: gitBranch,
  frontend_build_id: frontendBuildID,
}, null, 2)}\n`, { flag: 'wx', mode: 0o600 });
RELEASE_METADATA

COPYFILE_DISABLE=1 tar --no-xattrs \
  -C "${STAGING_DIR}" \
  -czf "${STAGING_DIR}/release.tar.gz" \
  backend frontend docs deploy release.json

echo "[5/7] Mengunggah artifact ke VPS..."
scp -q "${STAGING_DIR}/release.tar.gz" "${TARGET_HOST}:${REMOTE_ARCHIVE}"

echo "[6/7] Mengaktifkan release dan menjalankan pemeriksaan kesehatan..."
ssh "${TARGET_HOST}" sudo -n bash -s -- \
  "${RELEASE_ID}" "${REMOTE_ARCHIVE}" "${BUILD_ID}" "${local_commit}" <<'REMOTE_SCRIPT'
set -euo pipefail

release_id="${1:?Release ID wajib diberikan}"
remote_archive="${2:?Path artifact wajib diberikan}"
expected_build_id="${3:?BUILD_ID wajib diberikan}"
expected_commit="${4:?Commit Git wajib diberikan}"
app_root="/opt/ipnu-sso"
config_root="/etc/ipnu-sso"
new_release="${app_root}/releases/${release_id}"

if [[ ! "${release_id}" =~ ^[0-9]{14}$ ]]; then
  echo "Release ID tidak valid." >&2
  exit 1
fi
if [[ ! "${expected_commit}" =~ ^[a-f0-9]{40}$ ]]; then
  echo "Commit Git tidak valid." >&2
  exit 1
fi
if [[ "${remote_archive}" != "/tmp/ipnu-sso-release-${release_id}.tar.gz" ]]; then
  echo "Path artifact tidak valid." >&2
  exit 1
fi

current_release="$(readlink -f "${app_root}/current")"
case "${current_release}" in
  "${app_root}/releases/"*) ;;
  *)
    echo "Target release aktif tidak valid: ${current_release}" >&2
    exit 1
    ;;
esac
if [[ -e "${new_release}" ]]; then
  echo "Release ${release_id} sudah ada." >&2
  exit 1
fi
if [[ ! -s "${remote_archive}" ]]; then
  echo "Artifact upload tidak ditemukan atau kosong." >&2
  exit 1
fi

activated=0
rollback() {
  exit_code=$?
  trap - EXIT
  if [[ "${exit_code}" -eq 0 ]]; then
    return
  fi
  if [[ "${activated}" -eq 1 ]]; then
    echo "Deploy gagal; mengembalikan release ${current_release}." >&2
    ln -sfn "${current_release}" "${app_root}/current"
    systemctl restart ipnu-sso-backend.service ipnu-sso-frontend.service || true
  fi
  rm -f -- "${remote_archive}"
  rm -rf -- "${new_release}"
  exit "${exit_code}"
}
trap rollback EXIT

install -d -m 0755 "${new_release}"
tar -xzf "${remote_archive}" -C "${new_release}"

for required_path in \
  backend/sso-backend \
  frontend/server.js \
  frontend/.next/BUILD_ID \
  frontend/public/images/logo-sso.png \
  docs/index.html \
  deploy/verify-frontend-artifact.sh \
  deploy/url-config.sh \
  release.json; do
  if [[ ! -s "${new_release}/${required_path}" ]]; then
    echo "Artifact tidak lengkap: ${required_path}" >&2
    exit 1
  fi
done

actual_build_id="$(tr -d '\r\n' < "${new_release}/frontend/.next/BUILD_ID")"
if [[ "${actual_build_id}" != "${expected_build_id}" ]]; then
  echo "BUILD_ID artifact tidak cocok." >&2
  exit 1
fi
if ! grep -Fq "\"git_commit\": \"${expected_commit}\"" "${new_release}/release.json"; then
  echo "Metadata commit artifact tidak cocok." >&2
  exit 1
fi

chown -R root:ipnu-sso "${new_release}"
find "${new_release}" -type d -exec chmod 0755 {} +
find "${new_release}" -type f -exec chmod 0644 {} +
chmod 0755 "${new_release}/backend/sso-backend"
find "${new_release}/deploy" -type f -name '*.sh' -exec chmod 0755 {} +
install -d -o ipnu-sso -g ipnu-sso -m 0750 \
  "${new_release}/frontend/.next/cache"

bash "${new_release}/deploy/verify-frontend-artifact.sh" \
  "${new_release}/frontend" \
  "https://api.pelajarnumagetan.id" \
  "https://doc.pelajarnumagetan.id"
(
  cd "${new_release}/frontend"
  sudo -u ipnu-sso node -e '
    require("next");
    require("sharp")("public/images/logo-sso.png").metadata()
      .then(() => console.log("Runtime frontend siap di VPS."))
      .catch(() => process.exit(1));
  '
)

source "${new_release}/deploy/url-config.sh"
load_upstream_configuration "${config_root}/deploy.env"

for config_file in backend.env frontend.env deploy.env oidc-private.pem; do
  if [[ ! -s "${config_root}/${config_file}" ]]; then
    echo "Konfigurasi produksi tidak lengkap: ${config_file}" >&2
    exit 1
  fi
done

nginx -t
ln -sfn "${new_release}" "${app_root}/current"
activated=1
systemctl restart ipnu-sso-backend.service ipnu-sso-frontend.service

healthy=0
for _ in $(seq 1 30); do
  if curl --fail --silent --max-time 5 \
      "${BACKEND_UPSTREAM_URL%/}/ready" >/dev/null 2>&1 \
    && curl --fail --silent --max-time 5 \
      "${FRONTEND_UPSTREAM_URL%/}/login" >/dev/null 2>&1; then
    healthy=1
    break
  fi
  sleep 1
done
if [[ "${healthy}" -ne 1 ]]; then
  echo "Health check service gagal." >&2
  systemctl --no-pager --full status \
    ipnu-sso-backend.service ipnu-sso-frontend.service >&2 || true
  exit 1
fi

systemctl is-active --quiet ipnu-sso-backend.service
systemctl is-active --quiet ipnu-sso-frontend.service
test -s "${app_root}/current/docs/index.html"
nginx -t

public_ready="$(curl --fail --silent --show-error --max-time 15 \
  https://api.pelajarnumagetan.id/ready)"
if [[ "${public_ready}" != *'"status":"READY"'* ]]; then
  echo "Endpoint API publik gagal setelah aktivasi." >&2
  exit 1
fi
curl --fail --silent --show-error --max-time 15 \
  https://pelajarnumagetan.id/login >/dev/null
curl --fail --silent --show-error --max-time 15 \
  https://doc.pelajarnumagetan.id/ >/dev/null

trap - EXIT
rm -f -- "${remote_archive}"

# Simpan lima release terakhir. Nama wajib timestamp 14 digit dan release
# aktif tidak pernah dihapus.
mapfile -t releases < <(
  find "${app_root}/releases" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' |
    grep -E '^[0-9]{14}$' |
    sort -r
)
if (( ${#releases[@]} > 5 )); then
  for old_release_id in "${releases[@]:5}"; do
    old_release="${app_root}/releases/${old_release_id}"
    if [[ "$(readlink -f "${app_root}/current")" != "${old_release}" ]]; then
      rm -rf -- "${old_release}"
    fi
  done
fi

echo "RELEASE_OK=${release_id} COMMIT=${expected_commit}"
REMOTE_SCRIPT

echo "[7/7] Memeriksa endpoint publik..."
public_ready="$(curl --fail --silent --show-error --max-time 15 \
  https://api.pelajarnumagetan.id/ready)"
if [[ "${public_ready}" != *'"status":"READY"'* ]]; then
  echo "Endpoint API publik tidak mengarah ke backend release ini." >&2
  exit 1
fi
public_frontend="$(curl --fail --silent --show-error --max-time 15 \
  https://pelajarnumagetan.id/login)"
if [[ "${public_frontend}" != *'PelajarNU Magetan ID'* ]]; then
  echo "Domain frontend publik tidak mengarah ke frontend release ini." >&2
  exit 1
fi
curl --fail --silent --show-error --max-time 15 \
  https://doc.pelajarnumagetan.id/ >/dev/null

echo "DEPLOY_OK release=${RELEASE_ID} commit=${local_commit} build=${BUILD_ID}"
