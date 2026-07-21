include $(TOPDIR)/rules.mk

PKG_NAME:=rmm-agent-go-production
PKG_VERSION:=0.6.0
PKG_RELEASE:=1

PKG_MAINTAINER:=RMM OpenWrt
PKG_LICENSE:=MIT

include $(INCLUDE_DIR)/package.mk

define Package/rmm-agent-go-production
  SECTION:=admin
  CATEGORY:=Administration
  TITLE:=OpenWrt RMM Go agent (production replacement)
  DEPENDS:=+ca-bundle +ip-tiny +iwinfo +openssh-client +openssh-keygen
  PROVIDES:=rmm-agent-runtime
  CONFLICTS:=rmm-agent
endef

define Package/rmm-agent-go-production/description
  Go implementation of the outbound polling agent installed as the primary rmm-agent service.
endef

define Build/Compile
	[ -x ./files/usr/bin/rmm-agent ]
endef

define Package/rmm-agent-go-production/conffiles
/etc/rmm-agent.conf
/etc/config/rmm-agent
endef

define Package/rmm-agent-go-production/install
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

$(eval $(call BuildPackage,rmm-agent-go-production))
