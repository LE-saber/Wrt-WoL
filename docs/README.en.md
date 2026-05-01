# Wrt-Wol

Wrt-Wol is a lightweight Wake-on-LAN service for OpenWrt, ImmortalWrt, and Linux. It receives remote wake commands through Feishu, Telegram, or a self-hosted relay, then sends WoL magic packets inside the LAN.

Current version: `1.0.0`

## Features

- Feishu bot integration through long connection events.
- Telegram bot integration through Bot API long polling.
- Self-hosted relay with a web page and HTTP API.
- Device management with names, MAC addresses, IP addresses, interfaces, broadcast addresses, and ports.
- Unified commands: `/list`, `/on`, `/on 1`, `/on <device>`, `/on all`, `/last`, `/whoami`, `/help`.
- OpenWrt LuCI page under `Services` → `Wrt-Wol`.

## OpenWrt Installation

```bash
scp dist/feishu-wol_1.0.0_x86_64.ipk root@<router-ip>:/tmp/feishu-wol.ipk
ssh root@<router-ip>
opkg install /tmp/feishu-wol.ipk --force-checksum
rm -f /tmp/luci-indexcache
rm -rf /tmp/luci-modulecache
/etc/init.d/rpcd restart
/etc/init.d/uhttpd restart
```

The project name is `Wrt-Wol`; the OpenWrt package, binary, UCI config, and init service currently remain `feishu-wol` for upgrade compatibility.

## Configuration

After installation, open LuCI:

```text
Services → Wrt-Wol
```

Configure one or more triggers:

- Feishu: set `App ID` and `App Secret`, enable long connection events, and subscribe to `im.message.receive_v1`.
- Telegram: enable Telegram, set the BotFather token, and optionally restrict user or chat IDs.
- Self-hosted relay: enable the client, set the server URL and shared device token.

Then add wakeable devices in the device table. Each device should have at least a name and MAC address.

## Chat Commands

```text
/help          Show help
/list          List configured devices
/on            Wake the default device
/on 1          Wake device #1
/on NAS        Wake the device named NAS
/on all        Wake all devices
/last          Show the latest wake result
/whoami        Show sender identity
```

## Self-Hosted Relay

```bash
cd deploy/selfhost
cp config.yaml.example config.yaml
vi config.yaml
docker compose up -d --build
```

Open:

```text
http://<server-ip>:8080/
```

For production, place the service behind HTTPS with Nginx or Caddy.

## Build

```bash
make build
make ipk
```

If Go is installed outside `PATH`:

```bash
CGO_ENABLED=0 /usr/local/go/bin/go build -trimpath \
  -ldflags "-s -w -X main.version=1.0.0" \
  -o dist/feishu-wol ./cmd/feishu-wol
VERSION=1.0.0 ARCH=x86_64 BINARY=dist/feishu-wol OUTDIR=dist python3 scripts/build-ipk.py
```

## License

MIT License. See `LICENSE`.
