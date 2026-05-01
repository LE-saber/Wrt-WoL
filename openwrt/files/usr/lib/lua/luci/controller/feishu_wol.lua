module("luci.controller.feishu_wol", package.seeall)

function index()
	entry({"admin", "services", "feishu-wol"},
	      cbi("feishu_wol"),
	      _("Wrt-Wol"), 60)

	-- AJAX / redirect endpoints for service control
	entry({"admin", "services", "feishu-wol", "status"},
	      call("act_status")).leaf = true
	entry({"admin", "services", "feishu-wol", "start"},
	      call("act_start")).leaf = true
	entry({"admin", "services", "feishu-wol", "stop"},
	      call("act_stop")).leaf = true
	entry({"admin", "services", "feishu-wol", "restart"},
	      call("act_restart")).leaf = true
end

-- JSON status for polling
function act_status()
	local sys = require "luci.sys"
	local running = (sys.call("/etc/init.d/feishu-wol running >/dev/null 2>&1") == 0)
	luci.http.prepare_content("application/json")
	luci.http.write_json({ running = running })
end

function act_start()
	luci.sys.call("/etc/init.d/feishu-wol start >/dev/null 2>&1")
	luci.http.redirect(luci.dispatcher.build_url("admin/services/feishu-wol"))
end

function act_stop()
	luci.sys.call("/etc/init.d/feishu-wol stop >/dev/null 2>&1")
	luci.http.redirect(luci.dispatcher.build_url("admin/services/feishu-wol"))
end

function act_restart()
	luci.sys.call("/etc/init.d/feishu-wol restart >/dev/null 2>&1")
	luci.http.redirect(luci.dispatcher.build_url("admin/services/feishu-wol"))
end
