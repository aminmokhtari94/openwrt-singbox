# singbox-manager TODO

## Milestone 1 - OpenWrt Package Skeleton

- [x] Create `singbox-manager` backend package directory.
- [x] Create `luci-app-singbox-manager` frontend package directory.
- [x] Add backend OpenWrt package Makefile.
- [x] Add LuCI package Makefile.
- [x] Add default UCI configuration schema with examples.
- [x] Add procd init script for `singbox-managerd`.
- [x] Add rpcd ACL for LuCI access.
- [x] Add rpcd exec bridge entrypoint through `singbox-managerd rpcd`.
- [x] Add minimal Go daemon with Unix-socket JSON API.
- [x] Add initial ubus/rpcd `status` method.
- [x] Add initial LuCI JS dashboard view.

## Milestone 2 - Runtime and Renderer

- [x] Implement strict UCI parser and validator.
- [x] Implement internal typed model for manager, groups, subscriptions, nodes, routing, DNS, rulesets, TProxy, and TUN.
- [x] Implement sing-box config renderer.
- [x] Generate files under `/etc/singbox-manager/generated/`.
- [x] Generate runtime config at `/var/run/sing-box/config.json`.
- [x] Validate generated config with `sing-box check`.
- [x] Implement start, stop, restart, reload runtime methods.
- [x] Add golden-file renderer tests.

## Milestone 3 - Subscriptions and Nodes

- [x] Implement HTTP/HTTPS subscription fetcher.
- [x] Implement base64 and plain subscription detection.
- [x] Implement parsers for `vmess`, `vless`, `trojan`, and `shadowsocks`.
- [x] Add parsers for `hysteria2` and `tuic`.
- [x] Normalize imported nodes into internal model.
- [x] Persist imported nodes with source subscription metadata.
- [x] Add manual node CRUD API.
- [x] Add LuCI subscription and node pages.

## Milestone 4 - Strategies and Health

- [x] Implement manual strategy.
- [x] Implement selector outbound rendering.
- [x] Implement urltest outbound rendering.
- [x] Implement load-balance outbound rendering.
- [x] Implement URL latency tests.
- [x] Implement subscription and group health checks.
- [x] Add scheduled health checks.

## Milestone 5 - DNS

- [x] Implement DNS profiles: direct, proxy, split.
- [x] Support DoH, DoT, DoQ, and UDP servers.
- [x] Add provider templates: Cloudflare, Google, Quad9, AdGuard.
- [x] Implement DNS test API.
- [x] Add LuCI DNS page.

## Milestone 6 - Rule Sets and Geo Routing

- [x] Add built-in Iran Direct template.
- [x] Add built-in China Direct template.
- [x] Add built-in Russia Direct template.
- [x] Implement remote SRS download and refresh.
- [x] Store rulesets under `/etc/singbox-manager/rulesets/`.
- [x] Add scheduled ruleset updates.
- [x] Add LuCI rule-set page.

## Milestone 7 - PAC

- [x] Implement PAC renderer.
- [x] Serve `/proxy.pac`.
- [x] Support custom rules.
- [x] Support whitelist and blacklist.
- [x] Support local network bypass.
- [x] Add LuCI PAC page.

## Milestone 8 - TProxy and Firewall

- [x] Implement fw4-compatible nftables generator.
- [x] Generate include file under `/etc/nftables.d/`.
- [x] Support LAN transparent proxy.
- [x] Support selective devices, subnets, and MAC addresses.
- [x] Implement DNS hijacking.
- [x] Add cleanup on disable/stop.
- [x] Add LuCI transparent proxy page.

## Milestone 9 - TUN

- [x] Implement TUN inbound rendering.
- [x] Support auto route.
- [x] Support auto redirect.
- [x] Support IPv4 and IPv6 addresses.
- [x] Enforce TProxy/TUN mutual exclusion.
- [x] Add LuCI TUN controls.

## Milestone 10 - Production Hardening

- [ ] Add full backend unit tests.
- [ ] Add OpenWrt SDK build tests for 23.05 and 24.x.
- [ ] Add runtime smoke tests in OpenWrt VM.
- [ ] Add LuCI mobile and dark-mode checks.
- [ ] Add log streaming and download.
- [ ] Add package upgrade migration handling.
- [ ] Add release documentation.
