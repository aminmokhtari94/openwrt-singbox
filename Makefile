SHELL := /usr/bin/env bash

GO_SRC := singbox-manager/src
GOCACHE ?= /tmp/singbox-manager-go-build
DAEMON_OUT ?= /tmp/singbox-managerd

OPENWRT_VERSION ?= 25.12.4
OPENWRT_TARGET_PATH ?= x86/64
OPENWRT_TARGET_DASH ?= x86-64
OPENWRT_PROFILE ?= generic-ext4-combined
OPENWRT_GCC_VERSION ?= 14.3.0
OPENWRT_SDK_HOST ?= Linux-x86_64
OPENWRT_SDK_FEEDS ?= base packages luci
OPENWRT_SDK_FEED_PACKAGES ?= golang luci sing-box
OPENWRT_BASE_URL ?= https://downloads.openwrt.org/releases/$(OPENWRT_VERSION)/targets/$(OPENWRT_TARGET_PATH)

SDK_DIR ?= .sdk
OPENWRT_SDK_NAME := openwrt-sdk-$(OPENWRT_VERSION)-$(OPENWRT_TARGET_DASH)_gcc-$(OPENWRT_GCC_VERSION)_musl.$(OPENWRT_SDK_HOST)
OPENWRT_SDK_ARCHIVE := $(SDK_DIR)/$(OPENWRT_SDK_NAME).tar.zst
DEFAULT_OPENWRT_SDK := $(SDK_DIR)/$(OPENWRT_SDK_NAME)
OPENWRT_SDK ?= $(DEFAULT_OPENWRT_SDK)
DEFAULT_OPENWRT_SDK_ABSPATH := $(abspath $(DEFAULT_OPENWRT_SDK))
OPENWRT_SDK_ABSPATH := $(abspath $(OPENWRT_SDK))
OPENWRT_SDK_PACKAGE_DIR = $(OPENWRT_SDK)/package/openwrt-singbox
OPENWRT_SDK_GOLANG_PACKAGE_MK = $(OPENWRT_SDK)/feeds/packages/lang/golang/golang-package.mk
OPENWRT_SDK_LUCI_MK = $(OPENWRT_SDK)/feeds/luci/luci.mk
OPENWRT_SDK_SING_BOX_MAKEFILE = $(OPENWRT_SDK)/package/feeds/packages/sing-box/Makefile

VM_DIR ?= .vm
VM_BASE_IMAGE := $(VM_DIR)/openwrt-$(OPENWRT_VERSION)-$(OPENWRT_TARGET_DASH)-$(OPENWRT_PROFILE).img
VM_BASE_IMAGE_GZ := $(VM_BASE_IMAGE).gz
VM_DISK ?= $(VM_DIR)/openwrt-test.qcow2
VM_MEM ?= 512
VM_CONTROL_NET ?= 192.168.1.0/24
VM_WAN_NET ?= 10.0.2.0/24
VM_FACTORY_IP ?= 192.168.1.1
VM_GUEST_IP ?= 192.168.200.1
VM_GUEST_NETMASK ?= 255.255.255.0
VM_CONTROL_MAC ?= 52:54:00:12:34:56
VM_WAN_MAC ?= 52:54:00:12:34:57
VM_LAN_MAC ?= 52:54:00:12:34:58
VM_LAN_DEVICE ?= eth2
VM_SSH_PORT ?= 2222
VM_HTTP_PORT ?= 18080
VM_SSH_OPTS ?= -F /dev/null -o StrictHostKeyChecking=no -o UserKnownHostsFile=/tmp/openwrt-singbox-known_hosts -o BatchMode=yes -o ConnectTimeout=2
VM_SSH_WAIT ?= 60
VM_SSH = ssh $(VM_SSH_OPTS) root@$(VM_GUEST_IP)
VM_BOOTSTRAP_SSH = ssh -p $(VM_SSH_PORT) $(VM_SSH_OPTS) root@127.0.0.1
VM_LAN_BRIDGE ?= owrt-lab0
VM_LAN_TAP ?= owrt-lan0
VM_HOST_LAN_IP ?= 192.168.200.254/24

