#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
USER_TO_AUTHORIZE="${1:-$USER}"

sudo groupadd --force --system mm-web
sudo install -d -m 0755 /etc/polkit-1/rules.d
sudo install -m 0644 "$ROOT/deploy/polkit/50-mm-web-messaging.rules" /etc/polkit-1/rules.d/50-mm-web-messaging.rules
sudo install -d -m 0755 /etc/polkit-1/localauthority/50-local.d
sudo install -m 0644 "$ROOT/deploy/polkit/50-mm-web-messaging.pkla" /etc/polkit-1/localauthority/50-local.d/50-mm-web-messaging.pkla
sudo usermod -aG mm-web "$USER_TO_AUTHORIZE"

echo "PolicyKit rule installed. Start a new login session, then restart mm-web."
echo "For the current shell you may run: sg mm-web -c 'npm run dev'"
echo "If authorization is still denied, restart PolicyKit; this may briefly restart ModemManager:"
echo "  sudo systemctl restart polkit"
