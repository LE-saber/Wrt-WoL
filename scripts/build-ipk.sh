#!/bin/bash
# build-ipk.sh — Build a feishu-wol .ipk compatible with OpenWrt/ImmortalWrt opkg.
#
# Writes the OpenWrt/ImmortalWrt .ipk as a gzip-compressed tar archive.
# Debian-style ar archives are rejected by opkg as malformed packages.
#
# Usage:
#   bash scripts/build-ipk.sh
#   VERSION=1.0.0 ARCH=x86_64 BINARY=dist/feishu-wol bash scripts/build-ipk.sh
set -e

PKGNAME="feishu-wol"
VERSION="${VERSION:-1.0.0}"
ARCH="${ARCH:-x86_64}"
BINARY="${BINARY:-dist/feishu-wol}"
OUTDIR="dist"

BUILDDIR="$(mktemp -d)"
trap "rm -rf $BUILDDIR" EXIT

echo "==> Building ${PKGNAME}_${VERSION}_${ARCH}.ipk"

# ── Validate ──────────────────────────────────────────────────────────────────
[ -f "$BINARY" ] || {
    echo "ERROR: binary not found at '$BINARY'"
    echo "       Run 'make build' (or the appropriate cross-compile target) first."
    exit 1
}
command -v python3 >/dev/null || { echo "ERROR: python3 is required"; exit 1; }

# ── Stage data/ ───────────────────────────────────────────────────────────────
DATA="$BUILDDIR/data"

install -Dm755 "$BINARY"                                                               "$DATA/usr/bin/feishu-wol"
install -Dm755 openwrt/files/etc/init.d/feishu-wol                                    "$DATA/etc/init.d/feishu-wol"
install -Dm644 openwrt/files/etc/config/feishu-wol                                    "$DATA/etc/config/feishu-wol"
install -Dm644 openwrt/files/etc/feishu-wol/config.yaml                               "$DATA/etc/feishu-wol/config.yaml"
install -Dm644 openwrt/files/usr/lib/lua/luci/controller/feishu_wol.lua               "$DATA/usr/lib/lua/luci/controller/feishu_wol.lua"
install -Dm644 openwrt/files/usr/lib/lua/luci/model/cbi/feishu_wol.lua                "$DATA/usr/lib/lua/luci/model/cbi/feishu_wol.lua"
install -Dm644 openwrt/files/usr/lib/lua/luci/view/feishu_wol/status.htm              "$DATA/usr/lib/lua/luci/view/feishu_wol/status.htm"
install -Dm644 openwrt/files/usr/share/luci/menu.d/feishu-wol.json                    "$DATA/usr/share/luci/menu.d/feishu-wol.json"
install -Dm644 openwrt/files/usr/share/rpcd/acl.d/luci-app-feishu-wol.json            "$DATA/usr/share/rpcd/acl.d/luci-app-feishu-wol.json"

# ── Stage control/ ────────────────────────────────────────────────────────────
CTRL="$BUILDDIR/control"
mkdir -p "$CTRL"

INSTALLED_SIZE=$(du -sk "$DATA" | awk '{print $1}')

cat > "$CTRL/control" <<EOF
Package: ${PKGNAME}
Version: ${VERSION}
Architecture: ${ARCH}
Depends: libc, luci-base, luci-compat
Section: net
Category: Network
Title: Wrt-Wol: multi-trigger Wake-on-LAN
Maintainer: feishu-wol
Installed-Size: ${INSTALLED_SIZE}
Description: Wake-on-LAN service with Feishu, Telegram and self-hosted triggers.
 Supports device management, /list, /on <device>, and LuCI configuration.
 Includes LuCI web UI under Services > Wrt-Wol.
EOF

cat > "$CTRL/conffiles" <<'EOF'
/etc/config/feishu-wol
/etc/feishu-wol/config.yaml
EOF

cat > "$CTRL/postinst" <<'EOF'
#!/bin/sh
[ "${IPKG_NO_SCRIPT}" = "1" ] && exit 0
/etc/init.d/feishu-wol enable 2>/dev/null || true
rm -rf /tmp/luci-indexcache /tmp/luci-modulecache 2>/dev/null || true
/etc/init.d/rpcd reload 2>/dev/null || /etc/init.d/rpcd restart 2>/dev/null || true
/etc/init.d/uhttpd reload 2>/dev/null || true
exit 0
EOF
chmod 755 "$CTRL/postinst"

cat > "$CTRL/prerm" <<'EOF'
#!/bin/sh
/etc/init.d/feishu-wol stop    2>/dev/null || true
/etc/init.d/feishu-wol disable 2>/dev/null || true
exit 0
EOF
chmod 755 "$CTRL/prerm"

# ── Create tarballs ───────────────────────────────────────────────────────────
( cd "$DATA" && tar czf "$BUILDDIR/data.tar.gz"    --owner=0 --group=0 . )
( cd "$CTRL" && tar czf "$BUILDDIR/control.tar.gz" --owner=0 --group=0 . )
printf '2.0\n' > "$BUILDDIR/debian-binary"

# ── Assemble .ipk as gzip tar ────────────────────────────────────────────────
mkdir -p "$OUTDIR"
IPK="$OUTDIR/${PKGNAME}_${VERSION}_${ARCH}.ipk"

( cd "$BUILDDIR" && tar czf "$OLDPWD/$IPK" \
    --owner=0 --group=0 \
    ./debian-binary ./control.tar.gz ./data.tar.gz )

SIZE=$(du -sh "$IPK" | awk '{print $1}')
echo "==> Built: $IPK ($SIZE)"
echo ""
echo "Deploy to router:"
echo "  scp $IPK root@<router-ip>:/tmp/"
echo "  ssh root@<router-ip> 'opkg install /tmp/$(basename $IPK) --force-checksum'"
