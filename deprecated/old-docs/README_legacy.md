# feishu-wol

通过飞书机器人远程唤醒局域网内 PC 的轻量级 OpenWrt 服务。

## 项目介绍

**feishu-wol** 是一个运行在 OpenWrt 路由器上的 Go 语言服务。用户在飞书手机客户端向机器人发送 `/on` 指令，路由器程序接收后通过 **WoL（Wake-on-LAN）Magic Packet** 唤醒局域网内的指定 PC。

### 功能特性

- **飞书机器人接入** — 飞书开放平台事件订阅 v2.0，支持 AES-256-CBC 加密与签名验证
- **WoL 唤醒** — 程序内部构造 102 字节 Magic Packet，通过 UDP 广播发送，无需额外工具
- **OpenWrt 原生支持** — 静态编译二进制，提供 procd init.d 服务脚本，可打包为 `.ipk`
- **Docker 支持** — 容器化部署，使用 host 网络直连物理网卡
- **白名单安全控制** — 仅允许指定飞书用户 Open ID 或群 Chat ID 触发唤醒
- **完整日志** — 结构化日志同时输出到 stderr 和日志文件

## 架构说明

```
手机飞书 App          飞书服务器          OpenWrt 路由器           目标 PC
  /on 指令   ──→   事件回调 POST   ──→   feishu-wol 服务   ──→   WoL 唤醒
                                         ↓
                                   验证签名 & 解密
                                   检查白名单
                                   构造 Magic Packet
                                   UDP 广播 :9
```

消息流程：
1. 用户在飞书向机器人发送 `/on`
2. 飞书服务器 POST 事件到路由器上的回调地址
3. `feishu-wol` 验证 `X-Lark-Signature` 签名，AES 解密消息体
4. 检查发送者 Open ID / Chat ID 是否在白名单内
5. 构造 102 字节 WoL Magic Packet，通过指定网卡 UDP 广播到局域网
6. 目标 PC 网卡收到 Magic Packet 后开机，程序回复"已发送开机指令"

## 前置条件

### PC 端
- BIOS/UEFI 中开启 **Wake-on-LAN**（电源管理 → 网络唤醒）
- 网卡驱动开启 WoL，Linux 下验证：
  ```bash
  ethtool eth0 | grep "Wake-on"
  # Wake-on: g  ← 表示已开启
  # Wake-on: d  ← 需要执行：sudo ethtool -s eth0 wol g
  ```
- 关机状态下主板仍需通电（ATX 5V 待机），即插着电源线

### 路由器端
- OpenWrt 路由器，CPU 架构之一：mipsel / mips / armv7 / arm64 / x86_64
- 路由器有**公网 IP** 或已配置**内网穿透**（飞书服务器需能回调到路由器）

### 飞书端
- 飞书开放平台企业自建应用账号

---

## 飞书机器人创建步骤

### 1. 创建自建应用

