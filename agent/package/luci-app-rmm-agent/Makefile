include $(TOPDIR)/rules.mk

PKG_NAME:=luci-app-rmm-agent
PKG_VERSION:=0.2.1
PKG_RELEASE:=1

PKG_MAINTAINER:=RMM OpenWrt
PKG_LICENSE:=MIT

include $(INCLUDE_DIR)/package.mk

define Package/luci-app-rmm-agent
  SECTION:=luci
  CATEGORY:=LuCI
  SUBMENU:=3. Applications
  TITLE:=LuCI support for the OpenWrt RMM agent
  DEPENDS:=+luci-base
  PKGARCH:=all
endef

define Package/luci-app-rmm-agent/description
  LuCI application for configuring and controlling the OpenWrt RMM agent.
endef

define Build/Compile
endef

define Package/luci-app-rmm-agent/install
	$(INSTALL_DIR) $(1)/www/luci-static/resources/view/services
	$(INSTALL_DATA) ./htdocs/luci-static/resources/view/services/rmm-agent.js \
		$(1)/www/luci-static/resources/view/services/rmm-agent.js
	$(INSTALL_DIR) $(1)/usr/share/luci/menu.d
	$(INSTALL_DATA) ./root/usr/share/luci/menu.d/luci-app-rmm-agent.json \
		$(1)/usr/share/luci/menu.d/luci-app-rmm-agent.json
	$(INSTALL_DIR) $(1)/usr/share/rpcd/acl.d
	$(INSTALL_DATA) ./root/usr/share/rpcd/acl.d/luci-app-rmm-agent.json \
		$(1)/usr/share/rpcd/acl.d/luci-app-rmm-agent.json
endef

$(eval $(call BuildPackage,luci-app-rmm-agent))
