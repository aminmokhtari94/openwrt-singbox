SHELL := /usr/bin/env bash

# ---------------------------------------------------------------------------
# Go daemon (host build / tests)
# ---------------------------------------------------------------------------
GO_SRC := singbox-manager/src
GOCACHE ?= /tmp/singbox-manager-go-build
DAEMON_OUT ?= /tmp/singbox-managerd

# ---------------------------------------------------------------------------
# OpenWrt release coordinates
#
# OPENWRT_TARGET_PATH selects the target/subtarget (e.g. x86/64). Everything
# else is derived from it; override OPENWRT_TARGET_PATH alone to build for a
# different device, or use the `ipk-<arch>` presets further down.
# ---------------------------------------------------------------------------
OPENWRT_VERSION ?= 25.12.4
# Release series (major.minor) derived from OPENWRT_VERSION, e.g. 24.10.7 -> 24.10.
# Selects the toolchain version below and names the per-series package feed.
OPENWRT_SERIES := $(word 1,$(subst ., ,$(OPENWRT_VERSION))).$(word 2,$(subst ., ,$(OPENWRT_VERSION)))
OPENWRT_TARGET_PATH ?= x86/64
OPENWRT_TARGET_DASH ?= $(subst /,-,$(OPENWRT_TARGET_PATH))
OPENWRT_PROFILE ?= generic-ext4-combined

# GCC toolchain version per OpenWrt release series — embedded in the SDK archive
# filename, so it must match the targeted release. Add a line for a new series,
# or override OPENWRT_GCC_VERSION directly for an unlisted one.
OPENWRT_GCC_VERSION_24.10 := 13.3.0
OPENWRT_GCC_VERSION_25.12 := 14.3.0
OPENWRT_GCC_VERSION ?= $(OPENWRT_GCC_VERSION_$(OPENWRT_SERIES))

OPENWRT_SDK_HOST ?= Linux-x86_64
OPENWRT_SDK_FEEDS ?= base packages luci
OPENWRT_SDK_FEED_PACKAGES ?= golang luci sing-box
OPENWRT_BASE_URL ?= https://downloads.openwrt.org/releases/$(OPENWRT_VERSION)/targets/$(OPENWRT_TARGET_PATH)

# ---------------------------------------------------------------------------
# Multi-architecture package presets
#
# Each ARCH label maps to an OpenWrt target path via TARGET_PATH_<arch>. The
# GCC/musl toolchain version is uniform across targets within a release, so the
# only per-arch difference is the target path. Add a new label by defining
# TARGET_PATH_<name> and appending it to ARCHS (or pass ARCHS= on the CLI).
# ---------------------------------------------------------------------------
ARCHS ?= x86_64 aarch64 armv7 mipsel
TARGET_PATH_x86_64  := x86/64
TARGET_PATH_aarch64 := armsr/armv8
TARGET_PATH_armv7   := armsr/armv7
TARGET_PATH_mipsel  := ramips/mt7621

# SDK archive names embed the libc variant. Most targets use plain "musl";
# 32-bit ARM EABI (armv7) ships as "musl_eabi". Override LIBC_<arch> for any
# target whose suffix differs from the musl default.
LIBC_armv7 := musl_eabi

# ---------------------------------------------------------------------------
# Local OpenWrt SDK (downloaded into .sdk)
# ---------------------------------------------------------------------------
SDK_DIR ?= .sdk
OPENWRT_LIBC ?= musl
OPENWRT_SDK_NAME := openwrt-sdk-$(OPENWRT_VERSION)-$(OPENWRT_TARGET_DASH)_gcc-$(OPENWRT_GCC_VERSION)_$(OPENWRT_LIBC).$(OPENWRT_SDK_HOST)
OPENWRT_SDK_ARCHIVE := $(SDK_DIR)/$(OPENWRT_SDK_NAME).tar.zst
DEFAULT_OPENWRT_SDK := $(SDK_DIR)/$(OPENWRT_SDK_NAME)
OPENWRT_SDK ?= $(DEFAULT_OPENWRT_SDK)
DEFAULT_OPENWRT_SDK_ABSPATH := $(abspath $(DEFAULT_OPENWRT_SDK))
OPENWRT_SDK_ABSPATH := $(abspath $(OPENWRT_SDK))
OPENWRT_SDK_PACKAGE_DIR = $(OPENWRT_SDK)/package/openwrt-singbox
OPENWRT_SDK_GOLANG_PACKAGE_MK = $(OPENWRT_SDK)/feeds/packages/lang/golang/golang-package.mk
OPENWRT_SDK_LUCI_MK = $(OPENWRT_SDK)/feeds/luci/luci.mk
OPENWRT_SDK_SING_BOX_MAKEFILE = $(OPENWRT_SDK)/package/feeds/packages/sing-box/Makefile
# The base feed's build config lives under feeds/base_root (25.12+) or feeds/base
# (24.10). Resolve whichever exists — expanded at recipe time, after feeds are
# installed, so the directory is present by the time it is read.
OPENWRT_SDK_BASE_CONFIG = $(firstword $(wildcard \
	$(OPENWRT_SDK)/feeds/base_root/config/Config-build.in \
	$(OPENWRT_SDK)/feeds/base/config/Config-build.in))

