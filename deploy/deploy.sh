#!/usr/bin/env bash
set -euo pipefail

: "${LOUVADEIRA_HOST:?defina LOUVADEIRA_HOST (usuario@servidor) no mise.local.toml}"
root="$(cd "$(dirname "$0")/.." && pwd)"

platform="$(ssh "$LOUVADEIRA_HOST" 'uname -sm')"
case "$platform" in
  "Linux x86_64") arch=amd64 ;;
  "Linux aarch64" | "Linux arm64") arch=arm64 ;;
  *)
    echo "servidor não suportado: $platform (precisa ser Linux em x86_64 ou arm64)" >&2
    exit 1
    ;;
esac
binary="$root/bin/louvadeira-linux-$arch"
if [ ! -f "$binary" ]; then
  echo "falta $binary; rode 'mise run build:linux'" >&2
  exit 1
fi

ssh "$LOUVADEIRA_HOST" 'mkdir -p ~/louvadeira/bin ~/louvadeira/data ~/.config/systemd/user'
scp -q "$binary" "$LOUVADEIRA_HOST:louvadeira/bin/louvadeira.new"
scp -q "$root/deploy/louvadeira.service" "$LOUVADEIRA_HOST:.config/systemd/user/louvadeira.service"
scp -q "$root/deploy/lv" "$LOUVADEIRA_HOST:louvadeira/lv"

ssh "$LOUVADEIRA_HOST" 'bash -s' <<'REMOTE'
set -euo pipefail
cd ~/louvadeira
mv bin/louvadeira.new bin/louvadeira

if [ ! -f louvadeira.env ]; then
  umask 077
  cat > louvadeira.env <<ENV
LOUVADEIRA_ADDR=127.0.0.1:8090
LOUVADEIRA_DATA_DIR=data
LOUVADEIRA_PUBLIC_URL=
LOUVADEIRA_WHATSAPP_MODE=test
LOUVADEIRA_WHATSAPP_ALLOWLIST=
LOUVADEIRA_ADMIN_TOKEN=$(head -c 24 /dev/urandom | base64 | tr -d '/+=')
ENV
  echo "criado ~/louvadeira/louvadeira.env; defina nele LOUVADEIRA_PUBLIC_URL com o endereço público do app"
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
