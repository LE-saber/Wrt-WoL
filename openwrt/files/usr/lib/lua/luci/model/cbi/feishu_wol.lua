local sys  = require "luci.sys"
local disp = require "luci.dispatcher"

-- ── Map ───────────────────────────────────────────────────────────────────────
local m = Map("feishu-wol",
	translate("Wrt-Wol / 远程开机"),
	translate("通过飞书、Telegram 或自建服务远程唤醒局域网内设备。" ..
	          "发送 <code>/list</code> 查看设备，发送 <code>/on</code> 或 <code>/on 1</code> 执行唤醒。"))

-- ── Status section (template, no UCI backing) ─────────────────────────────────
local ss = m:section(SimpleSection)
ss.template = "feishu_wol/status"

-- ── Single NamedSection with tabs ─────────────────────────────────────────────
local s = m:section(NamedSection, "main", "feishu-wol")
s.addremove = false
s.anonymous = false

s:tab("basic",    translate("基本设置"))
s:tab("feishu",   translate("飞书凭证"))
s:tab("telegram", translate("Telegram"))
s:tab("selfhost", translate("自建服务"))
s:tab("wol",      translate("Wake-on-LAN"))
s:tab("security", translate("安全白名单"))
s:tab("log",      translate("日志"))

-- ── Tab: 基本设置 ──────────────────────────────────────────────────────────────
local o

o = s:taboption("basic", Flag, "enabled",
	translate("启用服务"),
	translate("保存并应用后服务将自动启动或停止"))
o.rmempty = false

-- ── Tab: 飞书凭证 ──────────────────────────────────────────────────────────────
o = s:taboption("feishu", Value, "app_id",
	translate("App ID"),
	translate("飞书开放平台 → 凭证与基础信息 → App ID"))
o.placeholder = "cli_xxxxxxxxxxxxxxxxxx"
o.rmempty = false

o = s:taboption("feishu", Value, "app_secret",
	translate("App Secret"),
	translate("飞书开放平台 → 凭证与基础信息 → App Secret"))
o.placeholder = "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
o.password = true
o.rmempty = false

-- ── Tab: Telegram ────────────────────────────────────────────────────────────
o = s:taboption("telegram", Flag, "telegram_enabled",
	translate("启用 Telegram"),
	translate("通过 Telegram Bot API 长轮询接收 /on 命令。国内网络可能需要代理。"))
o.rmempty = false

o = s:taboption("telegram", Value, "telegram_bot_token",
	translate("Bot Token"),
	translate("从 Telegram 的 BotFather 创建机器人后获取，例如 123456:ABC-DEF。"))
o.placeholder = "123456789:xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
o.password = true
o:depends("telegram_enabled", "1")

o = s:taboption("telegram", DynamicList, "telegram_allowed_user_ids",
	translate("允许的 Telegram User ID"),
	translate("留空则不限制用户。首次运行后向机器人发消息，查看日志中的 user_id。"))
o.placeholder = "123456789"
o:depends("telegram_enabled", "1")

o = s:taboption("telegram", DynamicList, "telegram_allowed_chat_ids",
	translate("允许的 Telegram Chat ID"),
	translate("留空则不限制会话。群组 ID 通常为负数。"))
o.placeholder = "-1001234567890"
o:depends("telegram_enabled", "1")

o = s:taboption("telegram", Value, "telegram_proxy_url",
	translate("代理 URL"),
	translate("可选。示例：http://127.0.0.1:7890 或 http://user:pass@host:port。"))
o.placeholder = "http://127.0.0.1:7890"
o:depends("telegram_enabled", "1")

-- ── Tab: 自建服务 ──────────────────────────────────────────────────────────────
o = s:taboption("selfhost", Flag, "selfhost_client_enabled",
	translate("启用自建服务客户端"),
	translate("OpenWrt 主动轮询你部署的自建服务器，无需飞书或 Telegram。"))
o.rmempty = false

o = s:taboption("selfhost", Value, "selfhost_server_url",
	translate("服务器地址"),
	translate("自建服务器地址，例如 https://wol.example.com。建议通过 HTTPS 反向代理暴露。"))
o.placeholder = "https://wol.example.com"
o:depends("selfhost_client_enabled", "1")

o = s:taboption("selfhost", Value, "selfhost_device_token",
	translate("设备 Token"),
	translate("OpenWrt 与自建服务器共享的设备 Token，必须与服务器端配置一致。"))
o.password = true
o:depends("selfhost_client_enabled", "1")