# ---------------------------------------------------------------------------
# OpenWrt test VM (x86 only) and isolated lab network
# ---------------------------------------------------------------------------
VM_DIR ?= .vm
VM_BASE_IMAGE := $(VM_DIR)/openwrt-$(OPENWRT_VERSION)-$(OPENWRT_TARGET_DASH)-$(OPENWRT_PROFILE).img
VM_BASE_IMAGE_GZ := $(VM_BASE_IMAGE).gz
VM_DISK ?= $(VM_DIR)/openwrt-test.qcow2
VM_MEM ?= 512
VM_CONTROL_NET ?= 192.168.1.0/24
VM_FACTORY_IP ?= 192.168.1.1
VM_GUEST_IP ?= 192.168.200.1
VM_GUEST_NETMASK ?= 255.255.255.0
VM_WAN_IP ?= 10.0.200.2
VM_WAN_NET ?= 10.0.200.0/24
VM_WAN_NETMASK ?= 255.255.255.0
VM_WAN_GATEWAY ?= 10.0.200.1
VM_CONTROL_MAC ?= 52:54:00:12:34:56
VM_WAN_MAC ?= 52:54:00:12:34:57
VM_LAN_MAC ?= 52:54:00:12:34:58
VM_LAN_DEVICE ?= eth2
VM_WAN_DEVICE ?= eth1
VM_SSH_PORT ?= 2222
VM_HTTP_PORT ?= 18080
VM_SSH_OPTS ?= -F /dev/null -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o GlobalKnownHostsFile=/dev/null -o LogLevel=ERROR -o BatchMode=yes -o ConnectTimeout=2
VM_SSH_WAIT ?= 60
VM_SSH = ssh $(VM_SSH_OPTS) root@$(VM_GUEST_IP)
VM_BOOTSTRAP_SSH = ssh -p $(VM_SSH_PORT) $(VM_SSH_OPTS) root@127.0.0.1
VM_LAN_BRIDGE ?= owrt-lab0
VM_LAN_TAP ?= owrt-lan0
VM_HOST_LAN_IP ?= 192.168.200.254/24
VM_WAN_TAP ?= owrt-wan0
VM_HOST_WAN_IP ?= $(VM_WAN_GATEWAY)/24

# ---------------------------------------------------------------------------
# Alpine guest used as a LAN client for proxy testing
# ---------------------------------------------------------------------------
ALPINE_VERSION ?= 3.24.0
ALPINE_ARCH ?= x86_64
ALPINE_BASE_URL ?= https://dl-cdn.alpinelinux.org/alpine/latest-stable/releases/$(ALPINE_ARCH)
ALPINE_ISO ?= $(VM_DIR)/alpine-virt-$(ALPINE_VERSION)-$(ALPINE_ARCH).iso
ALPINE_MEM ?= 512
ALPINE_TAP ?= alpine-lan0
ALPINE_LAN_MAC ?= 52:54:00:12:34:66
ALPINE_GUEST_IP ?= 192.168.200.2/24
ALPINE_GATEWAY ?= $(VM_GUEST_IP)

