include $(TOPDIR)/rules.mk

PKG_NAME:=luci-app-singbox-manager
# Stamped from the release tag via SINGBOX_PKG_VERSION; see singbox-manager/Makefile.
PKG_VERSION:=$(if $(SINGBOX_PKG_VERSION),$(SINGBOX_PKG_VERSION),0.1.0)
PKG_RELEASE:=1

LUCI_TITLE:=LuCI support for SingBox Manager
LUCI_DEPENDS:=+singbox-manager
LUCI_PKGARCH:=all

include $(TOPDIR)/feeds/luci/luci.mk

# call BuildPackage - OpenWrt buildroot signature
