# Wrt-Wol

Wrt-Wol 是一个运行在 OpenWrt/ImmortalWrt 或普通 Linux 服务器上的 Wake-on-LAN 服务。它可以通过飞书、Telegram 或自建 Web 中转服务接收远程开机指令，并在局域网内发送 WoL Magic Packet。

当前版本：`1.0.0`

## 核心能力

- 飞书机器人：使用飞书开放平台长连接接收消息。
- Telegram 机器人：使用 Telegram Bot API 长轮询接收消息。
- 自建服务：云服务器提供 Web 页面和 API，OpenWrt 主动轮询获取开机指令。
- 设备管理：支持备注、MAC、IP、单设备网口、广播地址和端口。
- 统一命令：`/list`、`/on`、`/on 1`、`/on 设备名`、`/on all`、`/last`、`/whoami`。
- OpenWrt LuCI：后台 `服务` → `Wrt-Wol` 可完成全部常用配置。

## 快速安装 OpenWrt 包

```bash
scp dist/feishu-wol_1.0.0_x86_64.ipk root@<路由器IP>:/tmp/feishu-wol.ipk
ssh root@<路由器IP>
opkg install /tmp/feishu-wol.ipk --force-checksum
rm -f /tmp/luci-indexcache
rm -rf /tmp/luci-modulecache
/etc/init.d/rpcd restart
/etc/init.d/uhttpd restart
```

安装后进入 OpenWrt 后台：

```text
服务 → Wrt-Wol
```

说明：项目名称为 `Wrt-Wol`；OpenWrt 包名、二进制名、UCI 配置名和 init 服务名当前仍保留 `feishu-wol`，用于兼容已安装用户和升级路径。

## 构建

```bash
make build
make ipk
```

如果本机没有 `go` 在 PATH 中，但安装在 `/usr/local/go/bin/go`：

```bash
CGO_ENABLED=0 /usr/local/go/bin/go build -trimpath \
  -ldflags "-s -w -X main.version=1.0.0" \
  -o dist/feishu-wol ./cmd/feishu-wol
VERSION=1.0.0 ARCH=x86_64 BINARY=dist/feishu-wol OUTDIR=dist python3 scripts/build-ipk.py
```

## 文档

完整开发、部署、配置和扩展说明见：

```text
开发文档.md
```

多语言说明：

- 中文：`README.md`
- English：`docs/README.en.md`
- 日本語：`docs/README.ja.md`

旧版安装包和旧文档已归档到：

```text
deprecated/
```

## License

MIT License. See `LICENSE`.
