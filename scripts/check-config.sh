#!/usr/bin/env bash
set -euo pipefail
exec "${ARP_RESPONDER_BIN:-/usr/local/bin/arp-responder}" -check \
  -config "${1:-/etc/arp-responder/config.yaml}" \
  -targets "${2:-/etc/arp-responder/targets.conf}"
