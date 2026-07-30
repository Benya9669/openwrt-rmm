# SPDX-License-Identifier: MIT
include $(TOPDIR)/rules.mk

PKG_NAME:=luci-app-rmm-agent
PKG_VERSION:=0.2.2
PKG_RELEASE:=1

PKG_MAINTAINER:=RMM OpenWrt
PKG_LICENSE:=MIT
PKG_LICENSE_FILES:=LICENSE
PKG_BUILD_DEPENDS:=luci-base/host

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

define Package/luci-i18n-rmm-agent-ru
  SECTION:=luci
  CATEGORY:=LuCI
  TITLE:=luci-app-rmm-agent - Russian translation
  DEPENDS:=+luci-app-rmm-agent
  PKGARCH:=all
endef

define Package/luci-i18n-rmm-agent-ru/description
  Russian translation for luci-app-rmm-agent.
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
	$(INSTALL_DIR) $(1)/usr/share/licenses/luci-app-rmm-agent
	$(INSTALL_DATA) ./LICENSE $(1)/usr/share/licenses/luci-app-rmm-agent/LICENSE
endef

define Package/luci-i18n-rmm-agent-ru/install
	$(INSTALL_DIR) $(1)/usr/lib/lua/luci/i18n
	$(STAGING_DIR_HOSTPKG)/bin/po2lmo ./po/ru/rmm-agent.po \
		$(1)/usr/lib/lua/luci/i18n/rmm-agent.ru.lmo
	$(INSTALL_DIR) $(1)/etc/uci-defaults
	echo "uci set luci.languages.ru='Русский (Russian)'; uci commit luci" \
		> $(1)/etc/uci-defaults/luci-i18n-rmm-agent-ru
endef

$(eval $(call BuildPackage,luci-app-rmm-agent))
$(eval $(call BuildPackage,luci-i18n-rmm-agent-ru))
