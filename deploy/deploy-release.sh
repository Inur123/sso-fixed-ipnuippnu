#!/usr/bin/env bash
set -euo pipefail

TARGET_HOST="${1:-34.50.73.15}"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPOSITORY_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"
FRONTEND_DIR="${REPOSITORY_ROOT}/frontend"
RELEASE_ID="$(date +%Y%m%d%H%M%S)"
REMOTE_ARCHIVE="/tmp/ipnu-sso-release-${RELEASE_ID}.tar.gz"
STAGING_DIR="$(mktemp -d "/tmp/ipnu-sso-release-${RELEASE_ID}.XXXXXX")"

cleanup() {
  if [[ -n "${STAGING_DIR:-}" && -d "${STAGING_DIR}" ]]; then
    rm -rf -- "${STAGING_DIR}"
  fi
}
trap cleanup EXIT

for command_name in go npm ssh scp tar; do
  if ! command -v "${command_name}" >/dev/null 2>&1; then
    echo "Perintah ${command_name} tidak tersedia." >&2
    exit 1
  fi
done

echo "[1/6] Memeriksa koneksi ke ${TARGET_HOST}..."
ssh -o BatchMode=yes -o ConnectTimeout=10 "${TARGET_HOST}" \
  'sudo test -d /opt/ipnu-sso/releases && sudo test -L /opt/ipnu-sso/current'

echo "[2/6] Menjalankan test backend, lint, dan build produksi..."
BACKEND_BINARY="${STAGING_DIR}/sso-backend"
(
  cd "${REPOSITORY_ROOT}/backend"
  go test ./...
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o "${BACKEND_BINARY}" .
)
(
  cd "${FRONTEND_DIR}"
  npm run lint -- --max-warnings=0
  npm run build:production
)

BUILD_ID="$(tr -d '\r\n' < "${FRONTEND_DIR}/.next/BUILD_ID")"
if [[ -z "${BUILD_ID}" ]]; then
  echo "BUILD_ID frontend kosong." >&2
  exit 1
fi

echo "[3/6] Menyiapkan artifact release ${RELEASE_ID}..."
install -d \
  "${STAGING_DIR}/backend" \
  "${STAGING_DIR}/frontend/.next" \
  "${STAGING_DIR}/deploy"
install -m 0755 "${BACKEND_BINARY}" "${STAGING_DIR}/backend/sso-backend"
cp -a "${FRONTEND_DIR}/.next/standalone/." "${STAGING_DIR}/frontend/"
cp -a "${FRONTEND_DIR}/.next/static" "${STAGING_DIR}/frontend/.next/static"
cp -a "${FRONTEND_DIR}/public" "${STAGING_DIR}/frontend/public"
cp \
  "${SCRIPT_DIR}/setup-vps.sh" \
  "${SCRIPT_DIR}/verify-frontend-artifact.sh" \
  "${STAGING_DIR}/deploy/"

bash "${SCRIPT_DIR}/verify-frontend-artifact.sh" \
  "${STAGING_DIR}/frontend" \
  "https://api.pelajarnumagetan.id" \
  "https://doc.pelajarnumagetan.id"
COPYFILE_DISABLE=1 tar --no-xattrs \
  -C "${STAGING_DIR}" \
  -czf "${STAGING_DIR}/release.tar.gz" \
  backend frontend deploy

echo "[4/6] Mengunggah artifact ke server..."
scp -q "${STAGING_DIR}/release.tar.gz" "${TARGET_HOST}:${REMOTE_ARCHIVE}"

echo "[5/6] Mengaktifkan release dan menjalankan pemeriksaan kesehatan..."
ssh "${TARGET_HOST}" sudo bash -s -- \
  "${RELEASE_ID}" "${REMOTE_ARCHIVE}" "${BUILD_ID}" <<'REMOTE_SCRIPT'
set -euo pipefail

release_id="${1:?Release ID wajib diberikan}"
remote_archive="${2:?Path artifact wajib diberikan}"
expected_build_id="${3:?BUILD_ID wajib diberikan}"
app_root="/opt/ipnu-sso"
config_root="/etc/ipnu-sso"

