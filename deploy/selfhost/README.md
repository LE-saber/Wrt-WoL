# Wrt-Wol 自建服务器部署

自建模式由两部分组成：

1. 云服务器运行 `selfhost.server`，提供 Web 页面和中转 API。
2. OpenWrt 运行 `selfhost.client`，主动轮询云服务器，收到指令后在局域网内发送 WoL。

这种方式不依赖飞书或 Telegram，也不需要 OpenWrt 暴露公网入口。

## 服务器端

```bash
cd deploy/selfhost
cp config.yaml.example config.yaml
vi config.yaml
docker compose up -d --build
```

配置项：

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

网页里的目标设备可填写 `all`、设备编号（如 `1`）或设备备注（如 `NAS`）。

生产环境建议使用 Nginx / Caddy 反向代理到 HTTPS。

## OpenWrt 端

后台进入 `服务` → `Wrt-Wol` → `自建服务`：

- 勾选 `启用自建服务客户端`
- `服务器地址` 填云服务器地址，例如 `https://wol.example.com`
- `设备 Token` 填服务器端相同的 `device_token`
- `轮询间隔` 默认 `5s`

保存并应用后重启服务。

## API 调用

也可以不用网页，直接通过 API 触发：

```bash
curl -X POST https://wol.example.com/api/wake \
  -H "Authorization: Bearer <admin_token>" \
  -d "target=all"
```
