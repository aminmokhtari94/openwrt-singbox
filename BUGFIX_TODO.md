# singbox-manager bugfix and improvement TODO

## CRUD and Editing

- [x] Add subscription edit API for name, URL, format, update interval, and enabled state.
- [x] Add subscription delete API that removes imported nodes and detaches the subscription from groups.
- [x] Add LuCI edit/delete actions for subscriptions.
- [x] Add LuCI edit/delete actions for manual nodes; keep imported subscription nodes read-only except refresh/delete subscription.
- [x] Add DNS profile CRUD API.
- [x] Add DNS profile LuCI forms.
- [x] Add DNS server CRUD API.
- [x] Add DNS server LuCI forms.
- [x] Add rule-set CRUD API.
- [x] Add rule-set LuCI forms.
- [x] Add route/routing-profile CRUD API and LuCI forms.

## Node Testing and Grouping

- [x] Add per-node URL latency test API.
- [x] Add per-node ICMP/TCP ping test API with OpenWrt-safe fallback behavior.
- [x] Show node health, latency, and last check in LuCI.
- [x] Group nodes by subscription in LuCI and add tabs/filters for subscription, manual, and all nodes.

## Subscription Updates

- [x] Schedule automatic subscription refresh based on each subscription update interval.
- [x] Record subscription refresh failures separately from health latency failures.
- [x] Add refresh-all subscriptions action.

## LuCI UX

- [x] Move add/edit forms into modal dialogs.
- [x] Improve table density, status badges, empty states, and mobile layout.
- [x] Add confirmation dialogs for destructive actions.
- [x] Add consistent action button styling across dashboard, subscriptions, nodes, DNS, rule sets, PAC, TProxy, and TUN.

## PAC

- [x] Add API to save a rendered PAC preview as a named custom PAC.
- [x] Add API to edit existing custom PAC content.
- [x] Add LuCI custom/generated PAC selector.
- [x] Fix typo in UI copy: "seve" -> "save".

## Logs and Metrics

- [x] Add log tab with streaming, refresh, and download actions.
- [x] Add runtime stats API for current connections and traffic counters.
- [x] Add realtime usage/connection graph in LuCI.

## Routing by Source Device

- [x] Parse DHCP leases and connected devices for selectable source IPs.
- [x] Add source-IP routing rules to the UCI model.
- [x] Render source-IP based sing-box route rules.
- [x] Add LuCI controls for connected-device routing.