ALPINE_VERSION ?= 3.24.0
ALPINE_ARCH ?= x86_64
ALPINE_BASE_URL ?= https://dl-cdn.alpinelinux.org/alpine/latest-stable/releases/$(ALPINE_ARCH)
ALPINE_ISO ?= $(VM_DIR)/alpine-virt-$(ALPINE_VERSION)-$(ALPINE_ARCH).iso
ALPINE_MEM ?= 512
ALPINE_TAP ?= alpine-lan0
ALPINE_LAN_MAC ?= 52:54:00:12:34:66

IPK_DIR ?= dist
OPENWRT_PACKAGES ?= $(strip $(wildcard $(IPK_DIR)/*.ipk) $(wildcard $(IPK_DIR)/*.apk))
IPKS ?= $(OPENWRT_PACKAGES)

.PHONY: help test build smoke sdk ensure-sdk sdk-check sdk-link ipk build-ipk vm-image vm-net-up vm-net-down vm-run vm-ssh alpine-iso alpine-run proxy-test-help deploy vm-clean

help:
	@printf '%s\n' \
		'targets:' \
		'  make test       Run Go tests' \
		'  make build      Build local daemon to /tmp/singbox-managerd' \
		'  make sdk        Download/extract the local OpenWrt SDK into .sdk' \
		'  make ipk        Build OpenWrt packages into IPK_DIR=dist' \
		'  make smoke      Run local rpcd smoke checks' \
		'  make vm-image   Download OpenWrt image and create qcow2 overlay' \
		'  make vm-run     Boot OpenWrt VM on isolated lab LAN and DHCP WAN' \
		'  make vm-ssh     SSH into the running VM' \
		'  make alpine-run Boot Alpine VM attached to the OpenWrt lab LAN' \
		'  make proxy-test-help Show Alpine connectivity/proxy test commands' \
		'  make deploy     Copy packages to VM and install them' \
		'  make vm-clean   Remove the qcow2 overlay'

test:
	env GOCACHE=$(GOCACHE) go -C $(GO_SRC) test ./...

build:
	env GOCACHE=$(GOCACHE) go -C $(GO_SRC) build -buildvcs=false -o $(DAEMON_OUT) ./cmd/singbox-managerd

smoke: build
	$(DAEMON_OUT) rpcd list
	$(DAEMON_OUT) rpcd call status

$(SDK_DIR):
	mkdir -p $@

$(OPENWRT_SDK_ARCHIVE): | $(SDK_DIR)
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

ipk build-ipk: sdk-link
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
		"$(OPENWRT_SDK)/feeds/base_root/config/Config-build.in"
	perl -0pi -e 's/(^config (?:DEFAULT_|MODULE_DEFAULT_|PACKAGE_)[^\n]*\n(?:(?!^config ).*\n)*?\h*default )[ym]\b/$${1}n/gm' \
		"$(OPENWRT_SDK)/Config-build.in"
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
		'CONFIG_PACKAGE_singbox-manager=m' \
		'CONFIG_PACKAGE_luci-app-singbox-manager=m' \
		>> "$(OPENWRT_SDK)/.config"
	$(MAKE) -C "$(OPENWRT_SDK)" defconfig
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
		'CONFIG_PACKAGE_sing-box=m' \
		'CONFIG_PACKAGE_singbox-manager=m' \
		'CONFIG_PACKAGE_luci-app-singbox-manager=m' \
		>> "$(OPENWRT_SDK)/.config"
	$(MAKE) -C "$(OPENWRT_SDK)" package/singbox-manager/compile package/luci-app-singbox-manager/compile V=s
	mkdir -p "$(IPK_DIR)"
	find "$(OPENWRT_SDK)/bin/packages" -type f \( \
		-name 'singbox-manager_*.ipk' -o \
		-name 'luci-app-singbox-manager_*.ipk' -o \
		-name 'singbox-manager-*.apk' -o \
		-name 'luci-app-singbox-manager-*.apk' \
		\) -exec cp -f {} "$(IPK_DIR)/" \;
	@test -n "$$(find "$(IPK_DIR)" -maxdepth 1 -type f \( -name '*.ipk' -o -name '*.apk' \) -print -quit)" || { echo "No OpenWrt package artifacts found in $(IPK_DIR)"; exit 1; }
	@find "$(IPK_DIR)" -maxdepth 1 -type f \( -name '*.ipk' -o -name '*.apk' \) -print | sort

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
	printf 'Lab LAN ready: bridge %s host %s, OpenWrt tap %s, Alpine tap %s\n' "$(VM_LAN_BRIDGE)" "$(VM_HOST_LAN_IP)" "$(VM_LAN_TAP)" "$(ALPINE_TAP)"

vm-net-down:
	@set -e; \
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
		-netdev user,id=wan,net=$(VM_WAN_NET) \
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
	if [ "$$ready_path" = 'bootstrap' ]; then \
		printf 'Configuring OpenWrt lab LAN on %s (%s)...\n' '$(VM_GUEST_IP)' '$(VM_LAN_DEVICE)'; \
		printf '%s\n' \
			'set network.lan.device=$(VM_LAN_DEVICE)' \
			'set network.lan.ipaddr=$(VM_GUEST_IP)' \
			'set network.lan.netmask=$(VM_GUEST_NETMASK)' \
			'delete network.wan' \
			'set network.wan=interface' \
			'set network.wan.device=eth1' \
			'set network.wan.proto=dhcp' \
			'delete network.wan6' \
			'set network.wan6=interface' \
			'set network.wan6.device=eth1' \
			'set network.wan6.proto=dhcpv6' \
			'set network.wan6.reqaddress=try' \
			'set network.wan6.reqprefix=auto' \
			'commit network' \
			| $(VM_BOOTSTRAP_SSH) 'uci -q batch'; \
		$(VM_BOOTSTRAP_SSH) '/etc/init.d/network reload; ifup lan; ifup wan; ifup wan6 || true' || true; \
		sleep 3; \
		printf 'Waiting for OpenWrt lab SSH on %s...\n' '$(VM_GUEST_IP)'; \
		ready=0; \
		i=0; \
		while [ $$i -lt $(VM_SSH_WAIT) ]; do \
			if ! kill -0 $$qemu_pid 2>/dev/null; then \
				echo 'QEMU exited before lab SSH became ready'; \
				wait $$qemu_pid; \
				exit 1; \
			fi; \
			if $(VM_SSH) true >/dev/null 2>&1; then \
				ready=1; \
				break; \
			fi; \
			i=$$((i + 1)); \
			sleep 1; \
		done; \
		if [ $$ready -ne 1 ]; then \
			printf 'Timed out waiting for OpenWrt lab SSH after %s seconds\n' '$(VM_SSH_WAIT)'; \
			exit 1; \
		fi; \
	fi; \
	$(VM_SSH) 'ubus call network.interface.wan status || true' || true; \
	printf 'OpenWrt VM is running. SSH: make vm-ssh, LuCI: http://%s/. In another terminal: make alpine-run\n' '$(VM_GUEST_IP)'; \
	wait $$qemu_pid; \
	status=$$?; \
	trap - INT TERM EXIT; \
	exit $$status

vm-ssh:
	$(VM_SSH)

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
		'  udhcpc -i eth0 || true' \
		'  ip addr show eth0' \
		'  ip route' \
		'  ping -c 3 $(VM_GUEST_IP)' \
		'  wget -O- http://$(VM_GUEST_IP):1088/proxy.pac' \
		'  http_proxy=http://$(VM_GUEST_IP):2080 wget -O- http://example.com/' \
		'' \
		'For explicit proxy testing, singbox-manager listens on 0.0.0.0:2080 by default.' \
		'For transparent proxy testing, enable TProxy for $(VM_LAN_DEVICE) and run plain wget/curl from Alpine.'

deploy:
	$(if $(strip $(IPKS)),,$(error No OpenWrt packages found in $(IPK_DIR). Set IPKS='/path/*.ipk /path/*.apk' or IPK_DIR=/path))
	scp $(VM_SSH_OPTS) $(IPKS) root@$(VM_GUEST_IP):/tmp/
	$(VM_SSH) 'if command -v apk >/dev/null 2>&1; then apk add --allow-untrusted --force-overwrite --force-reinstall --upgrade /tmp/*.apk; elif command -v opkg >/dev/null 2>&1; then opkg install --force-reinstall /tmp/*.ipk; else echo "No OpenWrt package manager found"; exit 1; fi'
	$(VM_SSH) 'rm -f /tmp/luci-indexcache.*; rm -rf /tmp/luci-modulecache/; [ ! -x /etc/init.d/rpcd ] || /etc/init.d/rpcd restart; [ ! -x /etc/init.d/uhttpd ] || /etc/init.d/uhttpd restart'

vm-clean:
	rm -f $(VM_DISK)