if [[ ! "${release_id}" =~ ^[0-9]{14}$ ]]; then
  echo "Release ID tidak valid." >&2
  exit 1
fi
if [[ "${remote_archive}" != "/tmp/ipnu-sso-release-${release_id}.tar.gz" ]]; then
  echo "Path artifact tidak valid." >&2
  exit 1
fi

current_release="$(readlink -f "${app_root}/current")"
new_release="${app_root}/releases/${release_id}"
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
  trap - ERR
  if [[ "${activated}" -eq 1 ]]; then
    echo "Deploy gagal; mengembalikan release ${current_release}." >&2
    ln -sfn "${current_release}" "${app_root}/current"
    systemctl daemon-reload || true
    systemctl restart ipnu-sso-backend.service ipnu-sso-frontend.service || true
    systemctl reload nginx || true
  fi
  rm -f -- "${remote_archive}"
  exit "${exit_code}"
}
trap rollback ERR

install -d -m 0755 "${new_release}"
cp -a "${current_release}/." "${new_release}/"

# Target berada pada release baru yang belum aktif. Frontend lama dibuang dari
# salinan ini saja; release aktif sebelumnya tetap utuh untuk rollback.
rm -rf -- "${new_release}/frontend"
tar -xzf "${remote_archive}" -C "${new_release}"

actual_build_id="$(tr -d '\r\n' < "${new_release}/frontend/.next/BUILD_ID")"
if [[ "${actual_build_id}" != "${expected_build_id}" ]]; then
  echo "BUILD_ID artifact tidak cocok." >&2
  exit 1
fi

ln -sfn "${new_release}" "${app_root}/current"
activated=1

read_env_value() {
  local key="${1:?Nama environment wajib diberikan}"
  awk -F= -v expected_key="${key}" '
    $1 == expected_key {
      sub(/^[^=]*=/, "")
      print
      exit
    }
  ' "${config_root}/backend.env"
}

admin_email="$(read_env_value SUPER_ADMIN_EMAIL)"
mail_username="$(read_env_value MAIL_USERNAME)"
mail_from_address="$(read_env_value MAIL_FROM_ADDRESS)"
if [[ -z "${admin_email}" || -z "${mail_username}" || -z "${mail_from_address}" ]]; then
  echo "Konfigurasi admin atau email production tidak lengkap." >&2
  exit 1
fi

env \
  DEPLOY_ADMIN_EMAIL="${admin_email}" \
  DEPLOY_MAIL_USERNAME="${mail_username}" \
  DEPLOY_MAIL_FROM_ADDRESS="${mail_from_address}" \
  bash "${new_release}/deploy/setup-vps.sh"

systemctl restart ipnu-sso-backend.service ipnu-sso-frontend.service

healthy=0
for _ in $(seq 1 30); do
  if curl --fail --silent --max-time 5 \
      http://127.0.0.1:8180/ready >/dev/null 2>&1 \
    && curl --fail --silent --max-time 5 \
      http://127.0.0.1:3100/login >/dev/null 2>&1; then
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
nginx -t

trap - ERR
rm -f -- "${remote_archive}"
echo "RELEASE_OK=${release_id}"
REMOTE_SCRIPT

echo "[6/6] Memeriksa endpoint publik..."
public_ready="$(curl --fail --silent --show-error --max-time 15 \
  https://api.pelajarnumagetan.id/ready)"
if [[ "${public_ready}" != *'"status":"READY"'* ]]; then
  echo "Endpoint API publik tidak mengarah ke backend release ini." >&2
  exit 1
fi
public_frontend="$(curl --fail --silent --show-error --max-time 15 \
  https://pelajarnumagetan.id/login)"
if [[ "${public_frontend}" != *'IPNU IPPNU Magetan ID'* ]]; then
  echo "Domain frontend publik tidak mengarah ke frontend release ini." >&2
  exit 1
fi

echo "DEPLOY_OK release=${RELEASE_ID} build=${BUILD_ID}"
