# Wrt-Wol

[English](../README.md) · [简体中文](README.zh.md) · [繁體中文](README.zh-TW.md) · [日本語](README.ja.md)

Wrt-Wol 是執行於 OpenWrt、ImmortalWrt 或 Linux 的輕量 Wake-on-LAN 服務。透過飛書、Telegram 或自行架設的 Web 中繼接收遠端喚醒指令，再於區域網路傳送 WoL Magic Packet。

目前版本：`1.0.0` · [下載](https://github.com/LE-saber/Wrt-WoL/releases/tag/v1.0.0)

## 主要功能

- 飛書機器人使用長連線事件，Telegram 機器人使用 Bot API 長輪詢。
- 自架中繼提供 Web 頁面與 HTTP API，OpenWrt 主動輪詢，不需要開放路由器的公網入口。
- 逐台設定名稱、MAC、IP、傳送介面、廣播位址與連接埠。
- 共用聊天指令及 LuCI「服務 → Wrt-Wol」設定頁面。

## 安裝 OpenWrt 套件

發布附件為 `Wrt-Wol_1.0.0_x86_64.ipk`，請先確認路由器架構為 x86_64；其他架構需建置相符版本。目標電腦須支援並啟用 Wake-on-LAN，傳送成功不代表已開機。

```sh
scp Wrt-Wol_1.0.0_x86_64.ipk root@<router-ip>:/tmp/feishu-wol.ipk
ssh root@<router-ip>
opkg install /tmp/feishu-wol.ipk
rm -f /tmp/luci-indexcache
rm -rf /tmp/luci-modulecache
/etc/init.d/rpcd restart
/etc/init.d/uhttpd restart
```

專案名稱為 `Wrt-Wol`；套件內部名稱、執行檔、UCI 設定與 init 服務仍使用 `feishu-wol`，以維持升級相容性。

## 設定與指令

在 LuCI「服務 → Wrt-Wol」啟用一個或多個入口：

- 飛書：填入 App ID / App Secret、啟用長連線並訂閱 `im.message.receive_v1`。
- Telegram：填入 BotFather Token，設定允許的使用者或聊天室 ID。
- 自架中繼：填入伺服器 URL 與共用裝置 Token。
- 裝置表至少填入名稱與 MAC，再依網路設定介面、廣播位址、連接埠與選填 IP。

請設定機器人允許清單；空清單代表允許該機器人接收到的所有傳送者。機器人回覆與中繼介面目前使用中文，文件四語化不會變更程式介面語言。

| 指令 | 用途 |
| --- | --- |
| `/help` | 顯示說明 |
| `/list` | 列出裝置 |
| `/on`、`/on all` | 喚醒全部裝置 |
| `/on 1` | 喚醒第 1 台裝置 |
| `/on NAS` | 依名稱選擇裝置 |
| `/last` | 查看最近喚醒記錄 |
| `/whoami` | 查看傳送者/聊天室 ID |

## 自架中繼

伺服器接收指令，OpenWrt 用戶端輪詢取得指令並於 LAN 內傳送 WoL，不依賴飛書或 Telegram。

```sh
git clone https://github.com/LE-saber/Wrt-WoL.git
cd Wrt-WoL/deploy/selfhost
cp config.yaml.example config.yaml
vi config.yaml
docker compose up -d --build
```

將管理與裝置 Token 分別替換為自己的值：

```yaml
selfhost:
  server:
    enabled: true
    host: "0.0.0.0"
    port: 8080
    admin_token: "change-this-admin-token"
    device_token: "change-this-device-token"
```

頁面位址為 `http://<server-ip>:8080/`，對外部署時以 Nginx 或 Caddy 提供 HTTPS。LuCI 自架服務中啟用用戶端，設定伺服器 URL、相同的 `device_token`，輪詢間隔預設 `5s`，儲存套用並重啟服務。

```sh
curl -X POST https://wol.example.com/api/wake \
  -H "Authorization: Bearer <admin_token>" \
  -d "target=all"
```

`target` 可為 `all`、裝置編號（`1`）或名稱（`NAS`）。中繼以記憶體保存最新指令與回報，不是持久化工作佇列。

## 建置

需要 [go.mod](../go.mod) 指定的 Go 1.26、Make 及打包用 Python 3。原生套件請在架構相符的 Linux 主機上建置；`make ipk` 使用主機架構，交叉編譯目標見 [Makefile](../Makefile)。

```sh
make build
make ipk
```

Go 不在 PATH 時，可使用明確路徑：

```sh
CGO_ENABLED=0 /usr/local/go/bin/go build -trimpath \
  -ldflags "-s -w -X main.version=1.0.0" \
  -o dist/feishu-wol ./cmd/feishu-wol
VERSION=1.0.0 ARCH=x86_64 BINARY=dist/feishu-wol OUTDIR=dist python3 scripts/build-ipk.py
```

## 疑難排解

LuCI 頁面未出現時，安裝 `luci-compat`，清除快取並重新啟動相關服務：

```sh
opkg install luci-compat
rm -f /tmp/luci-indexcache
rm -rf /tmp/luci-modulecache
/etc/init.d/rpcd restart
/etc/init.d/uhttpd restart
```

檢查狀態與記錄：

```sh
/etc/init.d/feishu-wol running
ubus call service list '{"name":"feishu-wol"}'
logread | grep -i feishu
cat /tmp/feishu-wol.log
```

## 文件與授權

[開發文件](../开发文档.md) · [中繼部署指南](../deploy/selfhost/README.md) · [歷史文件](../deprecated/old-docs/)

MIT 授權，詳見 [LICENSE](../LICENSE)。
