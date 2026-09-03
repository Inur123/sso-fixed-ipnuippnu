#!/usr/bin/env bash

# Dibaca, bukan di-source: nilai env tidak pernah dieksekusi sebagai shell.
# Node.js 22+ juga diperlukan oleh runtime frontend.
read_config_value() {
  node --input-type=module - "${1:?Path env wajib diberikan}" "${2:?Nama env wajib diberikan}" <<'NODE'
import { existsSync, readFileSync } from 'node:fs';
import { parseEnv } from 'node:util';
const [file, key] = process.argv.slice(2);
const value = existsSync(file) ? (parseEnv(readFileSync(file, 'utf8'))[key] ?? '') : '';
if (/[\r\n]/.test(value)) throw new Error(`${key} harus satu baris.`);
process.stdout.write(value.trim());
NODE
}

load_upstream_configuration() {
  local config_file="${1:?Path env deployment wajib diberikan}"
  local key value
  for key in BACKEND_UPSTREAM_URL FRONTEND_UPSTREAM_URL; do
    value="${!key:-}"
    if [[ -z "${value}" ]]; then
      value="$(read_config_value "${config_file}" "${key}")" || return 1
    fi
    if [[ -z "${value}" ]]; then
      echo "${key} wajib diatur di ${config_file} atau environment deployment." >&2
      return 1
    fi
    printf -v "${key}" '%s' "${value}"
    export "${key}"
  done
  node --input-type=module - <<'NODE'
for (const key of ['BACKEND_UPSTREAM_URL', 'FRONTEND_UPSTREAM_URL']) {
  const value = process.env[key];
  const url = new URL(value);
  if (!['http:', 'https:'].includes(url.protocol) ||
      url.username || url.password || url.hash || /[\s;{}$\\]/.test(value)) {
    throw new Error(`${key} bukan URL deployment yang aman.`);
  }
  if (!['127.0.0.1', 'localhost', '[::1]'].includes(url.hostname) || !url.port) {
    throw new Error(`${key} harus menunjuk loopback dengan port eksplisit.`);
  }
  if (url.pathname !== '/' || url.search) {
    throw new Error(`${key} harus origin tanpa path atau query.`);
  }
}
NODE
}

url_component() {
  node -e 'process.stdout.write(new URL(process.argv[1])[process.argv[2]])' "${1:?URL wajib diberikan}" "${2:?Komponen wajib diberikan}"
}
