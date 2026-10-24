# Wrt-Wol

[English](../README.md) · [简体中文](README.zh.md) · [繁體中文](README.zh-TW.md) · [日本語](README.ja.md)

Wrt-Wol は OpenWrt、ImmortalWrt、通常の Linux サーバーで動作する軽量な Wake-on-LAN サービスです。Feishu、Telegram、または自前の中継サーバーからリモート起動コマンドを受け取り、LAN 内へ WoL Magic Packet を送信します。

現在のバージョン: `1.0.0`

## 主な機能

- Feishu ボット: 長接続イベントでメッセージを受信。
- Telegram ボット: Bot API のロングポーリングでコマンドを受信。
- 自前中継サーバー: Web ページと HTTP API を提供。
- デバイス管理: 名前、MAC、IP、送信インターフェース、ブロードキャストアドレス、ポートを設定可能。
- 共通コマンド: `/list`、`/on`、`/on 1`、`/on <device>`、`/on all`、`/last`、`/whoami`、`/help`。
- OpenWrt LuCI: `サービス` → `Wrt-Wol` から設定可能。

## OpenWrt へのインストール

[v1.0.0 Release](https://github.com/LE-saber/Wrt-WoL/releases/tag/v1.0.0) の `Wrt-Wol_1.0.0_x86_64.ipk` を使用します。ルーターが x86_64 であることを確認し、他の構成には対応するビルドを用意してください。対象 PC 側で Wake-on-LAN を有効にする必要があります。パケットの送信成功は起動完了を意味しません。

```bash
scp Wrt-Wol_1.0.0_x86_64.ipk root@<router-ip>:/tmp/feishu-wol.ipk
ssh root@<router-ip>
opkg install /tmp/feishu-wol.ipk
rm -f /tmp/luci-indexcache
rm -rf /tmp/luci-modulecache
/etc/init.d/rpcd restart
/etc/init.d/uhttpd restart
```

プロジェクト名は `Wrt-Wol` です。OpenWrt のパッケージ名、実行ファイル名、UCI 設定名、init サービス名は、アップグレード互換性のため現在も `feishu-wol` のままです。

## 設定

インストール後、LuCI を開きます。

```text
サービス → Wrt-Wol
```

利用する入口を設定します。

- Feishu: `App ID` と `App Secret` を設定し、長接続イベントを有効化して `im.message.receive_v1` を購読します。
- Telegram: Telegram を有効化し、BotFather の token を設定します。必要に応じて user ID / chat ID を制限します。
- 自前中継: クライアントを有効化し、サーバー URL と共有 device token を設定します。

その後、デバイス管理テーブルに起動対象を追加します。最低限、名前と MAC アドレスが必要です。ボットの許可リストも設定してください。空の場合、そのボットが受信したすべての送信者を許可します。ボットの応答と中継 UI は現在中国語で、文書の四言語対応はアプリの UI 言語を変更しません。

## チャットコマンド

```text
/help          ヘルプを表示
/list          登録済みデバイスを表示
/on            すべてのデバイスを起動
/on 1          1 番のデバイスを起動
/on NAS        NAS という名前のデバイスを起動
/on all        すべてのデバイスを起動
/last          直近の起動結果を表示
/whoami        送信者情報を表示
```

## 自前中継サーバー

サーバーが Web/API を提供し、OpenWrt クライアントが外向きにポーリングして LAN 内に WoL を送信します。OpenWrt 側で公開受信ポートを開く必要はありません。最初にリポジトリをクローンし、そのルートから実行してください。

```bash
cd deploy/selfhost
cp config.yaml.example config.yaml
vi config.yaml
docker compose up -d --build
```

ブラウザで開きます。

```text
http://<server-ip>:8080/
```

本番環境では Nginx または Caddy で HTTPS 化することを推奨します。

LuCI の自前中継設定でクライアントを有効にし、サーバー URL、サーバーと同じ `device_token`、ポーリング間隔（既定 `5s`）を設定します。サーバー設定例と Docker 手順は[配備ガイド](../deploy/selfhost/README.md)を参照してください。管理用 `admin_token` とデバイス用 `device_token` は別々に設定します。

```sh
curl -X POST https://wol.example.com/api/wake \
  -H "Authorization: Bearer <admin_token>" \
  -d "target=all"
```

`target` は `all`、デバイス番号（`1`）、名前（`NAS`）です。Web ページは `http://<server-ip>:8080/`。中継は最新の命令と報告をメモリに保存し、永続的なジョブキューではありません。

## ビルド

[go.mod](../go.mod) が指定する Go 1.26、Make、パッケージ生成用 Python 3 が必要です。`make ipk` はホストのアーキテクチャを使うため、対応する Linux ホストで実行してください。クロスビルドのターゲットは [Makefile](../Makefile) を参照してください。

```bash
make build
make ipk
```

Go が `PATH` にない場合:

```bash
CGO_ENABLED=0 /usr/local/go/bin/go build -trimpath \
  -ldflags "-s -w -X main.version=1.0.0" \
  -o dist/feishu-wol ./cmd/feishu-wol
VERSION=1.0.0 ARCH=x86_64 BINARY=dist/feishu-wol OUTDIR=dist python3 scripts/build-ipk.py
```

## トラブルシューティング

LuCI ページが表示されない場合は互換ライブラリとキャッシュを確認します。

```sh
opkg install luci-compat
rm -f /tmp/luci-indexcache
rm -rf /tmp/luci-modulecache
/etc/init.d/rpcd restart
/etc/init.d/uhttpd restart
```

サービスとログの確認：

```sh
/etc/init.d/feishu-wol running
ubus call service list '{"name":"feishu-wol"}'
logread | grep -i feishu
cat /tmp/feishu-wol.log
```

[開発文書](../开发文档.md) · [配備ガイド](../deploy/selfhost/README.md) · [旧版文書](../deprecated/old-docs/)

## ライセンス

MIT License. 詳細は [LICENSE](../LICENSE) を参照してください。
