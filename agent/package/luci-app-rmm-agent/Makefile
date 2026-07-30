# SPDX-License-Identifier: MIT
include $(TOPDIR)/rules.mk

PKG_VERSION:=0.2.2
PKG_RELEASE:=1

PKG_MAINTAINER:=RMM OpenWrt
PKG_LICENSE:=MIT
PKG_LICENSE_FILES:=LICENSE

LUCI_TITLE:=OpenWrt RMM agent configuration
LUCI_DESCRIPTION:=Configure and control the OpenWrt RMM agent.
LUCI_DEPENDS:=+luci-base
LUCI_PKGARCH:=all

include $(TOPDIR)/feeds/luci/luci.mk
