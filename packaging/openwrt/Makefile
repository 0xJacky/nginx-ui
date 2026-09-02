include $(TOPDIR)/rules.mk

PKG_NAME:=nginx-ui
PKG_VERSION:=$(if $(NGINX_UI_VERSION),$(NGINX_UI_VERSION),0.0.0)
PKG_RELEASE:=1
PKG_LICENSE:=AGPL-3.0-only
PKG_LICENSE_FILES:=LICENSE
PKG_MAINTAINER:=Nginx UI maintainers <support@nginxui.com>

include $(INCLUDE_DIR)/package.mk

define Package/nginx-ui
  SECTION:=admin
  CATEGORY:=Administration
  TITLE:=Web interface for managing Nginx
  URL:=https://nginxui.com
  DEPENDS:=+ca-bundle
endef

define Package/nginx-ui/description
 Nginx UI is a web interface for managing an existing Nginx installation.
 This package installs the Nginx UI backend, configuration, and procd service.
endef

define Package/nginx-ui/conffiles
/etc/nginx-ui/app.ini
endef

define Build/Prepare
	mkdir -p $(PKG_BUILD_DIR)
	$(CP) ./files/nginx-ui $(PKG_BUILD_DIR)/nginx-ui
	$(CP) ./files/LICENSE $(PKG_BUILD_DIR)/LICENSE
endef

define Build/Compile
endef

define Package/nginx-ui/install
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_BIN) $(PKG_BUILD_DIR)/nginx-ui $(1)/usr/bin/nginx-ui
	$(INSTALL_DIR) $(1)/etc/nginx-ui
	$(INSTALL_CONF) ./files/app.ini $(1)/etc/nginx-ui/app.ini
	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/nginx-ui.init $(1)/etc/init.d/nginx-ui
	$(INSTALL_DIR) $(1)/lib/upgrade/keep.d
	$(INSTALL_DATA) ./files/nginx-ui.keep $(1)/lib/upgrade/keep.d/nginx-ui
endef

$(eval $(call BuildPackage,nginx-ui))
