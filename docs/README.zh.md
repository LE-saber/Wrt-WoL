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

从 GitHub Release 下载 `.ipk`。如果 Release 只提供 ZIP 附件，先解压出 `.ipk`。

```bash
scp feishu-wol_1.0.0_x86_64.ipk root@<路由器IP>:/tmp/feishu-wol.ipk
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

## 自建服务器部署

自建模式由两部分组成：

1. 云服务器运行 `selfhost.server`，提供 Web 页面和中转 API。
2. OpenWrt 运行 `selfhost.client`，主动轮询云服务器，收到指令后在局域网内发送 WoL。

这种方式不依赖飞书或 Telegram，也不需要 OpenWrt 暴露公网入口。

### 服务器端

```bash
git clone https://github.com/LE-saber/Wrt-WoL.git
cd Wrt-WoL/deploy/selfhost
cp config.yaml.example config.yaml
vi config.yaml
docker compose up -d --build
```

核心配置：

```yaml
selfhost:
  server:
    enabled: true
    host: "0.0.0.0"
    port: 8080
    admin_token: "用于网页点击开机的管理 Token"
    device_token: "OpenWrt 客户端共享的设备 Token"
```

访问：

```text
http://服务器IP:8080/
```

生产环境建议使用 Nginx / Caddy 反向代理到 HTTPS。

### OpenWrt 端

后台进入：

```text
服务 → Wrt-Wol → 自建服务
```

填写：

- 启用自建服务客户端：开启
- 服务器地址：例如 `https://wol.example.com`
- 设备 Token：与服务器端 `device_token` 一致
- 轮询间隔：默认 `5s`

保存并应用后重启服务。

### API 调用

```bash
curl -X POST https://wol.example.com/api/wake \
  -H "Authorization: Bearer <admin_token>" \
  -d "target=all"
```

`target` 可填写 `all`、设备编号（如 `1`）或设备备注（如 `NAS`）。

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

- English：`README.md`
- 中文：`docs/README.zh.md`
- 日本語：`docs/README.ja.md`

旧版安装包和旧文档已归档到：

```text
deprecated/
```

## License

MIT License. See `LICENSE`.