# ---------------------------------------------------------------------------
# Package artifacts
# ---------------------------------------------------------------------------
IPK_DIR ?= dist
OPENWRT_PACKAGES ?= $(strip $(wildcard $(IPK_DIR)/*.ipk) $(wildcard $(IPK_DIR)/*.apk))
IPKS ?= $(OPENWRT_PACKAGES)

# Note: the per-arch ipk-<arch> targets are intentionally NOT phony — declaring
# them phony excludes them from the ipk-% pattern rule's implicit-rule search.
.PHONY: help test build smoke sdk ensure-sdk sdk-check sdk-link \
	ipk build-ipk ipk-all apk-index \
	vm-image vm-net-up vm-net-down vm-run vm-ssh \
	alpine-iso alpine-run proxy-test-help deploy undeploy vm-clean

help:
	@printf '%s\n' \
		'targets:' \
		'  make test       Run Go tests' \
		'  make build      Build local daemon to /tmp/singbox-managerd' \
		'  make sdk        Download/extract the local OpenWrt SDK into .sdk' \
		'  make ipk        Build packages for OPENWRT_TARGET_PATH=$(OPENWRT_TARGET_PATH) into IPK_DIR=$(IPK_DIR)' \
		'  make ipk-<arch> Build packages for one arch ($(ARCHS)) into dist/<arch>' \
		'  make ipk-all    Build packages for every arch in ARCHS' \
		'  make apk-index-<arch> Build signed packages.adb feed index (APK_SIGN_KEY=<ec-key>)' \
		'  make smoke      Run local rpcd smoke checks' \
		'  make vm-image   Download OpenWrt image and create qcow2 overlay' \
		'  make vm-run     Boot OpenWrt VM on isolated lab LAN and host-NAT WAN' \
		'  make vm-ssh     SSH into the running VM' \
		'  make alpine-run Boot Alpine VM attached to the OpenWrt lab LAN' \
		'  make proxy-test-help Show Alpine connectivity/proxy test commands' \
		'  make deploy     Copy packages to VM and install them' \
		'  make undeploy   Stop, remove packages, and wipe leftover state on the VM' \
		'  make vm-clean   Remove the qcow2 overlay'

# ---------------------------------------------------------------------------
# Host build / test
# ---------------------------------------------------------------------------
test:
	env GOCACHE=$(GOCACHE) go -C $(GO_SRC) test ./...

build:
	env GOCACHE=$(GOCACHE) go -C $(GO_SRC) build -buildvcs=false -o $(DAEMON_OUT) ./cmd/singbox-managerd

smoke: build
	$(DAEMON_OUT) rpcd list
	$(DAEMON_OUT) rpcd call status

# ---------------------------------------------------------------------------
# OpenWrt SDK download / preparation
# ---------------------------------------------------------------------------
$(SDK_DIR):
	mkdir -p $@

$(OPENWRT_SDK_ARCHIVE): | $(SDK_DIR)
	@test -n "$(OPENWRT_GCC_VERSION)" || { echo "No GCC version known for OpenWrt series '$(OPENWRT_SERIES)' (from OPENWRT_VERSION=$(OPENWRT_VERSION)). Set OPENWRT_GCC_VERSION=<x.y.z> or add OPENWRT_GCC_VERSION_$(OPENWRT_SERIES) in the Makefile."; exit 1; }
	wget -O $@ $(OPENWRT_BASE_URL)/$(OPENWRT_SDK_NAME).tar.zst

ifeq ($(OPENWRT_SDK_ABSPATH),$(DEFAULT_OPENWRT_SDK_ABSPATH))
$(OPENWRT_SDK)/include/toplevel.mk: $(OPENWRT_SDK_ARCHIVE)
	rm -rf "$(OPENWRT_SDK)"
	zstd -dc $< | tar -xf - -C "$(SDK_DIR)"
	touch $@
endif

$(OPENWRT_SDK)/.feeds-installed: $(OPENWRT_SDK)/include/toplevel.mk Makefile
	cd "$(OPENWRT_SDK)" && ./scripts/feeds update $(OPENWRT_SDK_FEEDS)
	cd "$(OPENWRT_SDK)" && ./scripts/feeds install $(OPENWRT_SDK_FEED_PACKAGES)
	touch $@

sdk-check:
	@test -d "$(OPENWRT_SDK)" || { echo "OPENWRT_SDK does not exist: $(OPENWRT_SDK)"; exit 1; }
	@test -f "$(OPENWRT_SDK)/include/toplevel.mk" || { echo "OPENWRT_SDK is not an OpenWrt SDK/buildroot: $(OPENWRT_SDK)"; exit 1; }

sdk ensure-sdk: $(OPENWRT_SDK)/.feeds-installed
	@test -d "$(OPENWRT_SDK)" || { echo "OPENWRT_SDK does not exist: $(OPENWRT_SDK)"; exit 1; }
	@test -f "$(OPENWRT_SDK)/include/toplevel.mk" || { echo "OPENWRT_SDK is not an OpenWrt SDK/buildroot: $(OPENWRT_SDK)"; exit 1; }
	@test -f "$(OPENWRT_SDK_GOLANG_PACKAGE_MK)" || { echo "Missing OpenWrt packages feed helper: $(OPENWRT_SDK_GOLANG_PACKAGE_MK)"; exit 1; }
	@test -f "$(OPENWRT_SDK_LUCI_MK)" || { echo "Missing OpenWrt LuCI feed helper: $(OPENWRT_SDK_LUCI_MK)"; exit 1; }
	@test -f "$(OPENWRT_SDK_SING_BOX_MAKEFILE)" || { echo "Missing OpenWrt sing-box package: $(OPENWRT_SDK_SING_BOX_MAKEFILE)"; exit 1; }

sdk-link: ensure-sdk
	mkdir -p "$(OPENWRT_SDK_PACKAGE_DIR)"
	ln -sfnT "$(CURDIR)/singbox-manager" "$(OPENWRT_SDK_PACKAGE_DIR)/singbox-manager"
	ln -sfnT "$(CURDIR)/luci-app-singbox-manager" "$(OPENWRT_SDK_PACKAGE_DIR)/luci-app-singbox-manager"

# ---------------------------------------------------------------------------
# Package build
#
# sdk_config_reset strips every bulk/profile selection from the SDK .config and
# re-seeds the "not set" header shared by both config passes. Each pass then
# appends just the packages it wants before running defconfig / compile.
# ---------------------------------------------------------------------------
define sdk_config_reset
	touch "$(OPENWRT_SDK)/.config"
	sed -i \
		-e '/^CONFIG_ALL=/d' \
		-e '/^CONFIG_ALL_KMODS=/d' \
		-e '/^CONFIG_ALL_NONSHARED=/d' \
		-e '/^CONFIG_BUILDBOT=/d' \
		-e '/^CONFIG_TARGET_MULTI_PROFILE=/d' \
		-e '/^CONFIG_TARGET_ALL_PROFILES=/d' \
		-e '/^CONFIG_TARGET_PER_DEVICE_ROOTFS=/d' \
		-e '/^CONFIG_DEFAULT_/d' \
		-e '/^CONFIG_MODULE_DEFAULT_/d' \
		-e '/^CONFIG_PACKAGE_/d' \
		-e '/^# CONFIG_PACKAGE_/d' \
		"$(OPENWRT_SDK)/.config"
	printf '%s\n' \
		'# CONFIG_ALL is not set' \
		'# CONFIG_ALL_KMODS is not set' \
		'# CONFIG_ALL_NONSHARED is not set' \
		'# CONFIG_BUILDBOT is not set' \
		'# CONFIG_TARGET_MULTI_PROFILE is not set' \
		'# CONFIG_TARGET_ALL_PROFILES is not set' \
		'# CONFIG_TARGET_PER_DEVICE_ROOTFS is not set' \
		'# CONFIG_PACKAGE_sing-box-tiny is not set' \
		>> "$(OPENWRT_SDK)/.config"
endef

ipk build-ipk: sdk-link
	@test -n "$(OPENWRT_SDK_BASE_CONFIG)" || { echo "No base-feed Config-build.in under $(OPENWRT_SDK)/feeds (looked for base_root/ and base/)"; exit 1; }
	sed -i \
		-e '/^config TARGET_MULTI_PROFILE$$/,/^config TARGET_ALL_PROFILES$$/s/^\([[:space:]]*\)default y/\1default n/' \
		-e '/^config TARGET_ALL_PROFILES$$/,/^config TARGET_PER_DEVICE_ROOTFS$$/s/^\([[:space:]]*\)default y/\1default n/' \
		-e '/^config TARGET_PER_DEVICE_ROOTFS$$/,/^config TARGET_DEVICE_/s/^\([[:space:]]*\)default y/\1default n/' \
		-e '/^config DEFAULT_/,/^config /s/^\([[:space:]]*\)default y/\1default n/' \
		-e '/^config MODULE_DEFAULT_/,/^config /s/^\([[:space:]]*\)default y/\1default n/' \
		-e '/^config PACKAGE_/,/^config /s/^\([[:space:]]*\)default m/\1default n/' \
		-e '/^config ALL_NONSHARED$$/,/^config ALL_KMODS$$/s/^\([[:space:]]*\)default y/\1default n/' \
		-e '/^config ALL_KMODS$$/,/^config ALL$$/s/^\([[:space:]]*\)default y/\1default n/' \
		-e '/^config BUILDBOT$$/,/^config SIGNATURE_CHECK$$/s/^\([[:space:]]*\)default y/\1default n/' \
		"$(OPENWRT_SDK)/Config-build.in" \
		"$(OPENWRT_SDK_BASE_CONFIG)"
	perl -0pi -e 's/(^config (?:DEFAULT_|MODULE_DEFAULT_|PACKAGE_)[^\n]*\n(?:(?!^config ).*\n)*?\h*default )[ym]\b/$${1}n/gm' \
		"$(OPENWRT_SDK)/Config-build.in"
	$(sdk_config_reset)
	printf '%s\n' \
		'CONFIG_PACKAGE_singbox-manager=m' \
		'CONFIG_PACKAGE_luci-app-singbox-manager=m' \
		>> "$(OPENWRT_SDK)/.config"
	$(MAKE) -C "$(OPENWRT_SDK)" defconfig
	$(sdk_config_reset)
	printf '%s\n' \
		'CONFIG_PACKAGE_sing-box=m' \
		'CONFIG_PACKAGE_singbox-manager=m' \
		'CONFIG_PACKAGE_luci-app-singbox-manager=m' \
		>> "$(OPENWRT_SDK)/.config"
	# IGNORE_ERRORS=m lets the build skip the unbuildable kmod-nft-tproxy
	# dependency (the SDK has kernel headers only and cannot build kernel
	# modules) without failing the whole run. singbox-manager still packages and
	# keeps +kmod-nft-tproxy in its Depends, so the device pulls the stock kmod
	# from the official feed at install time. The artifact guard below ensures we
	# never publish an empty release if our own package genuinely fails to build.
	$(MAKE) -C "$(OPENWRT_SDK)" package/singbox-manager/compile package/luci-app-singbox-manager/compile IGNORE_ERRORS=m V=s
	mkdir -p "$(IPK_DIR)"
	find "$(OPENWRT_SDK)/bin/packages" -type f \( \
		-name 'singbox-manager_*.ipk' -o \
		-name 'luci-app-singbox-manager_*.ipk' -o \
		-name 'singbox-manager-*.apk' -o \
		-name 'luci-app-singbox-manager-*.apk' \
		\) -exec cp -f {} "$(IPK_DIR)/" \;
	@test -n "$$(find "$(IPK_DIR)" -maxdepth 1 -type f \( -name '*.ipk' -o -name '*.apk' \) -print -quit)" || { echo "No OpenWrt package artifacts found in $(IPK_DIR)"; exit 1; }
	@find "$(IPK_DIR)" -maxdepth 1 -type f \( -name '*.ipk' -o -name '*.apk' \) -print | sort

# Build for every arch in ARCHS, each into its own dist/<arch> directory.
ipk-all:
	@set -e; for arch in $(ARCHS); do \
		printf '==> building %s\n' "$$arch"; \
		$(MAKE) ipk-$$arch; \
	done

# Build for a single arch preset, e.g. `make ipk-aarch64`.
ipk-%:
	@test -n "$(TARGET_PATH_$*)" || { echo "Unknown arch '$*'. Known: $(ARCHS) (or set TARGET_PATH_$*=<target/subtarget>)"; exit 1; }
	$(MAKE) ipk OPENWRT_TARGET_PATH="$(TARGET_PATH_$*)" OPENWRT_LIBC="$(or $(LIBC_$*),musl)" IPK_DIR="$(IPK_DIR)/$*"

# ---------------------------------------------------------------------------
# apk feed index (packages.adb)
#
# OpenWrt 25.12+ apk clients (apk-tools 3) read a binary `packages.adb` index
# built with `apk mkndx` — NOT Alpine's legacy `apk index`/APKINDEX.tar.gz. Use
# the SDK's own host apk so the index format matches the packages exactly.
#
# Signing: when APK_SIGN_KEY points at an EC private key, the index is signed.
# apk embeds the key file's *basename* as the signer name, so clients must
# install the matching public key at /etc/apk/keys/<basename>. With no key the
# index is unsigned and clients need `apk --allow-untrusted`.
#
# `apk-index-<arch>` resolves the same per-arch SDK as `ipk-<arch>`, so run it
# after the matching `ipk-<arch>` build. It no-ops for opkg/.ipk releases
# (24.10), which have no .apk to index. It also writes `<IPK_DIR>/apk-arch` with
# the real OpenWrt package arch (e.g. mipsel_24kc) — the value a device reports
# as `cat /etc/apk/arch` — so the published feed dir can be named to match and
# clients can auto-select it.
# ---------------------------------------------------------------------------
APK_HOST_BIN = $(OPENWRT_SDK)/staging_dir/host/bin/apk

apk-index:
	@idx_dir="$(IPK_DIR)"; \
	if [ -z "$$(find "$$idx_dir" -maxdepth 1 -name '*.apk' -print -quit 2>/dev/null)" ]; then \
		echo "No .apk in $$idx_dir (opkg/.ipk release?); skipping apk index"; \
		exit 0; \
	fi; \
	apk_bin="$(abspath $(APK_HOST_BIN))"; \
	[ -x "$$apk_bin" ] || apk_bin="$$(find "$(abspath $(OPENWRT_SDK))" -path '*/staging_dir/host/bin/apk' -type f 2>/dev/null | head -1)"; \
	[ -n "$$apk_bin" ] && [ -x "$$apk_bin" ] || { echo "SDK host apk (apk-tools 3) not found under $(OPENWRT_SDK); build packages first"; exit 1; }; \
	sign_args=""; \
	if [ -n "$(APK_SIGN_KEY)" ]; then \
		test -f "$(APK_SIGN_KEY)" || { echo "APK_SIGN_KEY is not a file: $(APK_SIGN_KEY)"; exit 1; }; \
		sign_args="--sign-key $(abspath $(APK_SIGN_KEY))"; \
		echo "Signing $$idx_dir/packages.adb with key $(notdir $(APK_SIGN_KEY))"; \
	else \
		echo "APK_SIGN_KEY empty; building UNSIGNED $$idx_dir/packages.adb"; \
	fi; \
	( cd "$$idx_dir" && "$$apk_bin" mkndx --allow-untrusted $$sign_args --output packages.adb *.apk ); \
	echo "Wrote $$idx_dir/packages.adb"; \
	sample="$$(find "$(abspath $(OPENWRT_SDK))/bin/packages" -name 'singbox-manager-*.apk' -print -quit 2>/dev/null)"; \
	apk_arch="$$([ -n "$$sample" ] && basename "$$(dirname "$$(dirname "$$sample")")")"; \
	[ -n "$$apk_arch" ] || { echo "Could not determine apk arch from $(OPENWRT_SDK)/bin/packages"; exit 1; }; \
	printf '%s\n' "$$apk_arch" > "$$idx_dir/apk-arch"; \
	echo "Feed arch: $$apk_arch (-> $$idx_dir/apk-arch; matches the device's \`cat /etc/apk/arch\`)"

# Build the apk index for a single arch preset, e.g. `make apk-index-aarch64`.
apk-index-%:
	@test -n "$(TARGET_PATH_$*)" || { echo "Unknown arch '$*'. Known: $(ARCHS) (or set TARGET_PATH_$*=<target/subtarget>)"; exit 1; }
	$(MAKE) apk-index OPENWRT_TARGET_PATH="$(TARGET_PATH_$*)" OPENWRT_LIBC="$(or $(LIBC_$*),musl)" IPK_DIR="$(IPK_DIR)/$*"

# ---------------------------------------------------------------------------
# OpenWrt test VM
# ---------------------------------------------------------------------------
$(VM_DIR):
	mkdir -p $@

$(VM_BASE_IMAGE_GZ): | $(VM_DIR)
	wget -O $@ $(OPENWRT_BASE_URL)/openwrt-$(OPENWRT_VERSION)-$(OPENWRT_TARGET_DASH)-$(OPENWRT_PROFILE).img.gz

$(VM_BASE_IMAGE): $(VM_BASE_IMAGE_GZ)
	gzip -dkf $<

$(VM_DISK): $(VM_BASE_IMAGE)
	qemu-img create -f qcow2 -F raw -b $(abspath $(VM_BASE_IMAGE)) $@

vm-image: $(VM_DISK)

vm-net-up:
	@set -e; \
	user=$$(id -un); \
	add_filter_rule() { \
		chain="$$1"; \
		shift; \
		sudo iptables -C "$$chain" "$$@" 2>/dev/null || sudo iptables -I "$$chain" 1 "$$@"; \
	}; \
	add_optional_filter_rule() { \
		chain="$$1"; \
		shift; \
		if ! sudo iptables -C "$$chain" "$$@" 2>/dev/null; then \
			sudo iptables -I "$$chain" 1 "$$@" 2>/dev/null || true; \
		fi; \
	}; \
	add_nat_rule() { \
		chain="$$1"; \
		shift; \
		sudo iptables -t nat -C "$$chain" "$$@" 2>/dev/null || sudo iptables -t nat -I "$$chain" 1 "$$@"; \
	}; \
	sudo sysctl -w net.ipv4.ip_forward=1 >/dev/null; \
	if ! sudo ip link show "$(VM_LAN_BRIDGE)" >/dev/null 2>&1; then \
		sudo ip link add name "$(VM_LAN_BRIDGE)" type bridge; \
	fi; \
	sudo ip addr flush dev "$(VM_LAN_BRIDGE)"; \
	sudo ip addr replace "$(VM_HOST_LAN_IP)" dev "$(VM_LAN_BRIDGE)"; \
	sudo ip link set "$(VM_LAN_BRIDGE)" up; \
	for tap in "$(VM_LAN_TAP)" "$(ALPINE_TAP)"; do \
		if ! sudo ip link show "$$tap" >/dev/null 2>&1; then \
			sudo ip tuntap add dev "$$tap" mode tap user "$$user"; \
		fi; \
		sudo ip link set "$$tap" master "$(VM_LAN_BRIDGE)"; \
		sudo ip link set "$$tap" up; \
	done; \
	if ! sudo ip link show "$(VM_WAN_TAP)" >/dev/null 2>&1; then \
		sudo ip tuntap add dev "$(VM_WAN_TAP)" mode tap user "$$user"; \
	fi; \
	sudo ip addr flush dev "$(VM_WAN_TAP)"; \
	sudo ip addr replace "$(VM_HOST_WAN_IP)" dev "$(VM_WAN_TAP)"; \
	sudo ip link set "$(VM_WAN_TAP)" up; \
	add_filter_rule FORWARD -i "$(VM_WAN_TAP)" -j ACCEPT; \
	add_filter_rule FORWARD -o "$(VM_WAN_TAP)" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT; \
	add_filter_rule FORWARD -i "$(VM_LAN_BRIDGE)" -o "$(VM_LAN_BRIDGE)" -j ACCEPT; \
	add_optional_filter_rule FORWARD -i "$(ALPINE_TAP)" -o "$(VM_LAN_TAP)" -j ACCEPT; \
	add_optional_filter_rule FORWARD -i "$(VM_LAN_TAP)" -o "$(ALPINE_TAP)" -j ACCEPT; \
	add_optional_filter_rule FORWARD -m physdev --physdev-is-bridged --physdev-in "$(ALPINE_TAP)" --physdev-out "$(VM_LAN_TAP)" -j ACCEPT; \
	add_optional_filter_rule FORWARD -m physdev --physdev-is-bridged --physdev-in "$(VM_LAN_TAP)" --physdev-out "$(ALPINE_TAP)" -j ACCEPT; \
	add_nat_rule POSTROUTING -s "$(VM_WAN_NET)" -j MASQUERADE; \
	printf 'Lab LAN ready: bridge %s host %s, OpenWrt tap %s, Alpine tap %s\n' "$(VM_LAN_BRIDGE)" "$(VM_HOST_LAN_IP)" "$(VM_LAN_TAP)" "$(ALPINE_TAP)"; \
	printf 'Lab WAN ready: tap %s host %s, OpenWrt WAN %s via %s\n' "$(VM_WAN_TAP)" "$(VM_HOST_WAN_IP)" "$(VM_WAN_IP)" "$(VM_WAN_GATEWAY)"

vm-net-down:
	@set -e; \
	delete_filter_rule() { \
		chain="$$1"; \
		shift; \
		while sudo iptables -C "$$chain" "$$@" 2>/dev/null; do \
			sudo iptables -D "$$chain" "$$@"; \
		done; \
	}; \
	delete_nat_rule() { \
		chain="$$1"; \
		shift; \
		while sudo iptables -t nat -C "$$chain" "$$@" 2>/dev/null; do \
			sudo iptables -t nat -D "$$chain" "$$@"; \
		done; \
	}; \
	delete_nat_rule POSTROUTING -s "$(VM_WAN_NET)" -j MASQUERADE; \
	delete_filter_rule FORWARD -i "$(VM_WAN_TAP)" -j ACCEPT; \
	delete_filter_rule FORWARD -o "$(VM_WAN_TAP)" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT; \
	delete_filter_rule FORWARD -i "$(VM_LAN_BRIDGE)" -o "$(VM_LAN_BRIDGE)" -j ACCEPT; \
	delete_filter_rule FORWARD -i "$(ALPINE_TAP)" -o "$(VM_LAN_TAP)" -j ACCEPT; \
	delete_filter_rule FORWARD -i "$(VM_LAN_TAP)" -o "$(ALPINE_TAP)" -j ACCEPT; \
	delete_filter_rule FORWARD -m physdev --physdev-is-bridged --physdev-in "$(ALPINE_TAP)" --physdev-out "$(VM_LAN_TAP)" -j ACCEPT; \
	delete_filter_rule FORWARD -m physdev --physdev-is-bridged --physdev-in "$(VM_LAN_TAP)" --physdev-out "$(ALPINE_TAP)" -j ACCEPT; \
	if sudo ip link show "$(VM_WAN_TAP)" >/dev/null 2>&1; then \
		sudo ip link set "$(VM_WAN_TAP)" down 2>/dev/null || true; \
		sudo ip link delete "$(VM_WAN_TAP)" 2>/dev/null || true; \
	fi; \
	for tap in "$(VM_LAN_TAP)" "$(ALPINE_TAP)"; do \
		if sudo ip link show "$$tap" >/dev/null 2>&1; then \
			sudo ip link set "$$tap" down 2>/dev/null || true; \
			sudo ip link delete "$$tap" 2>/dev/null || true; \
		fi; \
	done; \
	if sudo ip link show "$(VM_LAN_BRIDGE)" >/dev/null 2>&1; then \
		sudo ip link set "$(VM_LAN_BRIDGE)" down 2>/dev/null || true; \
		sudo ip link delete "$(VM_LAN_BRIDGE)" 2>/dev/null || true; \
	fi

vm-run: $(VM_DISK) vm-net-up
	@set -e; \
	qemu-system-x86_64 \
		-enable-kvm \
		-m $(VM_MEM) \
		-drive file=$(VM_DISK),format=qcow2,if=virtio \
		-netdev user,id=control,net=$(VM_CONTROL_NET),hostfwd=tcp:127.0.0.1:$(VM_SSH_PORT)-$(VM_FACTORY_IP):22,hostfwd=tcp:127.0.0.1:$(VM_HTTP_PORT)-$(VM_FACTORY_IP):80 \
		-netdev tap,id=wan,ifname=$(VM_WAN_TAP),script=no,downscript=no \
		-netdev tap,id=lan,ifname=$(VM_LAN_TAP),script=no,downscript=no \
		-device virtio-net-pci,netdev=control,mac=$(VM_CONTROL_MAC) \
		-device virtio-net-pci,netdev=wan,mac=$(VM_WAN_MAC) \
		-device virtio-net-pci,netdev=lan,mac=$(VM_LAN_MAC) \
		-nographic </dev/null & \
	qemu_pid=$$!; \
	trap 'kill $$qemu_pid 2>/dev/null || true; wait $$qemu_pid 2>/dev/null || true' INT TERM EXIT; \
	printf 'Waiting for OpenWrt SSH on %s or 127.0.0.1:%s...\n' '$(VM_GUEST_IP)' '$(VM_SSH_PORT)'; \
	ready=0; \
	ready_path=''; \
	i=0; \
	while [ $$i -lt $(VM_SSH_WAIT) ]; do \
		if ! kill -0 $$qemu_pid 2>/dev/null; then \
			echo 'QEMU exited before SSH became ready'; \
			wait $$qemu_pid; \
			exit 1; \
		fi; \
		if $(VM_SSH) true >/dev/null 2>&1; then \
			ready=1; \
			ready_path='lab'; \
			break; \
		fi; \
		if $(VM_BOOTSTRAP_SSH) true >/dev/null 2>&1; then \
			ready=1; \
			ready_path='bootstrap'; \
			break; \
		fi; \
		i=$$((i + 1)); \
		sleep 1; \
	done; \
	if [ $$ready -ne 1 ]; then \
		printf 'Timed out waiting for OpenWrt SSH after %s seconds\n' '$(VM_SSH_WAIT)'; \
		exit 1; \
	fi; \
	if [ "$$ready_path" = 'lab' ]; then \
		printf 'OpenWrt VM is running. SSH: make vm-ssh (%s), LuCI: http://%s/. In another terminal: make alpine-run\n' '$(VM_GUEST_IP)' '$(VM_GUEST_IP)'; \
	else \
		printf 'OpenWrt VM is running. SSH: ssh -p %s root@127.0.0.1, LuCI: http://127.0.0.1:%s/ (factory network config).\n' '$(VM_SSH_PORT)' '$(VM_HTTP_PORT)'; \
	fi; \
	wait $$qemu_pid; \
	status=$$?; \
	trap - INT TERM EXIT; \
	exit $$status

vm-ssh:
	$(VM_SSH)

# ---------------------------------------------------------------------------
# Alpine LAN client
# ---------------------------------------------------------------------------
$(ALPINE_ISO).sha256: | $(VM_DIR)
	wget -O $@ $(ALPINE_BASE_URL)/$(notdir $@)

$(ALPINE_ISO): $(ALPINE_ISO).sha256
	wget -O $@ $(ALPINE_BASE_URL)/$(notdir $@)
	cd "$(VM_DIR)" && sha256sum -c "$(notdir $@).sha256"

alpine-iso: $(ALPINE_ISO)

alpine-run: $(ALPINE_ISO) vm-net-up
	qemu-system-x86_64 \
		-enable-kvm \
		-m $(ALPINE_MEM) \
		-cdrom $(ALPINE_ISO) \
		-boot d \
		-netdev tap,id=lan,ifname=$(ALPINE_TAP),script=no,downscript=no \
		-device virtio-net-pci,netdev=lan,mac=$(ALPINE_LAN_MAC) \
		-nographic

proxy-test-help:
	@printf '%s\n' \
		'Inside Alpine:' \
		'  ip link set eth0 up' \
		'  udhcpc -i eth0 -q || ip addr add $(ALPINE_GUEST_IP) dev eth0' \
		'  ip route replace default via $(ALPINE_GATEWAY)' \
		'  ip addr show eth0' \
		'  ip route' \
		'  ping -c 3 $(VM_GUEST_IP)' \
		'  ping -c 3 1.1.1.1' \
		'  printf "nameserver 1.1.1.1\n" > /etc/resolv.conf' \
		'  wget -O- http://example.com/' \
		'  http_proxy=http://$(VM_GUEST_IP):2080 wget -O- http://example.com/' \
		'  nslookup example.com $(VM_GUEST_IP)' \
		'' \
		'For explicit proxy testing, singbox-manager listens on 0.0.0.0:2080 by default.' \
		'For transparent proxy testing, enable TProxy for $(VM_LAN_DEVICE) and run plain wget/curl from Alpine.'

# ---------------------------------------------------------------------------
# Deploy / clean
# ---------------------------------------------------------------------------
deploy:
	$(if $(strip $(IPKS)),,$(error No OpenWrt packages found in $(IPK_DIR). Set IPKS='/path/*.ipk /path/*.apk' or IPK_DIR=/path))
	scp $(VM_SSH_OPTS) $(IPKS) root@$(VM_GUEST_IP):/tmp/
	$(VM_SSH) 'if command -v apk >/dev/null 2>&1; then apk add --allow-untrusted --force-overwrite --force-reinstall --upgrade /tmp/*.apk; elif command -v opkg >/dev/null 2>&1; then opkg install --force-reinstall /tmp/*.ipk; else echo "No OpenWrt package manager found"; exit 1; fi'
	$(VM_SSH) 'rm -f /tmp/luci-indexcache.*; rm -rf /tmp/luci-modulecache/; [ ! -x /etc/init.d/rpcd ] || /etc/init.d/rpcd restart; [ ! -x /etc/init.d/uhttpd ] || /etc/init.d/uhttpd restart'

# undeploy stops the daemon (its stop runs `singbox-managerd cleanup`, tearing
# down the nftables include and fwmark policy routing), removes both packages,
# and wipes leftover state — the UCI config conffile, generated/runtime files,
# the firewall fragment, and any stale tproxy routing — so the next deploy lands
# on a clean slate. Safe to run when nothing is installed.
undeploy:
	$(VM_SSH) '/etc/init.d/singbox-managerd stop 2>/dev/null; if command -v apk >/dev/null 2>&1; then apk del luci-app-singbox-manager singbox-manager 2>/dev/null; elif command -v opkg >/dev/null 2>&1; then opkg remove luci-app-singbox-manager singbox-manager 2>/dev/null; fi; rm -rf /etc/config/singbox-manager /etc/singbox-manager; rm -f /etc/nftables.d/90-singbox-manager.nft; [ ! -x /etc/init.d/firewall ] || /etc/init.d/firewall reload; for f in -4 -6; do ip $$f rule del fwmark 0x1 lookup 100 2>/dev/null; ip $$f route flush table 100 2>/dev/null; done; true'
	$(VM_SSH) 'rm -f /tmp/luci-indexcache.*; rm -rf /tmp/luci-modulecache/; [ ! -x /etc/init.d/rpcd ] || /etc/init.d/rpcd restart; [ ! -x /etc/init.d/uhttpd ] || /etc/init.d/uhttpd restart'

vm-clean:
	rm -f $(VM_DISK)
