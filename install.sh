#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
[[ ${EUID} -eq 0 ]] || { echo "install.sh must run as root" >&2; exit 1; }
command -v go >/dev/null || { echo "Go 1.24+ is required" >&2; exit 1; }
minor=$(go env GOVERSION | sed -E 's/^go1\.([0-9]+).*/\1/')
(( minor >= 24 )) || { echo "Go 1.24+ is required" >&2; exit 1; }
cd "$ROOT"
go build -trimpath -ldflags='-s -w' -o /usr/local/bin/arp-responder ./cmd/arp-responder
getent group arp-responder >/dev/null || groupadd --system arp-responder
id arp-responder >/dev/null 2>&1 || useradd --system --gid arp-responder --home-dir /nonexistent --shell /usr/sbin/nologin arp-responder
install -d -m 0750 -o root -g arp-responder /etc/arp-responder
if [[ ! -e /etc/arp-responder/config.yaml ]]; then
  install -m 0640 -o root -g arp-responder configs/config.yaml /etc/arp-responder/config.yaml
fi
if [[ ! -e /etc/arp-responder/targets.conf ]]; then
  install -m 0640 -o root -g arp-responder configs/targets.conf /etc/arp-responder/targets.conf
fi
install -m 0644 configs/arp-responder.service /etc/systemd/system/arp-responder.service
install -d -m 0755 /usr/share/doc/arp-responder
install -m 0644 README.md /usr/share/doc/arp-responder/README.md
systemctl daemon-reload
systemctl enable --now arp-responder.service
systemctl --no-pager status arp-responder.service || true
