# arp-responder

A small, production-oriented Linux daemon that answers IPv4 ARP requests for addresses which do **not** need to be assigned to the host. It uses Linux `AF_PACKET` sockets directly—no pcap, cgo, routing, NAT, firewall, or forwarding code.

## Design

- one nonblocking `AF_PACKET/SOCK_RAW` socket per interface, filtered by the kernel to ARP EtherType
- one `epoll` event loop with `eventfd` wakeup; no polling or goroutine per packet
- fixed receive/reply buffers reused for the lifetime of the loop
- allocation-free ARP parser and builder
- sorted, merged IPv4 interval target table with logarithmic lookup
- transactional target/config parsing and live interface reconciliation
- fsnotify directory watches (safe with atomic file replacement), plus periodic fallback
- JSON structured logging through Go `slog`
- optional isolated Prometheus registry, including Go/process CPU and memory collectors
- graceful SIGINT/SIGTERM shutdown and hardened systemd service

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Requirements

- Ubuntu 24.04 or another Linux system with `AF_PACKET`
- Go 1.24 or newer to build
- `CAP_NET_RAW` (provided by the service unit)
- Ethernet interfaces with six-byte MAC addresses

The host's IP forwarding, routes, bridge/libvirt topology, reverse-path filtering, and firewall are deliberately outside this daemon's scope. Configure those separately.

## Configuration

Default paths are `/etc/arp-responder/config.yaml` and `/etc/arp-responder/targets.conf`.

```yaml
interface:
  - eno1
  - eno2
reload_interval: 10
log_level: info            # info, warning, or error
reply_mac: auto            # auto uses each receiving interface's MAC; or 02:00:00:00:00:01
logging:
  output: journal          # stdout, journal, or file; journal writes JSON to stdout for systemd
  file: /var/log/arp-responder.log
metrics:
  enabled: false
  listen: 127.0.0.1:9108
```

Targets contain one IPv4 address or CIDR per line. Blank lines and `#` comments are accepted:

```text
# routed customers
192.168.1.5
192.168.2.0/24
10.10.0.0/16
```

Malformed entries and all IPv6 entries reject the complete reload; the previous working configuration stays active. Only Ethernet/IPv4 ARP requests are answered. ARP replies, unknown opcodes, malformed packets, and gratuitous requests (`sender IP == target IP`) are ignored.

Interface, reply MAC, and target changes are applied live. The logging sink, metrics listener, and periodic timer are established at process startup; restart the service after changing those three operational settings. File changes are still noticed immediately through fsnotify.

Validate files without running the event loop:

```bash
sudo /usr/local/bin/arp-responder -check
# or
scripts/check-config.sh ./configs/config.yaml ./configs/targets.conf
```


## Build and test

```bash
make build
make test       # includes race detector
make vet
make bench
```

Run the privileged Linux integration check separately:

```bash
sudo go test -tags integration ./internal/network
```

The parser stress test processes 100,000 requests. `BenchmarkProcess100000` benchmarks batches of 100,000 complete parse/build operations and reports allocations.

## Install

Edit `configs/config.yaml` and `configs/targets.conf`, then:

```bash
sudo ./install.sh
```

The installer builds a stripped binary, creates an unprivileged `arp-responder` system user, preserves existing configuration files (`install -C`), installs the unit, reloads systemd, and enables/starts the service.

Useful commands:

```bash
systemctl status arp-responder
journalctl -u arp-responder -f
systemctl restart arp-responder
```

The unit grants only `CAP_NET_RAW`, bounds address families, enables `NoNewPrivileges`, and applies filesystem/kernel hardening. If file logging is selected, create a writable destination for the service user and adjust `ProtectSystem`/unit write paths as appropriate; journal output is recommended.

## Metrics

When enabled, `/metrics` exposes:

- `arp_responder_requests_total`
- `arp_responder_replies_total`
- `arp_responder_invalid_packets_total`
- `arp_responder_dropped_packets_total`
- `arp_responder_reloads_total`
- standard `process_*` CPU/memory/file-descriptor metrics
- standard `go_*` runtime metrics

The default binds only to loopback. Protect a non-loopback listener with the host firewall or a secured reverse proxy.

## Capacity and tuning

The implementation requests a 4 MiB socket receive buffer; Linux may cap it at `net.core.rmem_max`. For burst-heavy deployments, tune that sysctl and monitor drops. Benchmark on the target NIC/bridge configuration. At very high fan-in, IRQ/RPS placement and bridge netfilter settings generally dominate the responder itself.

The target matcher merges overlapping and adjacent ranges, so large CIDRs consume one interval rather than one entry per address.

## Packet behavior

For a matching request, the reply contains:

- Ethernet destination: requester's source MAC
- Ethernet source: configured reply MAC or receiving interface MAC
- ARP sender MAC: reply MAC
- ARP sender IP: requested target IP
- ARP target MAC/IP: requester's MAC/IP

No addresses are added to an interface and no kernel neighbor, route, NAT, or forwarding state is modified.

## License

Use under the repository's applicable license terms.
