include $(TOPDIR)/rules.mk

PKG_NAME:=singbox-manager
PKG_VERSION:=0.1.0
PKG_RELEASE:=5

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
  # kmod-nft-tproxy is required at runtime for transparent (TProxy) mode. Like
  # sing-box's own +kmod-tun/+kmod-inet-diag deps, it is only *selected* (never
  # *compiled*) by our SDK build, so declaring it here is safe and ensures
  # opkg/apk pulls the stock kmod from the kernel feed at install time. Do NOT
  # add it to a kmod /compile target — the SDK can't build kernel modules.
  DEPENDS:=$(GO_ARCH_DEPENDS) +sing-box +rpcd +ubus +uci +jsonfilter +kmod-nft-tproxy
endef

define Package/singbox-manager/description
 Lightweight OpenWrt-native Sing-Box manager daemon with UCI-backed
 configuration, rpcd/ubus API exposure, and generated sing-box runtime
 configuration.

 Transparent (TProxy) mode requires the kmod-nft-tproxy kernel module, which is
 a declared dependency and installed automatically from the OpenWrt feed.
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