o = s:taboption("selfhost", Value, "selfhost_poll_interval",
	translate("轮询间隔"),
	translate("OpenWrt 检查远程指令的间隔，例如 3s、5s、10s。"))
o.placeholder = "5s"
o:depends("selfhost_client_enabled", "1")

-- ── Tab: Wake-on-LAN ──────────────────────────────────────────────────────────
o = s:taboption("wol", DynamicList, "mac_addresses",
	translate("兼容 MAC 地址列表"),
	translate("旧版兼容字段。推荐在下方“设备管理”中填写备注、MAC 和可选 IP；若设备管理为空，则使用这里的 MAC 自动生成设备1、设备2。"))
o.placeholder = "AA:BB:CC:DD:EE:FF"
o.rmempty = true

o = s:taboption("wol", Value, "interface",
	translate("发包网口"),
	translate("发送 Magic Packet 的网口，ImmortalWrt 默认为 br-lan"))
o.placeholder = "br-lan"
o:value("br-lan", "br-lan（默认 LAN 桥）")
o:value("eth0",   "eth0")
o:value("eth1",   "eth1")

o = s:taboption("wol", Value, "broadcast_ip",
	translate("广播地址"),
	translate("通常无需修改；定向广播可填写如 192.168.1.255"))
o.placeholder = "255.255.255.255"
o.datatype = "ipaddr"

o = s:taboption("wol", Value, "port",
	translate("UDP 端口"),
	translate("WoL 标准端口为 9，也可使用 7"))
o.datatype = "port"
o.placeholder = "9"

-- ── Tab: 安全白名单 ────────────────────────────────────────────────────────────
o = s:taboption("security", DynamicList, "allowed_open_ids",
	translate("允许的 Open ID"),
	translate("填写允许发送 /on 命令的飞书用户 open_id（ou_ 前缀）。" ..
	          "留空则不限制，所有人均可触发。" ..
	          "首次运行后查看日志（logread | grep feishu-wol）可获取自己的 open_id。"))
o.placeholder = "ou_xxxxxxxxxxxxxxxxxxxxxxxx"

o = s:taboption("security", DynamicList, "allowed_chat_ids",
	translate("允许的群聊 Chat ID"),
	translate("填写允许触发 /on 命令的飞书群聊 chat_id（oc_ 前缀）。" ..
	          "私聊机器人或不限制群聊时可留空。"))
o.placeholder = "oc_xxxxxxxxxxxxxxxxxxxxxxxx"

-- ── Tab: 日志 ─────────────────────────────────────────────────────────────────
o = s:taboption("log", ListValue, "log_level", translate("日志级别"))
o:value("debug", "Debug（详细）")
o:value("info",  "Info（默认）")
o:value("warn",  "Warn")
o:value("error", "Error")

o = s:taboption("log", Value, "log_file",
	translate("日志文件"),
	translate("日志同时输出到 syslog（logread）。填写路径可额外保存到文件，留空则仅 syslog。"))
o.placeholder = "/tmp/feishu-wol.log"

-- ── Device management ────────────────────────────────────────────────────────
local ds = m:section(TypedSection, "device",
	translate("设备管理"),
	translate("为每台可唤醒设备配置备注和 MAC。聊天命令支持 /list、/on 1、/on 备注、/on all。"))
ds.addremove = true
ds.anonymous = true
ds.template = "cbi/tblsection"

o = ds:option(Value, "name", translate("备注"))
o.placeholder = "设备1"
o.rmempty = true

o = ds:option(Value, "mac", translate("MAC 地址"))
o.placeholder = "AA:BB:CC:DD:EE:FF"
o.datatype = "macaddr"
o.rmempty = false

o = ds:option(Value, "ip", translate("IP 地址（可选）"))
o.placeholder = "192.168.1.100"
o.datatype = "ipaddr"
o.rmempty = true

o = ds:option(Value, "interface", translate("网口（可选）"))
o.placeholder = "br-lan"
o:value("", translate("继承全局"))
o:value("br-lan", "br-lan")
o:value("eth0", "eth0")
o:value("eth1", "eth1")
o.rmempty = true

o = ds:option(Value, "broadcast_ip", translate("广播地址（可选）"))
o.placeholder = "255.255.255.255"
o.datatype = "ipaddr"
o.rmempty = true

o = ds:option(Value, "port", translate("端口（可选）"))
o.placeholder = "9"
o.datatype = "port"
o.rmempty = true

return m
