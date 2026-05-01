#!/usr/bin/env python3
"""
build-ipk.py — Pure-Python .ipk builder for feishu-wol.

OpenWrt .ipk format:
  The outer container is a GZIP-compressed USTAR tar archive, containing:
    ./debian-binary   — plain text "2.0\n"
    ./control.tar.gz  — gzip-compressed USTAR tar of control files
    ./data.tar.gz     — gzip-compressed USTAR tar of package files

  This is different from Debian's .deb format which uses an ar archive as the
  outer container. OpenWrt/ImmortalWrt opkg extracts .ipk files as gzip tar
  archives, so ar/deb-shaped packages fail with "Malformed package file".

Key choices:
  - tarfile.USTAR_FORMAT  → most compatible; no PAX / GNU extensions
  - Python gzip module    → standard deflate, mtime=0 for reproducibility
  - ASCII-only control    → avoids UTF-8 parsing issues in opkg
"""

import os, sys, gzip, io, stat, tarfile

# ── Config ────────────────────────────────────────────────────────────────────
VERSION  = os.environ.get("VERSION",  "1.0.0")
ARCH     = os.environ.get("ARCH",     "x86_64")
BINARY   = os.environ.get("BINARY",   "dist/feishu-wol")
OUTDIR   = os.environ.get("OUTDIR",   "dist")
PKGNAME  = "feishu-wol"

# ── Helpers ───────────────────────────────────────────────────────────────────
def make_tar_gz(members: list[tuple[str, bytes, int]]) -> bytes:
    """
    Create a gzip-compressed USTAR tarball in memory.
    members: list of (arcname, content_bytes, mode)
    """
    raw = io.BytesIO()
    with tarfile.open(fileobj=raw, mode="w", format=tarfile.USTAR_FORMAT) as tf:
        for arcname, data, mode in members:
            info = tarfile.TarInfo(name=arcname)
            info.size  = len(data)
            info.mode  = mode
            info.uid   = 0
            info.gid   = 0
            info.mtime = 0
            info.type  = tarfile.REGTYPE
            tf.addfile(info, io.BytesIO(data))
    raw.seek(0)

    gz_buf = io.BytesIO()
    with gzip.GzipFile(filename="", mtime=0, mode="wb", fileobj=gz_buf) as gz:
        gz.write(raw.read())
    return gz_buf.getvalue()


def make_tar_gz_from_dir(src_dir: str) -> bytes:
    """
    Walk src_dir and add every file to a gzip USTAR tarball.
    Preserves execute bit; UIDs/GIDs are normalised to 0.
    """
    raw = io.BytesIO()
    with tarfile.open(fileobj=raw, mode="w", format=tarfile.USTAR_FORMAT) as tf:
        for root, dirs, files in os.walk(src_dir):
            dirs.sort()
            for fname in sorted(files):
                abs_path = os.path.join(root, fname)
                rel_path = os.path.relpath(abs_path, src_dir)
                arcname  = "./" + rel_path

                file_stat = os.stat(abs_path)
                mode = 0o755 if file_stat.st_mode & stat.S_IXUSR else 0o644

                info = tarfile.TarInfo(name=arcname)
                info.size  = file_stat.st_size
                info.mode  = mode
                info.uid   = 0
                info.gid   = 0
                info.mtime = 0
                info.type  = tarfile.REGTYPE

                with open(abs_path, "rb") as fh:
                    tf.addfile(info, fh)

    raw.seek(0)
    gz_buf = io.BytesIO()
    with gzip.GzipFile(filename="", mtime=0, mode="wb", fileobj=gz_buf) as gz:
        gz.write(raw.read())
    return gz_buf.getvalue()


def write_ipk(out_path: str, debian_binary: bytes,
              ctrl_gz: bytes, data_gz: bytes):
    """
    Write the .ipk as a gzip-compressed USTAR tar.

    Structure:
      outer.tar.gz
        ./debian-binary
        ./control.tar.gz
        ./data.tar.gz
    """
    members = [
        ("./debian-binary",  debian_binary, 0o644),
        ("./control.tar.gz", ctrl_gz,       0o644),
        ("./data.tar.gz",    data_gz,       0o644),
    ]
    ipk_bytes = make_tar_gz(members)
    with open(out_path, "wb") as f:
        f.write(ipk_bytes)


