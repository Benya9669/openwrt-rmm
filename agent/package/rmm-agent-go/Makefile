include $(TOPDIR)/rules.mk

PKG_NAME:=rmm-agent-go
PKG_VERSION:=0.5.0
PKG_RELEASE:=2

PKG_MAINTAINER:=RMM OpenWrt
PKG_LICENSE:=MIT

include $(INCLUDE_DIR)/package.mk

define Package/rmm-agent-go
  SECTION:=admin
  CATEGORY:=Administration
  TITLE:=OpenWrt RMM Go agent
  DEPENDS:=+ca-bundle +ip-tiny +iwinfo +openssh-client +openssh-keygen
endef

define Package/rmm-agent-go/description
  Go implementation of the outbound polling agent for OpenWrt RMM.
endef

define Build/Compile
	[ -x ./files/usr/bin/rmm-agent-go ]
endef

define Package/rmm-agent-go/conffiles
/etc/rmm-agent-go.conf
endef

define Package/rmm-agent-go/install
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_BIN) ./files/usr/bin/rmm-agent-go $(1)/usr/bin/rmm-agent-go
	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/etc/init.d/rmm-agent-go $(1)/etc/init.d/rmm-agent-go
	$(INSTALL_DIR) $(1)/etc
	$(INSTALL_CONF) ./files/etc/rmm-agent-go.conf $(1)/etc/rmm-agent-go.conf
endef

$(eval $(call BuildPackage,rmm-agent-go))
