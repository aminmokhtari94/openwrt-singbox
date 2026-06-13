include $(TOPDIR)/rules.mk

PKG_NAME:=singbox-manager
PKG_VERSION:=0.1.0
PKG_RELEASE:=3

PKG_BUILD_DIR:=$(BUILD_DIR)/$(PKG_NAME)-$(PKG_VERSION)
PKG_BUILD_DEPENDS:=golang/host
PKG_BUILD_PARALLEL:=1
PKG_USE_MIPS16:=0

GO_PKG:=github.com/openwrt-singbox/singbox-manager
GO_PKG_BUILD_PKG:=github.com/openwrt-singbox/singbox-manager/cmd/singbox-managerd

include $(INCLUDE_DIR)/package.mk
include $(TOPDIR)/feeds/packages/lang/golang/golang-package.mk

GO_PKG_BUILD_VARS+=GOFLAGS="-buildvcs=false"

define Package/singbox-manager
  SECTION:=net
  CATEGORY:=Network
  TITLE:=Sing-Box Manager daemon for OpenWrt
  DEPENDS:=$(GO_ARCH_DEPENDS) +sing-box +rpcd +ubus +uci +jsonfilter +kmod-nft-tproxy
endef

define Package/singbox-manager/description
 Lightweight OpenWrt-native Sing-Box manager daemon with UCI-backed
 configuration, rpcd/ubus API exposure, and generated sing-box runtime
 configuration.
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

$(eval $(call GoBinPackage,singbox-manager))
$(eval $(call BuildPackage,singbox-manager))
