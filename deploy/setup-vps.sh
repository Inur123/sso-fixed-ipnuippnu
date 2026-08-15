#!/usr/bin/env bash
set -euo pipefail

APP_USER="ipnu-sso"
APP_GROUP="ipnu-sso"
APP_ROOT="/opt/ipnu-sso"
CONFIG_ROOT="/etc/ipnu-sso"
DB_USER="ipnu_sso"
DB_NAME="ipnu_ippnu_id_sso"
DEPLOY_ADMIN_EMAIL="${DEPLOY_ADMIN_EMAIL:?DEPLOY_ADMIN_EMAIL wajib diatur}"
DEPLOY_MAIL_USERNAME="${DEPLOY_MAIL_USERNAME:?DEPLOY_MAIL_USERNAME wajib diatur}"
DEPLOY_MAIL_FROM_ADDRESS="${DEPLOY_MAIL_FROM_ADDRESS:?DEPLOY_MAIL_FROM_ADDRESS wajib diatur}"

if [[ "${EUID}" -ne 0 ]]; then
  echo "Jalankan sebagai root." >&2
  exit 1
fi

if ! id -u "${APP_USER}" >/dev/null 2>&1; then
  useradd --system --home-dir "${APP_ROOT}" --shell /usr/sbin/nologin "${APP_USER}"
fi

install -d -m 0755 "${APP_ROOT}" "${APP_ROOT}/releases"
install -d -o root -g "${APP_GROUP}" -m 0750 "${CONFIG_ROOT}"

# Artifact release bersifat read-only bagi service. Hanya cache image Next.js
# yang boleh ditulis agar optimasi logo/aset tidak gagal oleh ProtectSystem.
if [[ -L "${APP_ROOT}/current" ]]; then
  current_release="$(readlink -f "${APP_ROOT}/current")"
  chown -R root:"${APP_GROUP}" "${current_release}"
  find "${current_release}" -type d -exec chmod 0755 {} +
  find "${current_release}" -type f -exec chmod 0644 {} +
  chmod 0755 "${current_release}/backend/sso-backend"
  install -d -o "${APP_USER}" -g "${APP_GROUP}" -m 0750 \
    "${current_release}/frontend/.next/cache"
fi

if [[ ! -s "${CONFIG_ROOT}/oidc-private.pem" ]]; then
  openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:3072 \
    -out "${CONFIG_ROOT}/oidc-private.pem" >/dev/null 2>&1
fi
chown root:"${APP_GROUP}" "${CONFIG_ROOT}/oidc-private.pem"
chmod 0640 "${CONFIG_ROOT}/oidc-private.pem"

