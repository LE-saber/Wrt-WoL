# Wrt-Wol

Wrt-Wol is a lightweight Wake-on-LAN service for OpenWrt, ImmortalWrt, and Linux. It receives remote wake commands through Feishu, Telegram, or a self-hosted relay, then sends WoL magic packets inside the LAN.

Current version: `1.0.0`

## Languages

- English: `README.md`
- 中文：`docs/README.zh.md`
- 日本語: `docs/README.ja.md`
- Development document: `开发文档.md`

## Features

- Feishu bot integration through long connection events.
- Telegram bot integration through Bot API long polling.
- Self-hosted relay with a web page and HTTP API.
- Device management with names, MAC addresses, IP addresses, interfaces, broadcast addresses, and ports.
- Unified commands: `/list`, `/on`, `/on 1`, `/on <device>`, `/on all`, `/last`, `/whoami`, `/help`.
- OpenWrt LuCI page under `Services` → `Wrt-Wol`.

## OpenWrt Installation

Download the `.ipk` package from the GitHub Release page. If GitHub only provides the ZIP asset, extract the `.ipk` first.

```bash
scp feishu-wol_1.0.0_x86_64.ipk root@<router-ip>:/tmp/feishu-wol.ipk
ssh root@<router-ip>
opkg install /tmp/feishu-wol.ipk --force-checksum
rm -f /tmp/luci-indexcache
rm -rf /tmp/luci-modulecache
/etc/init.d/rpcd restart
/etc/init.d/uhttpd restart
```

The project name is `Wrt-Wol`; the OpenWrt package, binary, UCI config, and init service currently remain `feishu-wol` for upgrade compatibility.

## OpenWrt Configuration

After installation, open LuCI:

```text
Services → Wrt-Wol
```

Configure one or more triggers:

- Feishu: set `App ID` and `App Secret`, enable long connection events, and subscribe to `im.message.receive_v1`.
- Telegram: enable Telegram, set the BotFather token, and optionally restrict user or chat IDs.
- Self-hosted relay: enable the client, set the server URL and shared device token.

Then add wakeable devices in the device table. Each device should have at least a name and MAC address.

## Self-Hosted Relay Deployment

The self-hosted mode has two parts:

1. A server deployed on your own VPS or home server.
2. An OpenWrt client that polls the server and sends WoL packets inside the LAN.

This mode does not require Feishu or Telegram, and OpenWrt does not need a public inbound port.

### Server

Deploy with Docker:

```bash
git clone https://github.com/LE-saber/Wrt-WoL.git
cd Wrt-WoL/deploy/selfhost
cp config.yaml.example config.yaml
vi config.yaml
docker compose up -d --build
```

Minimal server config:

```yaml
selfhost:
  server:
    enabled: true
    host: "0.0.0.0"
    port: 8080
    admin_token: "change-this-admin-token"
    device_token: "change-this-device-token"
```

Open the web page:

```text
http://<server-ip>:8080/
```

For production, put the service behind HTTPS with Nginx or Caddy.

### OpenWrt Client

In LuCI, open:

```text
Services → Wrt-Wol → Self-hosted Service
```

Set:

- Enable self-hosted client: `on`
- Server URL: `https://wol.example.com`
- Device token: same as `selfhost.server.device_token`
- Poll interval: `5s`

Save, apply, then restart the service.

### HTTP API

Trigger wake from your own scripts:

```bash
curl -X POST https://wol.example.com/api/wake \
  -H "Authorization: Bearer <admin_token>" \
  -d "target=all"
```

`target` can be:

- `all`
- device index, for example `1`
- device name, for example `NAS`

## Chat Commands

```text
/help          Show help
/list          List configured devices
/on            Wake all devices
/on 1          Wake device #1
/on NAS        Wake the device named NAS
/on all        Wake all devices
/last          Show the latest wake result
/whoami        Show sender identity
```

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

## Troubleshooting

If LuCI does not show the page:

```bash
opkg install luci-compat
rm -f /tmp/luci-indexcache
rm -rf /tmp/luci-modulecache
/etc/init.d/rpcd restart
/etc/init.d/uhttpd restart
```

If the service status is wrong:

```bash
/etc/init.d/feishu-wol running
ubus call service list '{"name":"feishu-wol"}'
logread | grep -i feishu
cat /tmp/feishu-wol.log
```

## License

MIT License. See `LICENSE`.
