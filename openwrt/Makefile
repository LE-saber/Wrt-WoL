##############################################################################
# OpenWrt Package Makefile for Wrt-Wol
# Place this file at: <openwrt-sdk>/package/feishu-wol/Makefile
##############################################################################

include $(TOPDIR)/rules.mk

PKG_NAME      := feishu-wol
PKG_VERSION   := 1.0.0
PKG_RELEASE   := 1

PKG_SOURCE_PROTO  := git
PKG_SOURCE_URL    := https://github.com/LE-saber/Wrt-Wol.git
PKG_SOURCE_VERSION:= main
PKG_MIRROR_HASH   := skip

PKG_BUILD_DEPENDS := golang/host
PKG_BUILD_PARALLEL:= 1

GO_PKG        := github.com/LE-saber/Wrt-Wol
GO_PKG_BUILD_PKG := $(GO_PKG)/cmd/feishu-wol

include $(INCLUDE_DIR)/package.mk
include $(TOPDIR)/feeds/packages/lang/golang/golang-package.mk

define Package/feishu-wol
  SECTION  := net
  CATEGORY := Network
  TITLE    := Wrt-Wol multi-trigger Wake-on-LAN
  URL      := https://github.com/LE-saber/Wrt-Wol
  DEPENDS  := @(aarch64||arm||mips||mipsel||x86_64) +luci-base +luci-compat
endef

define Package/feishu-wol/description
  Lightweight OpenWrt service that listens for Feishu, Telegram, or
  self-hosted relay commands and sends Wake-on-LAN magic packets to wake
  LAN hosts on demand.
endef

define Package/feishu-wol/conffiles
/etc/config/feishu-wol
endef

GO_PKG_LDFLAGS_X := main.version=$(PKG_VERSION)

define Package/feishu-wol/install
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_BIN) $(GO_PKG_BUILD_BIN_DIR)/feishu-wol $(1)/usr/bin/feishu-wol

	$(INSTALL_DIR) $(1)/etc/feishu-wol
	$(INSTALL_CONF) ./files/etc/feishu-wol/config.yaml $(1)/etc/feishu-wol/config.yaml

	$(INSTALL_DIR) $(1)/etc/config
	$(INSTALL_CONF) ./files/etc/config/feishu-wol $(1)/etc/config/feishu-wol

	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/etc/init.d/feishu-wol $(1)/etc/init.d/feishu-wol

	$(INSTALL_DIR) $(1)/usr/lib/lua/luci/controller
	$(INSTALL_DATA) ./files/usr/lib/lua/luci/controller/feishu_wol.lua $(1)/usr/lib/lua/luci/controller/feishu_wol.lua
	$(INSTALL_DIR) $(1)/usr/lib/lua/luci/model/cbi
	$(INSTALL_DATA) ./files/usr/lib/lua/luci/model/cbi/feishu_wol.lua $(1)/usr/lib/lua/luci/model/cbi/feishu_wol.lua
	$(INSTALL_DIR) $(1)/usr/lib/lua/luci/view/feishu_wol
	$(INSTALL_DATA) ./files/usr/lib/lua/luci/view/feishu_wol/status.htm $(1)/usr/lib/lua/luci/view/feishu_wol/status.htm
	$(INSTALL_DIR) $(1)/usr/share/luci/menu.d
	$(INSTALL_DATA) ./files/usr/share/luci/menu.d/feishu-wol.json $(1)/usr/share/luci/menu.d/feishu-wol.json
	$(INSTALL_DIR) $(1)/usr/share/rpcd/acl.d
	$(INSTALL_DATA) ./files/usr/share/rpcd/acl.d/luci-app-feishu-wol.json $(1)/usr/share/rpcd/acl.d/luci-app-feishu-wol.json
endef

$(eval $(call GoBinPackage,feishu-wol))
$(eval $(call BuildPackage,feishu-wol))