if [[ ! -s "${CONFIG_ROOT}/backend.env" ]]; then
  if [[ ! -s "${CONFIG_ROOT}/mail-password" ]]; then
    echo "${CONFIG_ROOT}/mail-password belum tersedia." >&2
    exit 1
  fi

  db_password="$(openssl rand -hex 32)"
  jwt_secret="$(openssl rand -hex 48)"
  otp_secret="$(openssl rand -hex 48)"
  client_secret_key="$(openssl rand -base64 32 | tr -d '\n')"
  mail_password="$(tr -d '\r\n' < "${CONFIG_ROOT}/mail-password")"

  if [[ "$(sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='${DB_USER}'")" != "1" ]]; then
    sudo -u postgres psql -v ON_ERROR_STOP=1 -c "CREATE ROLE ${DB_USER} LOGIN PASSWORD '${db_password}'" >/dev/null
  else
    sudo -u postgres psql -v ON_ERROR_STOP=1 -c "ALTER ROLE ${DB_USER} WITH LOGIN PASSWORD '${db_password}'" >/dev/null
  fi
  if [[ "$(sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='${DB_NAME}'")" != "1" ]]; then
    sudo -u postgres createdb --owner="${DB_USER}" "${DB_NAME}"
  fi

  umask 027
  {
    printf '%s\n' \
      'APP_ENV=production' \
      'APP_NAME=IPNU IPPNU Magetan ID' \
      'BACKEND_PORT=8180' \
      'BACKEND_PUBLIC_URL=https://api.ipnu.web.id' \
      'FRONTEND_PUBLIC_URL=https://ipnu.web.id' \
      'BACKEND_CORS_ALLOWED_ORIGINS=https://ipnu.web.id' \
      'SESSION_COOKIE_NAME=sso_session' \
      'SESSION_COOKIE_DOMAIN=.ipnu.web.id' \
      'DB_HOST=127.0.0.1' \
      "DB_USER=${DB_USER}" \
      "DB_PASSWORD=${db_password}" \
      "DB_NAME=${DB_NAME}" \
      'DB_PORT=5432' \
      'DB_SSLMODE=disable' \
      'DB_MAX_OPEN_CONNS=25' \
      'DB_MAX_IDLE_CONNS=10' \
      'DB_CONN_MAX_LIFETIME_MINUTES=30' \
      'DB_CONN_MAX_IDLE_TIME_MINUTES=5' \
      "JWT_SECRET=${jwt_secret}" \
      "CLIENT_SECRET_ENCRYPTION_KEY=${client_secret_key}" \
      "OTP_HASH_SECRET=${otp_secret}" \
      "OIDC_PRIVATE_KEY_PATH=${CONFIG_ROOT}/oidc-private.pem" \
      "SUPER_ADMIN_EMAIL=${DEPLOY_ADMIN_EMAIL}" \
      'MAIL_MAILER=smtp' \
      'MAIL_HOST=smtp.gmail.com' \
      'MAIL_PORT=587' \
      "MAIL_USERNAME=${DEPLOY_MAIL_USERNAME}" \
      "MAIL_PASSWORD=${mail_password}" \
      'MAIL_ENCRYPTION=tls' \
      "MAIL_FROM_ADDRESS=${DEPLOY_MAIL_FROM_ADDRESS}" \
      'MAIL_FROM_NAME=SSO IPNU IPPNU Magetan ID' \
      'MAIL_OTP_TTL_MINUTES=10' \
      'PROVISIONING_TARGETS_JSON={}' \
      'PROVISIONING_MAX_ATTEMPTS=12' \
      'PROVISIONING_CONCURRENCY=4' \
      'BACKEND_TRUSTED_PROXIES=127.0.0.1,::1'
  } > "${CONFIG_ROOT}/backend.env"
  chown root:"${APP_GROUP}" "${CONFIG_ROOT}/backend.env"
  chmod 0640 "${CONFIG_ROOT}/backend.env"
  shred -u "${CONFIG_ROOT}/mail-password" 2>/dev/null || rm -f "${CONFIG_ROOT}/mail-password"
fi

cat > "${CONFIG_ROOT}/frontend.env" <<'EOF'
NODE_ENV=production
PORT=3100
HOSTNAME=127.0.0.1
BACKEND_SESSION_COOKIE_NAME=sso_session
NEXT_PUBLIC_BACKEND_URL=https://api.ipnu.web.id
NEXT_PUBLIC_DOCUMENTATION_URL=https://doc.ipnu.web.id
NEXT_PUBLIC_APP_NAME=IPNU IPPNU Magetan ID
NEXT_PUBLIC_APP_TAGLINE=Single Sign-On
NEXT_PUBLIC_APP_DESCRIPTION=Pusat identitas dan Single Sign-On resmi PC IPNU IPPNU Kabupaten Magetan.
NEXT_PUBLIC_ORGANIZATION_NAME=PC IPNU IPPNU Kabupaten Magetan
EOF
chown root:"${APP_GROUP}" "${CONFIG_ROOT}/frontend.env"
chmod 0640 "${CONFIG_ROOT}/frontend.env"

cat > /etc/systemd/system/ipnu-sso-backend.service <<'EOF'
[Unit]
Description=IPNU IPPNU Magetan ID SSO Backend
After=network-online.target postgresql.service
Wants=network-online.target

[Service]
Type=simple
User=ipnu-sso
Group=ipnu-sso
WorkingDirectory=/opt/ipnu-sso/current/backend
EnvironmentFile=/etc/ipnu-sso/backend.env
ExecStart=/opt/ipnu-sso/current/backend/sso-backend
Restart=on-failure
RestartSec=3
TimeoutStopSec=25
KillSignal=SIGTERM
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=/opt/ipnu-sso/current/frontend/.next/cache
ProtectHome=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictSUIDSGID=true
LockPersonality=true
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
MemoryMax=512M
TasksMax=256

[Install]
WantedBy=multi-user.target
EOF

cat > /etc/systemd/system/ipnu-sso-frontend.service <<'EOF'
[Unit]
Description=IPNU IPPNU Magetan ID SSO Frontend
After=network-online.target ipnu-sso-backend.service
Wants=network-online.target

[Service]
Type=simple
User=ipnu-sso
Group=ipnu-sso
WorkingDirectory=/opt/ipnu-sso/current/frontend
EnvironmentFile=/etc/ipnu-sso/frontend.env
ExecStart=/usr/bin/node /opt/ipnu-sso/current/frontend/server.js
Restart=on-failure
RestartSec=3
TimeoutStopSec=20
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictSUIDSGID=true
LockPersonality=true
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
MemoryMax=768M
TasksMax=256

[Install]
WantedBy=multi-user.target
EOF

cat > /etc/nginx/snippets/ipnu-sso-cloudflare-real-ip.conf <<'EOF'
set_real_ip_from 173.245.48.0/20;
set_real_ip_from 103.21.244.0/22;
set_real_ip_from 103.22.200.0/22;
set_real_ip_from 103.31.4.0/22;
set_real_ip_from 141.101.64.0/18;
set_real_ip_from 108.162.192.0/18;
set_real_ip_from 190.93.240.0/20;
set_real_ip_from 188.114.96.0/20;
set_real_ip_from 197.234.240.0/22;
set_real_ip_from 198.41.128.0/17;
set_real_ip_from 162.158.0.0/15;
set_real_ip_from 104.16.0.0/13;
set_real_ip_from 104.24.0.0/14;
set_real_ip_from 172.64.0.0/13;
set_real_ip_from 131.0.72.0/22;
real_ip_header CF-Connecting-IP;
real_ip_recursive on;
EOF

cat > /etc/nginx/sites-available/ipnu-sso.conf <<'EOF'
server {
    listen 80;
    listen [::]:80;
    server_name ipnu.web.id;
    include /etc/nginx/snippets/ipnu-sso-cloudflare-real-ip.conf;

    location / {
        proxy_pass http://127.0.0.1:3100;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 60s;
    }
}

server {
    listen 80;
    listen [::]:80;
    server_name api.ipnu.web.id;
    include /etc/nginx/snippets/ipnu-sso-cloudflare-real-ip.conf;
    client_max_body_size 1m;

    location / {
        proxy_pass http://127.0.0.1:8180;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_connect_timeout 5s;
        proxy_read_timeout 65s;
    }
}

server {
    listen 80;
    listen [::]:80;
    server_name doc.ipnu.web.id;
    include /etc/nginx/snippets/ipnu-sso-cloudflare-real-ip.conf;
    root /opt/ipnu-sso/current/docs;
    index index.html;

    location /assets/ {
        try_files $uri =404;
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
    location / {
        try_files $uri $uri.html $uri/ =404;
    }
}
EOF

ln -sfn /etc/nginx/sites-available/ipnu-sso.conf /etc/nginx/sites-enabled/ipnu-sso.conf
systemctl daemon-reload
systemctl enable --now ipnu-sso-backend.service ipnu-sso-frontend.service
nginx -t
systemctl reload nginx

echo "SETUP_OK"
