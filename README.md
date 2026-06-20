# openwrt-singbox

[![CI](https://github.com/aminmokhtari94/openwrt-singbox/actions/workflows/ci.yml/badge.svg)](https://github.com/aminmokhtari94/openwrt-singbox/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/aminmokhtari94/openwrt-singbox?sort=semver)](https://github.com/aminmokhtari94/openwrt-singbox/releases/latest)

An OpenWrt-native manager for [sing-box](https://sing-box.sagernet.org/): a small Go
daemon plus a LuCI web app that turn `sing-box` into a first-class OpenWrt service —
configured through UCI, exposed over `rpcd`/`ubus`, and driven from a five-page LuCI UI.

It manages subscriptions and node groups, DNS, routing rules and rule-sets, and a
transparent (TProxy) gateway with per-device overrides — without hand-editing
`sing-box` JSON.

## Features

- **Subscriptions & node groups** — import nodes from subscription URLs, organize them
  into groups, auto-select by `urltest`, and run ping / latency / health checks.
- **DNS** — manage DNS servers (UDP / DoH / …) with per-server detours and DNS rules.
- **Routing** — route rules backed by remote rule-sets (GeoIP / Geosite, e.g. Iran/CN),
  with a configurable final outbound.
- **Transparent proxy** — nftables TProxy gateway for the LAN, optional DNS hijack,
  kill switch, and per-device (`proxy_device`) mode overrides (including TCP-only with
  direct UDP for consoles).
- **TUN mode** — optional `tun` inbound with `auto_route` / `auto_redirect`.
- **OpenWrt-native** — UCI config, a procd service, an `rpcd`/`ubus` API, and a LuCI app.

## Components

| Path | What it is |
| --- | --- |
| `singbox-manager/` | The `singbox-managerd` Go daemon + OpenWrt package (Makefile, UCI config, init script). |
| `luci-app-singbox-manager/` | LuCI web app (Dashboard, Nodes, DNS, Routing, Network). |
| `Makefile` | Top-level dev/build orchestration: Go tests, SDK download, package builds, QEMU test lab. |
| `flake.nix` | Nix dev shell with the full toolchain (Go, QEMU, OpenWrt SDK prerequisites). |

### How it works

`singbox-managerd` runs three roles from one binary:

- `serve` — the long-running procd service. It reconciles the UCI config into a generated
  `sing-box` config, supervises the `sing-box` process, and programs the nftables TProxy
  include and fwmark policy routing.
- `rpcd` — the `rpcd`/`ubus` plugin (symlinked as `/usr/libexec/rpcd/singbox.manager`) that
  backs the LuCI UI and `ubus call singbox.manager …`.
- `cleanup` — tears down the nftables include and policy routing (run on service stop).

## Install

Download the packages from the
[Releases](https://github.com/aminmokhtari94/openwrt-singbox/releases/latest) page. The
release build targets OpenWrt 25.12, which uses the `apk` package manager, and ships:

- `singbox-manager-<version>_<arch>.apk` — the daemon, one per architecture
  (`x86_64`, `aarch64`, `armv7`, `mipsel`)
- `luci-app-singbox-manager-<version>.apk` — the web UI (architecture-independent)

```sh
# pick the daemon matching your device's architecture
apk add --allow-untrusted \
  ./singbox-manager-*_x86_64.apk \
  ./luci-app-singbox-manager-*.apk
```

> On OpenWrt releases older than 24.10 (which use `opkg`/`.ipk`), build for your target
> with `make ipk-<arch>` and install the resulting `.ipk` with `opkg`.

Then open **LuCI → Services → SingBox Manager**. Configuration lives in
`/etc/config/singbox-manager`.

> `sing-box` is pulled in automatically as a dependency. For **transparent (TProxy)
> mode**, also install the `kmod-nft-tproxy` kernel module — it is not a package
> dependency because the OpenWrt SDK can't build kernel modules, so it must be installed
> from the stock feed:
>
> ```sh
> opkg install kmod-nft-tproxy   # or: apk add kmod-nft-tproxy
> ```

## Build from source

Packages are built with the OpenWrt SDK. The top-level `Makefile` downloads and prepares
the SDK automatically.

```sh
make ipk-x86_64      # build for one arch into dist/x86_64/
make ipk-aarch64     # armsr/armv8
make ipk-armv7       # armsr/armv7
make ipk-mipsel      # ramips/mt7621
make ipk-all         # build every arch in ARCHS into dist/<arch>/
```

Override the target without a preset:

```sh
make ipk OPENWRT_TARGET_PATH=ath79/generic OPENWRT_VERSION=25.12.4
```

## Develop

```sh
make test     # run the Go test suite
make build    # build the daemon locally to /tmp/singbox-managerd
make smoke    # local rpcd smoke checks (list + status)
make help     # list all targets
```

A reproducible toolchain is available via Nix (`nix develop`) or direnv (`direnv allow`).

### Test in a VM

The `Makefile` can stand up an isolated OpenWrt QEMU lab (x86) with an Alpine LAN client
for end-to-end proxy testing:

```sh
make vm-image          # download the OpenWrt image + create a qcow2 overlay
make vm-run            # boot OpenWrt on an isolated lab LAN + host-NAT WAN
make deploy            # build (dist/) then install packages onto the VM
make alpine-run        # boot an Alpine LAN client (separate terminal)
make proxy-test-help   # print connectivity / proxy test commands
make undeploy          # remove packages and wipe leftover state
```

## CI / Releases

- **CI** (`.github/workflows/ci.yml`) — runs `gofmt`, `go vet`, and the Go test suite on
  every push and pull request.
- **Release** (`.github/workflows/release.yml`) — on a `v*` tag, builds the OpenWrt
  packages for all architectures via the SDK and publishes them as a GitHub Release.

Cut a release:

```sh
git tag v0.1.0
git push origin v0.1.0
```
