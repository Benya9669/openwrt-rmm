include $(TOPDIR)/rules.mk

PKG_NAME:=rmm-agent
PKG_VERSION:=0.1.0
PKG_RELEASE:=2

PKG_MAINTAINER:=RMM OpenWrt
PKG_LICENSE:=MIT

include $(INCLUDE_DIR)/package.mk

define Package/rmm-agent
  SECTION:=admin
  CATEGORY:=Administration
  TITLE:=OpenWrt RMM agent
  DEPENDS:=+uclient-fetch +ubus +ip-tiny
  PROVIDES:=rmm-agent-runtime
  PKGARCH:=all
endef

define Package/rmm-agent/description
  Lightweight outbound polling agent for OpenWrt RMM.
endef

define Build/Compile
endef

define Package/rmm-agent/conffiles
/etc/rmm-agent.conf
/etc/config/rmm-agent
endef

define Package/rmm-agent/install
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_BIN) ./files/usr/bin/rmm-agent $(1)/usr/bin/rmm-agent
	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/etc/init.d/rmm-agent $(1)/etc/init.d/rmm-agent
	$(INSTALL_DIR) $(1)/etc
	$(INSTALL_CONF) ./files/etc/rmm-agent.conf $(1)/etc/rmm-agent.conf
	$(INSTALL_DIR) $(1)/etc/config
	$(INSTALL_CONF) ./files/etc/config/rmm-agent $(1)/etc/config/rmm-agent
	$(INSTALL_DIR) $(1)/usr/libexec
	$(INSTALL_BIN) ./files/usr/libexec/rmm-agent-uci-sync $(1)/usr/libexec/rmm-agent-uci-sync
endef

$(eval $(call BuildPackage,rmm-agent))
