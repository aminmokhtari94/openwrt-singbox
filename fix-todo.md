# Fix TODO

## 1. Remove PAC Completely

- Remove PAC config sections from defaults, parser, validator, editor helpers, models, RPC methods, ACL, and LuCI menu/views.
- Remove PAC server startup and `/proxy.pac` handling from `singbox-managerd`.
- Remove PAC package/internal renderer code and tests.
- Remove PAC references from `Makefile proxy-test-help`, docs/help text, status payloads, and generated UI labels.
- Add migration cleanup for existing `config pac` and `config pac_custom` UCI sections.
- Acceptance: no `pac`, `PAC`, `proxy.pac`, or `pac_custom` feature surface remains except migration notes/tests.

## 2. Improve DNS UX

- Merge the confusing DNS controls into two clear concepts:
  - DNS capture: whether LAN client DNS packets are intercepted.
  - DNS upstream policy: where intercepted/resolved DNS requests are sent.
- Remove duplicate-looking toggles between TProxy DNS hijack and DNS profile hijack, or make one the authoritative control.
- Show active DNS flow in one summary: client source -> capture mode -> resolver profile -> upstream detour.
- Add validation warnings when DNS capture is enabled but no usable DNS server/profile exists.
- Add status/debug output for rendered sing-box DNS servers, DNS rules, and active DNS inbound.
- Acceptance: a user can configure DNS behavior without opening both TProxy and DNS tabs.

## 3. Improve Policy-Based Router UX

- Make router policy the primary workflow, not PAC/proxy-client setup.
- Introduce policy groups for source devices:
  - group name
  - source IP/CIDR/MAC/device binding
  - routing profile
  - DNS profile
  - fallback behavior
- Keep destination policy separate but connected:
  - rule sets for geoip/geosite/domain/IP destinations
  - final outbound for unmatched traffic
  - per-rule-set outbound action
- Show a policy matrix: source group x destination policy -> direct/proxy/block/dns-only.
- Add device picker integration from discovered DHCP/ARP devices.
- Acceptance: common router use cases can be configured from one Policy page.

## 4. Add UI Tooltips And Help

- Add concise tooltips for advanced terms:
  - TProxy
  - DNS capture/hijack
  - DNS detour
  - rule set
  - source rule
  - final outbound
  - kill switch
  - TUN auto route / auto redirect
- Add inline help near destructive/risky options, especially kill switch and DNS capture.
- Add examples in UI help text:
  - "Proxy this phone only"
  - "Direct local country sites, proxy everything else"
  - "DNS-over-proxy only for selected devices"
  - "Block internet for a device group"
- Acceptance: every non-obvious control has a tooltip or help affordance.

## 5. DNS Resolve-Over-Proxy-Only Routing Rule

- Add a first-class routing action for DNS-only proxying:
  - DNS queries from selected sources are captured.
  - DNS upstream traffic goes through proxy.
  - Non-DNS traffic from those sources remains direct unless another policy matches.
- Add upstream forwarder policy list:
  - named upstream forwarder policies
  - resolver list
  - detour: direct/proxy/specific outbound
  - match by source group and optional destination domain/ruleset
- Extend validation so DNS-only policies require DNS capture and at least one enabled upstream.
- Render sing-box route rules in correct order:
  - source + protocol DNS -> hijack-dns
  - source non-DNS fallback -> direct or configured fallback
  - upstream DNS server detour -> proxy/specified outbound
- Add tests for rule ordering and detour rendering.
- Acceptance: selected clients can use proxied DNS resolution without proxying their normal traffic.

## Suggested End State

- Remove PAC entirely.
- Replace separate "Rulesets", "DNS", and "TProxy DNS Hijack" mental models with:
  - Devices / Source Groups
  - Destination Policies / Rule Sets
  - DNS Capture and Upstream Policies
  - Runtime Mode / Transparent Router
- Keep expert controls available, but make the default path a router policy builder.
