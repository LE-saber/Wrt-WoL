# Wrt-Wol

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

```bash
scp dist/feishu-wol_1.0.0_x86_64.ipk root@<router-ip>:/tmp/feishu-wol.ipk
ssh root@<router-ip>
opkg install /tmp/feishu-wol.ipk --force-checksum
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

その後、デバイス管理テーブルに起動対象を追加します。最低限、名前と MAC アドレスが必要です。

## チャットコマンド

```text
/help          ヘルプを表示
/list          登録済みデバイスを表示
/on            デフォルトデバイスを起動
/on 1          1 番のデバイスを起動
/on NAS        NAS という名前のデバイスを起動
/on all        すべてのデバイスを起動
/last          直近の起動結果を表示
/whoami        送信者情報を表示
```

## 自前中継サーバー

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

## ビルド

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

## ライセンス

MIT License. 詳細は `LICENSE` を参照してください。