访问 [飞书开放平台](https://open.feishu.cn/app) → 创建企业自建应用，填写名称（如"远程唤醒"）。

### 2. 开启机器人能力

应用详情 → **功能** → **机器人** → 开启。

### 3. 添加权限

权限管理 → 搜索并添加：
- `im:message`（读取消息）
- `im:message:send_as_bot`（机器人发消息）

### 4. 配置事件订阅

事件订阅页面：
1. 设置回调 URL（服务启动后才能通过校验）：
   ```
   http://你的公网IP:8080/webhook/feishu
   ```
2. 加密配置 → 开启加密，记录 **Encrypt Key** 和 **Verification Token**
3. 添加事件：`im.message.receive_v1`（接收消息）

### 5. 获取凭证

应用凭证页面记录：
| 字段 | 配置项 |
|------|--------|
| App ID | `feishu.app_id` |
| App Secret | `feishu.app_secret` |
| Verification Token | `feishu.verification_token` |
| Encrypt Key | `feishu.encrypt_key` |

### 6. 发布应用

版本管理与发布 → 创建版本 → 提交发布（企业内部应用通常即时生效）。

### 7. 获取用户 Open ID

将用户 Open ID 加入白名单的方法：
- **方式 A**：先不配置白名单（`allowed_open_ids: []`），让用户发送任意消息，查看日志中打印的 `open_id` 字段，再填入配置
- **方式 B**：飞书开放平台 → API 调试工具 → `contact:user:get` 接口查询

---

## 配置文件说明

`config.yaml` 完整示例：

```yaml
feishu:
  app_id: "cli_your_app_id"           # 飞书自建应用 App ID
  app_secret: "your_app_secret"       # 飞书自建应用 App Secret
  verification_token: "your_token"    # 事件订阅 Verification Token（用于校验消息来源）
  encrypt_key: "your_encrypt_key"     # 事件订阅 Encrypt Key（留空则不加密，不推荐）

wol:
  mac_addresses:
    - "AA:BB:CC:DD:EE:FF"             # 目标 PC 的 MAC 地址，支持多台
  interface: "br-lan"                 # 发送 WoL 的网卡（OpenWrt LAN 桥接口）
  port: 9                             # WoL UDP 目标端口（默认 9，也可用 7）
  broadcast_ip: "255.255.255.255"     # 广播地址（也可指定子网广播如 192.168.1.255）

server:
  host: "0.0.0.0"                     # 监听地址
  port: 8080                          # 监听端口（与飞书回调 URL 一致）
  path: "/webhook/feishu"             # Webhook 路径

security:
  allowed_open_ids:                   # 允许触发的飞书用户 Open ID（ou_ 前缀）
    - "ou_xxxxxxxxxxxxxxxxxxxxxxxx"
  allowed_chat_ids:                   # 允许触发的群聊 Chat ID（oc_ 前缀），留空则不限群
    - []

log:
  level: "info"                       # 日志级别：debug / info / warn / error
  file: "/var/log/feishu-wol.log"     # 日志文件路径，留空仅输出到 stderr
```

> **安全提示**：`encrypt_key` 和 `app_secret` 是敏感信息，确保配置文件权限为 `600`（`chmod 600 /etc/feishu-wol/config.yaml`）。

---

## 安装方式一：OpenWrt .ipk

### 1. 确认路由器架构

```bash
ssh root@192.168.1.1 "uname -m && cat /proc/cpuinfo | grep 'model name' | head -1"
```

| `uname -m` | 编译目标 |
|------------|---------|
| `mipsel` | `make build-mipsel` |
| `mips` | `make build-mips` |
| `armv7l` | `make build-arm` |
| `aarch64` | `make build-arm64` |
| `x86_64` | `make build` |

### 2. 交叉编译（需本机安装 Go 1.21+）

```bash
git clone https://github.com/soulteary/feishu-wol
cd feishu-wol
make build-mipsel   # 根据架构选择
```

输出文件：`feishu-wol-mipsel`

### 3. 手动安装到路由器

```bash
# 上传二进制
scp feishu-wol-mipsel root@192.168.1.1:/usr/bin/feishu-wol
ssh root@192.168.1.1 "chmod +x /usr/bin/feishu-wol"

# 上传 init.d 脚本
scp openwrt/files/etc/init.d/feishu-wol root@192.168.1.1:/etc/init.d/feishu-wol
ssh root@192.168.1.1 "chmod +x /etc/init.d/feishu-wol"

# 上传并编辑配置文件
ssh root@192.168.1.1 "mkdir -p /etc/feishu-wol"
scp config.yaml.example root@192.168.1.1:/etc/feishu-wol/config.yaml
ssh root@192.168.1.1 "vi /etc/feishu-wol/config.yaml"
```

### 4. 服务管理

```bash
/etc/init.d/feishu-wol enable    # 设置开机自启
/etc/init.d/feishu-wol start     # 启动
/etc/init.d/feishu-wol stop      # 停止
/etc/init.d/feishu-wol restart   # 重启
```

### 5. 查看日志

```bash
tail -f /var/log/feishu-wol.log
# 或
logread | grep feishu-wol
```

---

## 安装方式二：Docker

> 适用于支持 Docker 的 OpenWrt 设备，或同一局域网内的 Linux 服务器。

**必须使用 `network_mode: host`**：WoL Magic Packet 需通过宿主机物理网卡发出 UDP 广播，bridge 网络无法穿透到 LAN。

```bash
# 准备配置
mkdir -p /opt/feishu-wol
cp config.yaml.example /opt/feishu-wol/config.yaml
vi /opt/feishu-wol/config.yaml

# 启动
docker compose up -d

# 查看日志
docker compose logs -f
```

---

## 公网访问方案

飞书服务器需能从公网访问到路由器的回调地址，三选一：

### 方案 A：路由器有公网 IP（最简单）

在 OpenWrt 防火墙开放 8080 端口入站，建议限制来源为飞书服务器 IP 段：

```bash
# LuCI → 网络 → 防火墙 → 端口转发，或 UCI：
uci add firewall rule
uci set firewall.@rule[-1].name='feishu-wol-inbound'
uci set firewall.@rule[-1].src='wan'
uci set firewall.@rule[-1].dest_port='8080'
uci set firewall.@rule[-1].target='ACCEPT'
uci set firewall.@rule[-1].proto='tcp'
uci commit firewall && /etc/init.d/firewall reload
```

### 方案 B：内网穿透（无公网 IP 首选）

使用 frp，在公网服务器部署 frps，路由器部署 frpc：

```ini
# frpc.ini（路由器端）
[common]
server_addr = your.public.server.com
server_port = 7000

[feishu-wol]
type = tcp
local_ip = 127.0.0.1
local_port = 8080
remote_port = 18080
```

飞书回调 URL 改为：`http://your.public.server.com:18080/webhook/feishu`

### 方案 C：WireGuard VPN

在公网服务器运行 WireGuard，路由器作为 peer 接入，通过 WireGuard 隧道转发飞书回调流量。适合已有 VPN 基础设施的场景。

---

## 测试

### 1. 发送 WoL 测试包（不启动 HTTP 服务）

```bash
feishu-wol -config /etc/feishu-wol/config.yaml -test-wol
```

### 2. 模拟飞书 URL 验证 Challenge

```bash
curl -X POST http://localhost:8080/webhook/feishu \
  -H "Content-Type: application/json" \
  -d '{"challenge":"test_challenge_string","token":"your_verification_token","type":"url_verification"}'
# 期望响应：{"challenge":"test_challenge_string"}
```

### 3. 健康检查

```bash
curl http://localhost:8080/healthz
# 期望响应：ok
```

### 4. 飞书端完整测试

1. 启动服务并确认日志显示 `HTTP server listening`
2. 在飞书中与机器人私聊发送 `/on`
3. 服务日志应出现：`message received`、`WoL sent successfully`
4. 机器人回复"✅ 已向 [...] 发送 Wake-on-LAN 开机指令！"
5. 目标 PC 启动

---

## 故障排查

| 问题 | 可能原因 | 解决方法 |
|------|----------|----------|
| 飞书回调返回 401 | Encrypt Key / Verification Token 不匹配 | 对照飞书后台检查 `config.yaml` 中的 `encrypt_key` 和 `verification_token` |
| 日志出现 `signature mismatch` | 请求非来自飞书或 Encrypt Key 错误 | 确认 Encrypt Key 配置正确；检查是否有代理修改了请求体 |
| 飞书回调无响应 / 超时 | 端口未开放或内网穿透未生效 | 从公网 `curl` 回调 URL 测试连通性；检查防火墙规则 |
| WoL 发包成功但 PC 不开机 | BIOS WoL 未开启 / 网卡 WoL 未启用 / 断电过 | 进 BIOS 确认；`ethtool eth0` 确认 `Wake-on: g`；检查 PC 是否完全断电（ATX 5V 待机需保持） |
| `interface not found` | 配置的网卡名不存在 | 在路由器执行 `ip link show` 确认 LAN 桥接口名，通常为 `br-lan` |
| 权限被拒绝 / `access denied` | 发送者不在白名单 | 查看日志中的 `open_id`，添加到 `security.allowed_open_ids` |
| 机器人无法回复消息 | App Secret 错误或权限不足 | 检查 `app_id` / `app_secret`；确认已申请 `im:message:send_as_bot` 权限 |

---

## 安全建议

1. **启用 Encrypt Key**：强制 AES-256-CBC 加密，防止消息被篡改
2. **配置白名单**：`allowed_open_ids` 只填信任的用户，避免任何人都能触发开机
3. **限制回调 IP**：在防火墙规则中限制只接受飞书服务器 IP 段的请求
4. **保护配置文件**：`chmod 600 /etc/feishu-wol/config.yaml`，避免密钥泄露
5. **不暴露管理界面**：OpenWrt LuCI 不要开放到公网
6. **定期更新**：关注飞书 IP 段变更，及时更新防火墙白名单

---

## 命令行参数

```
feishu-wol [选项]

选项：
  -config string    配置文件路径（默认 /etc/feishu-wol/config.yaml）
  -version          打印版本信息并退出
  -test-wol         发送一次 WoL 测试包并退出（不启动 HTTP 服务）
```

---

## 构建

```bash
# 本机构建
make build

# 交叉编译（OpenWrt）
make build-mipsel    # MIPS Little-Endian
make build-mips      # MIPS Big-Endian
make build-arm       # ARM v7
make build-arm64     # AArch64

# 全平台
make build-all
```

---

## License

MIT License © 2024