# ── Staged data tree ──────────────────────────────────────────────────────────
def stage_files(staging_dir: str):
    """Copy package files into staging_dir."""
    import shutil

    def install(src, dst, executable=False):
        os.makedirs(os.path.dirname(dst), exist_ok=True)
        shutil.copy2(src, dst)
        os.chmod(dst, 0o755 if executable else 0o644)

    install(BINARY,
            f"{staging_dir}/usr/bin/feishu-wol",
            executable=True)
    install("openwrt/files/etc/init.d/feishu-wol",
            f"{staging_dir}/etc/init.d/feishu-wol",
            executable=True)
    install("openwrt/files/etc/config/feishu-wol",
            f"{staging_dir}/etc/config/feishu-wol")
    install("openwrt/files/etc/feishu-wol/config.yaml",
            f"{staging_dir}/etc/feishu-wol/config.yaml")
    install("openwrt/files/usr/lib/lua/luci/controller/feishu_wol.lua",
            f"{staging_dir}/usr/lib/lua/luci/controller/feishu_wol.lua")
    install("openwrt/files/usr/lib/lua/luci/model/cbi/feishu_wol.lua",
            f"{staging_dir}/usr/lib/lua/luci/model/cbi/feishu_wol.lua")
    install("openwrt/files/usr/lib/lua/luci/view/feishu_wol/status.htm",
            f"{staging_dir}/usr/lib/lua/luci/view/feishu_wol/status.htm")
    install("openwrt/files/usr/share/luci/menu.d/feishu-wol.json",
            f"{staging_dir}/usr/share/luci/menu.d/feishu-wol.json")
    install("openwrt/files/usr/share/rpcd/acl.d/luci-app-feishu-wol.json",
            f"{staging_dir}/usr/share/rpcd/acl.d/luci-app-feishu-wol.json")


# ── Control files ─────────────────────────────────────────────────────────────
def build_control_tar_gz(installed_kb: int) -> bytes:
    control = (
        f"Package: {PKGNAME}\n"
        f"Version: {VERSION}\n"
        f"Architecture: {ARCH}\n"
        f"Depends: libc, luci-base, luci-compat\n"
        f"Section: net\n"
        f"Category: Network\n"
        f"Title: Wrt-Wol: multi-trigger Wake-on-LAN\n"
        f"Maintainer: feishu-wol\n"
        f"Installed-Size: {installed_kb}\n"
        f"Description: Wake-on-LAN service with Feishu, Telegram and self-hosted triggers.\n"
        f" Supports device management, /list, /on <device>, and LuCI configuration.\n"
        f" Includes LuCI web UI under Services > Wrt-Wol.\n"
    ).encode("ascii")

    conffiles = b"/etc/config/feishu-wol\n/etc/feishu-wol/config.yaml\n"

    postinst = (
        b"#!/bin/sh\n"
        b"[ -n \"${IPKG_INSTROOT}\" ] && exit 0\n"
        b"[ \"${IPKG_NO_SCRIPT}\" = \"1\" ] && exit 0\n"
        b"/etc/init.d/feishu-wol enable 2>/dev/null || true\n"
        b"rm -f /tmp/luci-indexcache\n"
        b"rm -rf /tmp/luci-modulecache\n"
        b"/etc/init.d/rpcd reload 2>/dev/null || /etc/init.d/rpcd restart 2>/dev/null || true\n"
        b"/etc/init.d/uhttpd reload 2>/dev/null || true\n"
        b"exit 0\n"
    )

    prerm = (
        b"#!/bin/sh\n"
        b"[ -n \"${IPKG_INSTROOT}\" ] && exit 0\n"
        b"[ \"${IPKG_NO_SCRIPT}\" = \"1\" ] && exit 0\n"
        b"/etc/init.d/feishu-wol stop    2>/dev/null || true\n"
        b"/etc/init.d/feishu-wol disable 2>/dev/null || true\n"
        b"rm -f /tmp/luci-indexcache\n"
        b"rm -rf /tmp/luci-modulecache\n"
        b"exit 0\n"
    )

    return make_tar_gz([
        ("./control",   control,   0o644),
        ("./conffiles", conffiles, 0o644),
        ("./postinst",  postinst,  0o755),
        ("./prerm",     prerm,     0o755),
    ])


# ── Main ──────────────────────────────────────────────────────────────────────
def main():
    import shutil, tempfile

    os.makedirs(OUTDIR, exist_ok=True)

    if not os.path.isfile(BINARY):
        print(f"ERROR: binary not found: {BINARY}")
        print("       Run 'make build' first.")
        sys.exit(1)

    print(f"==> Building {PKGNAME}_{VERSION}_{ARCH}.ipk")

    staging = tempfile.mkdtemp(prefix="feishu-wol-staging-")
    try:
        stage_files(staging)

        total = sum(
            os.path.getsize(os.path.join(r, f))
            for r, _, files in os.walk(staging)
            for f in files
        )
        installed_kb = (total + 1023) // 1024
        print(f"    staging: {total} bytes ({installed_kb} KB installed)")

        print("    building control.tar.gz ...")
        ctrl_gz = build_control_tar_gz(installed_kb)

        print("    building data.tar.gz ...")
        data_gz = make_tar_gz_from_dir(staging)

        print(f"    control.tar.gz: {len(ctrl_gz)} bytes")
        print(f"    data.tar.gz:    {len(data_gz)} bytes")

    finally:
        shutil.rmtree(staging, ignore_errors=True)

    out_path = os.path.join(OUTDIR, f"{PKGNAME}_{VERSION}_{ARCH}.ipk")
    write_ipk(out_path, b"2.0\n", ctrl_gz, data_gz)

    size_kb = os.path.getsize(out_path) // 1024
    print(f"==> Built: {out_path} ({size_kb} KB)")
    print()
    print("Deploy:")
    print(f"  scp {out_path} root@<router>:/tmp/feishu-wol.ipk")
    print(f"  ssh root@<router> 'opkg install /tmp/feishu-wol.ipk --force-checksum'")


if __name__ == "__main__":
    main()
