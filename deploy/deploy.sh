#!/usr/bin/env bash
set -euo pipefail

: "${LOUVADEIRA_HOST:?defina LOUVADEIRA_HOST (usuario@ip) no mise.local.toml}"
root="$(cd "$(dirname "$0")/.." && pwd)"

ssh "$LOUVADEIRA_HOST" 'mkdir -p ~/louvadeira/bin ~/louvadeira/data ~/.config/systemd/user'
scp -q "$root/bin/louvadeira-linux-arm64" "$LOUVADEIRA_HOST:louvadeira/bin/louvadeira.new"
scp -q "$root/deploy/louvadeira.service" "$LOUVADEIRA_HOST:.config/systemd/user/louvadeira.service"
scp -q "$root/deploy/lv" "$LOUVADEIRA_HOST:louvadeira/lv"

ssh "$LOUVADEIRA_HOST" 'bash -s' <<'REMOTE'
set -euo pipefail
cd ~/louvadeira
mv bin/louvadeira.new bin/louvadeira

if [ ! -f louvadeira.env ]; then
  umask 077
  cat > louvadeira.env <<ENV
LOUVADEIRA_ADDR=:8090
LOUVADEIRA_DATA_DIR=data
LOUVADEIRA_WHATSAPP_MODE=test
LOUVADEIRA_WHATSAPP_ALLOWLIST=
LOUVADEIRA_ADMIN_TOKEN=$(head -c 24 /dev/urandom | base64 | tr -d '/+=')
ENV
  echo "criado ~/louvadeira/louvadeira.env"
fi
if ! grep -q '^LOUVADEIRA_PUBLIC_URL=' louvadeira.env; then
  echo "LOUVADEIRA_PUBLIC_URL=http://$(hostname -I | awk '{print $1}'):8090" >> louvadeira.env
fi

systemctl --user daemon-reload
systemctl --user enable louvadeira >/dev/null 2>&1
systemctl --user restart louvadeira
sleep 2
systemctl --user --no-pager --lines=0 status louvadeira | head -3
curl -fsS http://127.0.0.1:8090/api/health && echo

if [ "$(loginctl show-user "$USER" -p Linger --value)" != "yes" ]; then
  echo "AVISO: sem linger, o serviço para quando a sessão SSH fecha. Rode uma vez: sudo loginctl enable-linger $USER"
fi
REMOTE
