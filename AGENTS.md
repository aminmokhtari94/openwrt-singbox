# Repository Guidelines

## Project Structure & Module Organization

This repository contains two OpenWrt packages:

- `singbox-manager/`: backend package for the Go daemon `singbox-managerd`.
- `singbox-manager/src/`: Go module source and tests.
- `singbox-manager/files/`: OpenWrt-installed files such as UCI defaults, init scripts, and runtime directories.
- `luci-app-singbox-manager/`: LuCI frontend package.
- `luci-app-singbox-manager/htdocs/luci-static/resources/view/singbox-manager/`: LuCI JavaScript views.
- `luci-app-singbox-manager/root/usr/share/`: LuCI menu and rpcd ACL metadata.
- `TODO.md`: milestone roadmap and implementation checklist.

Generated sing-box configuration must come from UCI through the daemon. Do not hand-edit runtime JSON outputs.

## Build, Test, and Development Commands

Use `rtk` before shell commands in this workspace.

- `rtk env GOCACHE=/tmp/singbox-manager-go-build go test ./...` from `singbox-manager/src`: run Go tests.
- `rtk env GOCACHE=/tmp/singbox-manager-go-build go build -buildvcs=false -o /tmp/singbox-managerd ./cmd/singbox-managerd` from `singbox-manager/src`: build the daemon locally.
- `rtk /tmp/singbox-managerd rpcd list`: smoke-test rpcd method discovery.
- `rtk /tmp/singbox-managerd rpcd call status`: smoke-test JSON status output.

Use OpenWrt `apk` commands for package installation, removal, and inspection; do not use `opkg` unless explicitly targeting an older OpenWrt image that still uses it.

OpenWrt package builds should be run from an OpenWrt SDK/buildroot using the package directories in this repo.

## Coding Style & Naming Conventions

Go code must be formatted with `gofmt`. Keep backend packages under `internal/` as implementation grows, with command entrypoints under `cmd/`. Use clear lower-case package names and explicit JSON field names matching the ubus API.

LuCI code must use modern JavaScript views only, no legacy Lua pages. Keep UI names concise and OpenWrt-native.

## Testing Guidelines

Use Go’s standard `testing` package. Test files should be named `*_test.go`, and test functions should use `TestNameBehavior` style, for example `TestLoadConfigReadsManagerAndActiveGroup`.

Add focused unit tests for UCI parsing, model validation, subscription parsing, rendering, and firewall generation. Renderer tests should use golden JSON fixtures once the renderer exists.

## Commit & Pull Request Guidelines

No readable git history is available in this workspace, so use Conventional Commits going forward, such as `feat: add subscription parser` or `test: cover UCI validation`.

Pull requests should include a short summary, changed package areas, test results, and screenshots for LuCI UI changes. Link related issues or TODO milestones when applicable.

## Security & Configuration Tips

Keep secrets, subscription URLs, and generated runtime configs out of commits. Treat UCI as the source of truth. Runtime files belong under `/var/run/`, generated persistent files under `/etc/singbox-manager/`, and rule sets under `/etc/singbox-manager/rulesets/`.
