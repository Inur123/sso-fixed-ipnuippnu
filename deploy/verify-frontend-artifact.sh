#!/usr/bin/env bash
set -euo pipefail

frontend_dir="${1:?Path frontend artifact wajib diberikan}"
expected_backend_url="${2:-https://api.pelajarnumagetan.id}"
expected_documentation_url="${3:-https://doc.pelajarnumagetan.id}"
static_dir="${frontend_dir}/.next/static"
invalid_url_report="$(mktemp /tmp/ipnu-sso-invalid-frontend-urls.XXXXXX)"
trap 'rm -f "${invalid_url_report}"' EXIT

if [[ ! -f "${frontend_dir}/server.js" && ! -f "${frontend_dir}/.next/standalone/server.js" ]]; then
  echo "Artifact frontend tidak memiliki standalone server.js." >&2
  exit 1
fi
if [[ ! -d "${static_dir}" ]]; then
  echo "Artifact frontend tidak memiliki .next/static." >&2
  exit 1
fi
if [[ ! -s "${frontend_dir}/public/images/logo-sso.png" ]]; then
  echo "Logo frontend tidak tersedia atau kosong." >&2
  exit 1
fi

local_url_pattern='https?://(localhost|127\.0\.0\.1|\[::1\]):(8080|3001)'
if grep -RIlE --include='*.js' --include='*.json' --exclude='*.map' \
  "${local_url_pattern}" "${static_dir}" > "${invalid_url_report}"; then
  echo "Artifact frontend masih berisi URL API/dokumentasi development:" >&2
  sed -n '1,10p' "${invalid_url_report}" >&2
  exit 1
fi

for expected_url in "${expected_backend_url}" "${expected_documentation_url}"; do
  if ! grep -RIlF --include='*.js' --include='*.json' --exclude='*.map' \
    "${expected_url}" "${static_dir}" >/dev/null; then
    echo "Artifact frontend tidak memuat URL produksi: ${expected_url}" >&2
    exit 1
  fi
done

echo "Artifact frontend valid: URL publik produksi tertanam dan tidak ada endpoint development."
