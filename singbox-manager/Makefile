include $(TOPDIR)/rules.mk

PKG_NAME:=singbox-manager
PKG_VERSION:=0.1.0
PKG_RELEASE:=7

PKG_BUILD_DIR:=$(BUILD_DIR)/$(PKG_NAME)-$(PKG_VERSION)
PKG_BUILD_DEPENDS:=golang/host
PKG_BUILD_PARALLEL:=1
# cgo is incompatible with MIPS16 (Go's gcc_mipsx.S fails to assemble), so
# disable it for MIPS targets like ramips/mt7621. PKG_BUILD_FLAGS:=no-mips16 is
# the mechanism the upstream sing-box package uses; the older PKG_USE_MIPS16:=0
# is not honored on current OpenWrt.
PKG_BUILD_FLAGS:=no-mips16

GO_PKG:=github.com/openwrt-singbox/singbox-manager
GO_PKG_BUILD_PKG:=github.com/openwrt-singbox/singbox-manager/cmd/singbox-managerd

include $(INCLUDE_DIR)/package.mk
include $(TOPDIR)/feeds/packages/lang/golang/golang-package.mk

GO_PKG_BUILD_VARS+=GOFLAGS="-buildvcs=false"

define Package/singbox-manager
  SECTION:=net
  CATEGORY:=Network
  TITLE:=Sing-Box Manager daemon for OpenWrt
  # kmod-nft-tproxy is required at runtime for transparent (TProxy) mode but is
  # intentionally NOT declared here. Unlike sing-box's leaf kmod deps
  # (kmod-tun/kmod-inet-diag), kmod-nft-tproxy pulls sibling netfilter modules
  # whose .ko's the OpenWrt SDK cannot build, which breaks every SDK/CI package
  # build. Install it on the router when enabling transparent proxying:
  #   apk add kmod-nft-tproxy   (or: opkg install kmod-nft-tproxy)
  # The daemon reports a clear, actionable error if the module is missing.
  DEPENDS:=$(GO_ARCH_DEPENDS) +sing-box +rpcd +ubus +uci +jsonfilter
endef

define Package/singbox-manager/description
 Lightweight OpenWrt-native Sing-Box manager daemon with UCI-backed
 configuration, rpcd/ubus API exposure, and generated sing-box runtime
 configuration.

 Transparent (TProxy) mode additionally requires the kmod-nft-tproxy kernel
 module; install it with `apk add kmod-nft-tproxy` (or `opkg install
 kmod-nft-tproxy`) when enabling that feature.
endef

define Build/Prepare
	$(call Build/Prepare/Default)
	$(CP) ./src/. $(PKG_BUILD_DIR)/
endef

define Package/singbox-manager/conffiles
/etc/config/singbox-manager
endef

define Package/singbox-manager/install
	$(call GoPackage/Package/Install/Bin,$(1))
	$(INSTALL_DIR) $(1)/etc/config
	$(INSTALL_CONF) ./files/etc/config/singbox-manager $(1)/etc/config/singbox-manager
	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/etc/init.d/singbox-managerd $(1)/etc/init.d/singbox-managerd
	$(INSTALL_DIR) $(1)/etc/uci-defaults
	$(INSTALL_BIN) ./files/etc/uci-defaults/90-singbox-manager $(1)/etc/uci-defaults/90-singbox-manager
	$(INSTALL_DIR) $(1)/etc/singbox-manager/generated
	$(INSTALL_DIR) $(1)/etc/singbox-manager/rulesets
	$(INSTALL_DIR) $(1)/usr/libexec/rpcd
	$(LN) /usr/bin/singbox-managerd $(1)/usr/libexec/rpcd/singbox.manager
endef

# Restart the long-running daemon on (re)install so it re-execs the new binary.
# Without this, an upgrade leaves the old `serve` process running a deleted
# inode and write RPCs added in the new build return "unknown method".
define Package/singbox-manager/postinst
#!/bin/sh
[ -n "$${IPKG_INSTROOT}" ] || {
	/etc/init.d/singbox-managerd enable
	/etc/init.d/singbox-managerd restart
}
exit 0
endef

$(eval $(call GoBinPackage,singbox-manager))
$(eval $(call BuildPackage,singbox-manager))
