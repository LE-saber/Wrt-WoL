# feishu-wol 部署文档

> 通过飞书机器人长连接远程唤醒局域网 PC 的 OpenWrt/ImmortalWrt 服务。

---

## 目录

1. [工作原理](#工作原理)
2. [前置条件](#前置条件)
3. [飞书机器人配置](#飞书机器人配置)
4. [编译](#编译)
5. [部署到 ImmortalWrt](#部署到-immortalwrt)
6. [配置文件详解](#配置文件详解)
7. [服务管理](#服务管理)
8. [测试验证](#测试验证)
9. [故障排查](#故障排查)

---

## 工作原理

```
手机飞书 App
    │  发送 /on
    ▼
飞书服务器
    │  WebSocket 长连接推送事件
    ▼
feishu-wol（运行在 ImmortalWrt 路由器上）
    │  构造 102 字节 Magic Packet
    ▼
UDP 广播 → br-lan → PC 网卡
    │
    ▼
PC 开机 ✅
```

**长连接模式的优势：**

| 对比项 | HTTP Webhook | WebSocket 长连接（本项目） |
|--------|-------------|--------------------------|
| 需要公网 IP | ✅ 需要 | ❌ 不需要 |
| 需要端口转发 | ✅ 需要 | ❌ 不需要 |
| 签名/加密配置 | 复杂 | SDK 自动处理 |
| 连接方向 | 飞书 → 路由器（入站） | 路由器 → 飞书（出站） |
| 重连机制 | 需自行实现 | SDK 自动重连 |

路由器只需能访问互联网，飞书服务器主动将消息推送到已建立的 WebSocket 连接上。

---

## 前置条件

### 1. PC 端：开启 Wake-on-LAN

**BIOS/UEFI 设置：**

进入 BIOS → 电源管理（Power Management）→ 找到 `Wake on LAN` / `Power On By PCI-E` → 开启。

**操作系统（Linux）验证：**

```bash
# 查看网卡 WoL 状态
ethtool eth0 | grep "Wake-on"
# Wake-on: g   ← 已开启（g = Magic Packet）
# Wake-on: d   ← 未开启，执行下面命令开启：

sudo ethtool -s eth0 wol g

# 让设置重启后持久化（写入 /etc/network/interfaces 或 systemd）
```

**查看 PC 的 MAC 地址：**

```bash
ip link show eth0 | grep "link/ether"
# link/ether aa:bb:cc:dd:ee:ff brd ff:ff:ff:ff:ff:ff
```

将 `aa:bb:cc:dd:ee:ff` 填入 `config.yaml` 的 `wol.mac_addresses`。

### 2. 路由器端

- ImmortalWrt 24.10.4 x86（或其他版本）
- 能正常访问互联网（飞书 WebSocket 出站）
- SSH 访问权限

---

## 飞书机器人配置

### Step 1：创建自建应用

1. 访问 [飞书开放平台](https://open.feishu.cn/app) → **创建企业自建应用**
2. 填写应用名称（如"远程唤醒"），上传图标

### Step 2：开启机器人能力

应用详情页 → **功能** → **机器人** → 开启

### Step 3：添加权限

**权限管理** → 搜索并添加：

- `im:message`（读取用户发给机器人的消息）
- `im:message:send_as_bot`（机器人发送消息）

### Step 4：开启长连接

**事件订阅** → 选择 **"使用长连接接收事件"**（无需填写回调 URL）→ 添加事件：

- `im.message.receive_v1`（接收消息事件）

> ⚠️ 确保选择的是**长连接**模式，不是 HTTP 回调模式。长连接模式下不需要配置 Encrypt Key 和 Verification Token。

### Step 5：获取凭证

应用凭证与基本信息页面：

| 字段 | 填入 config.yaml |
|------|-----------------|
| App ID | `feishu.app_id` |
| App Secret | `feishu.app_secret` |

### Step 6：发布应用

**版本管理与发布** → 创建版本 → 填写更新说明 → **申请发布**（企业内部应用通常即时生效）

### Step 7：获取自己的 Open ID（用于白名单）

在飞书中向刚创建的机器人发送任意消息，然后查看路由器日志：

```bash
tail -f /var/log/feishu-wol.log
# 日志会打印：open_id=ou_xxxxxxxxxxxxxxxxxxxxxxxx
```

将该 `open_id` 填入 `config.yaml` 的 `security.allowed_open_ids`。

---

## 编译

本项目在开发机（Ubuntu 22.04 x86_64）上编译，产出静态二进制直接部署到路由器。

### 安装 Go（仅首次）

```bash
# 下载并安装 Go 1.26
wget https://go.dev/dl/go1.26.2.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.26.2.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

### 拉取依赖

```bash
cd feishu-wol
GOPROXY=https://goproxy.cn,direct go mod tidy
```

### 编译

由于开发机和 ImmortalWrt x86 均为 amd64，直接本机编译：

```bash
make build
# 输出：dist/feishu-wol（约 7MB 静态二进制）
```

其他架构（如需）：

```bash
make build-mipsel   # MIPS LE
make build-arm      # ARM v7
make build-arm64    # AArch64
```

---

## 部署到 ImmortalWrt

### 一键部署脚本

```bash
ROUTER=192.168.1.1   # 替换为你的路由器 IP

# 1. 上传二进制
scp dist/feishu-wol root@$ROUTER:/usr/bin/feishu-wol

# 2. 上传 init.d 服务脚本
scp openwrt/files/etc/init.d/feishu-wol root@$ROUTER:/etc/init.d/feishu-wol

# 3. 创建配置目录并上传配置模板
ssh root@$ROUTER "mkdir -p /etc/feishu-wol"
scp config.yaml.example root@$ROUTER:/etc/feishu-wol/config.yaml
```

### 在路由器上设置权限并编辑配置

```bash
ssh root@$ROUTER

chmod +x /usr/bin/feishu-wol
chmod +x /etc/init.d/feishu-wol
chmod 600 /etc/feishu-wol/config.yaml

# 填写飞书凭证和 PC MAC 地址
vi /etc/feishu-wol/config.yaml
```

---

## 配置文件详解

文件路径：`/etc/feishu-wol/config.yaml`

```yaml
feishu:
  app_id: "cli_xxxxxxxxxxxxxxxx"      # 飞书 App ID
  app_secret: "xxxxxxxxxxxxxxxx"      # 飞书 App Secret

wol:
  mac_addresses:
    - "AA:BB:CC:DD:EE:FF"             # 目标 PC 的 MAC 地址（可填多台）
  interface: "br-lan"                 # 发包网口，ImmortalWrt 默认 LAN 桥为 br-lan
  port: 9                             # WoL UDP 端口（默认 9）
  broadcast_ip: "255.255.255.255"     # 广播地址，通常不需要修改

security:
  allowed_open_ids:                   # 允许触发的用户 open_id（留空不限制）
    - "ou_xxxxxxxxxxxxxxxxxxxxxxxx"
  allowed_chat_ids: []                # 允许触发的群聊 chat_id（留空不限制）

log:
  level: "info"                       # debug / info / warn / error
  file: "/var/log/feishu-wol.log"     # 日志文件（留空只输出到 stderr）
```

**安全建议：**

```bash
chmod 600 /etc/feishu-wol/config.yaml   # 防止其他用户读取 App Secret
```

---

## 服务管理

```bash
# 设置开机自启并立即启动
/etc/init.d/feishu-wol enable
/etc/init.d/feishu-wol start

# 其他操作
/etc/init.d/feishu-wol stop
/etc/init.d/feishu-wol restart
/etc/init.d/feishu-wol status

# 实时查看日志
tail -f /var/log/feishu-wol.log

# 系统日志过滤
logread | grep feishu-wol
```

正常启动后日志示例：

```
time=2026-04-30T17:00:00Z level=INFO msg="feishu-wol starting" version=1.0.0
time=2026-04-30T17:00:00Z level=INFO msg="connecting to Feishu via WebSocket long connection"
```

收到 `/on` 命令后：

```
time=2026-04-30T17:01:00Z level=INFO msg="message received" open_id=ou_xxx chat_id=oc_xxx msg_type=text
time=2026-04-30T17:01:00Z level=INFO msg="command" cmd=/on open_id=ou_xxx
time=2026-04-30T17:01:00Z level=INFO msg="sending WoL packet" macs=[AA:BB:CC:DD:EE:FF] interface=br-lan
time=2026-04-30T17:01:00Z level=INFO msg="WoL sent successfully"
```

---

## 测试验证

### 1. 测试 WoL 发包（不启动机器人）

```bash
feishu-wol -config /etc/feishu-wol/config.yaml -test-wol
# 看到 "WoL packet sent" 且 PC 开机 → WoL 配置正确
```

### 2. 测试飞书长连接

```bash
/etc/init.d/feishu-wol start
tail -f /var/log/feishu-wol.log
# 看到 "connecting to Feishu via WebSocket long connection" 后，
# 在飞书中给机器人发 /on，日志应出现完整的处理链
```

### 3. 飞书端完整流程

1. 打开飞书，找到刚创建的机器人
2. 发送 `/on`
3. 机器人回复 `✅ 已向 [AA:BB:CC:DD:EE:FF] 发送 Wake-on-LAN 开机指令！`
4. PC 启动

发送 `/help` 查看可用命令列表。

---

## 故障排查

| 现象 | 可能原因 | 解决方法 |
|------|----------|----------|
| 日志停在 "connecting..." 无后续 | 路由器无法访问飞书服务器 | `curl https://open.feishu.cn` 测试网络 |
| 日志报 `app_id or app_secret error` | 凭证填写错误 | 重新核对飞书后台的 App ID / App Secret |
| 日志报 `access denied` | 发送者不在白名单 | 从日志取 `open_id` 填入 `allowed_open_ids` |
| WoL 发包成功但 PC 不开机 | BIOS 未开启 WoL | 进 BIOS 开启；检查 `ethtool eth0` 显示 `Wake-on: g` |
| WoL 发包失败 `interface not found` | 网口名称错误 | 路由器执行 `ip link show` 确认 LAN 桥名 |
| 机器人无响应（飞书端） | 权限未申请或应用未发布 | 检查 `im:message` 权限；确认应用已发布 |
| 服务启动后立即退出 | 配置文件解析失败 | `feishu-wol -config ... -version` 验证配置路径 |

### 开启调试日志

修改 `config.yaml`：

```yaml
log:
  level: "debug"
```

重启服务后可看到 SDK 的详细连接和事件日志。

---

## 项目结构

```
feishu-wol/
├── cmd/feishu-wol/main.go              # 程序入口
├── internal/
│   ├── config/config.go                # 配置加载
│   ├── feishu/bot.go                   # 飞书 WebSocket 长连接机器人
│   └── wol/wol.go                      # WoL Magic Packet 构造与发送
├── openwrt/
│   ├── Makefile                        # OpenWrt .ipk 打包
│   └── files/
│       ├── etc/init.d/feishu-wol       # procd 服务脚本
│       └── etc/feishu-wol/config.yaml  # 默认配置模板
├── config.yaml.example                 # 配置说明文件
├── Makefile                            # 编译（含交叉编译）
├── Dockerfile                          # Docker 镜像（host 网络）
├── docker-compose.yml
├── DEPLOYMENT.md                       # 本文档
└── README.md
```

---

## 依赖

| 依赖 | 版本 | 用途 |
|------|------|------|
| `github.com/larksuite/oapi-sdk-go/v3` | v3.4.10 | 飞书官方 Go SDK（长连接、消息 API） |
| `gopkg.in/yaml.v3` | v3.0.1 | YAML 配置解析 |

编译产物为**静态二进制**，部署到路由器无需任何运行时依赖。
