package commands

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LE-saber/Wrt-Wol/internal/config"
)

type WakeFunc func([]config.DeviceConfig) (string, error)

type Source struct {
	Platform string
	UserID   string
	ChatID   string
}

type Handler struct {
	cfg    *config.Config
	logger *slog.Logger
	wake   WakeFunc

	mu       sync.Mutex
	lastWake string
}

func New(cfg *config.Config, logger *slog.Logger, wake WakeFunc) *Handler {
	return &Handler{cfg: cfg, logger: logger, wake: wake}
}

func (handler *Handler) Handle(text string, source Source) string {
	args := strings.Fields(strings.TrimSpace(text))
	if len(args) == 0 {
		return handler.help()
	}

	command := normalizeCommand(args[0])
	switch strings.ToLower(command) {
	case "/on":
		return handler.wakeDevices(args[1:], source)
	case "/list":
		return handler.listDevices()
	case "/help":
		return handler.help()
	case "/whoami":
		return handler.whoami(source)
	case "/last":
		return handler.last()
	default:
		return fmt.Sprintf("未知命令 %q，发送 /help 查看可用命令。", command)
	}
}

func (handler *Handler) listDevices() string {
	devices := handler.cfg.WoL.Devices
	if len(devices) == 0 {
		return "暂无可用设备。"
	}

	var builder strings.Builder
	builder.WriteString("可用设备：\n")
	for i, device := range devices {
		builder.WriteString(fmt.Sprintf("%d. %s — %s", i+1, device.Name, device.MAC))
		if device.IP != "" {
			builder.WriteString(fmt.Sprintf("（IP: %s）", device.IP))
		}
		builder.WriteString("\n")
	}
	builder.WriteString("\n用法：/on 1、/on 设备名、/on all")
	return strings.TrimSpace(builder.String())
}

func (handler *Handler) wakeDevices(targets []string, source Source) string {
	devices, label, err := handler.resolveTargets(targets)
	if err != nil {
		return "❌ " + err.Error()
	}
	if len(devices) == 0 {
		return "❌ 没有匹配到可唤醒设备。发送 /list 查看设备列表。"
	}

	result, err := handler.wake(devices)
	if err != nil {
		return fmt.Sprintf("❌ 发送开机指令失败：%v", err)
	}

	handler.recordLast(source, label)
	return result
}

func (handler *Handler) resolveTargets(targets []string) ([]config.DeviceConfig, string, error) {
	devices := handler.cfg.WoL.Devices
	if len(devices) == 0 {
		return nil, "", fmt.Errorf("暂无可用设备。")
	}

	if len(targets) == 0 || (len(targets) == 1 && strings.EqualFold(targets[0], "all")) {
		return devices, "全部设备", nil
	}

	var selected []config.DeviceConfig
	var labels []string
	seen := map[int]bool{}
	for _, target := range targets {
		device, index, err := matchDevice(devices, target)
		if err != nil {
			return nil, "", err
		}
		if seen[index] {
			continue
		}
		seen[index] = true
		selected = append(selected, device)
		labels = append(labels, fmt.Sprintf("%d:%s", index+1, device.Name))
	}

	return selected, strings.Join(labels, ", "), nil
}

func matchDevice(devices []config.DeviceConfig, target string) (config.DeviceConfig, int, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return config.DeviceConfig{}, -1, fmt.Errorf("设备参数为空。")
	}

	if index, err := strconv.Atoi(target); err == nil {
		if index < 1 || index > len(devices) {
			return config.DeviceConfig{}, -1, fmt.Errorf("设备编号 %d 不存在。发送 /list 查看可用编号。", index)
		}
		return devices[index-1], index - 1, nil
	}

	for i, device := range devices {
		if strings.EqualFold(device.Name, target) || strings.EqualFold(device.MAC, target) {
			return device, i, nil
		}
	}

	var matches []int
	for i, device := range devices {
		if strings.Contains(strings.ToLower(device.Name), strings.ToLower(target)) {
			matches = append(matches, i)
		}
	}
	if len(matches) == 1 {
		index := matches[0]
		return devices[index], index, nil
	}
	if len(matches) > 1 {
		var names []string
		for _, index := range matches {
			names = append(names, fmt.Sprintf("%d:%s", index+1, devices[index].Name))
		}
		return config.DeviceConfig{}, -1, fmt.Errorf("设备名 %q 匹配多台：%s。请使用编号。", target, strings.Join(names, ", "))
	}

	return config.DeviceConfig{}, -1, fmt.Errorf("未找到设备 %q。发送 /list 查看设备列表。", target)
}

func (handler *Handler) help() string {
	return strings.Join([]string{
		"可用命令：",
		"/list — 查看可唤醒设备",
		"/on — 唤醒全部设备",
		"/on 1 — 唤醒编号为 1 的设备",
		"/on 设备名 — 按备注唤醒设备",
		"/on all — 唤醒全部设备",
		"/last — 查看最近一次唤醒记录",
		"/whoami — 查看当前用户/会话 ID",
	}, "\n")
}

func (handler *Handler) whoami(source Source) string {
	lines := []string{"当前会话："}
	if source.Platform != "" {
		lines = append(lines, "平台："+source.Platform)
	}
	if source.UserID != "" {
		lines = append(lines, "用户 ID："+source.UserID)
	}
	if source.ChatID != "" {
		lines = append(lines, "会话 ID："+source.ChatID)
	}
	return strings.Join(lines, "\n")
}

func (handler *Handler) last() string {
	handler.mu.Lock()
	defer handler.mu.Unlock()
	if handler.lastWake == "" {
		return "暂无唤醒记录。"
	}
	return handler.lastWake
}

func (handler *Handler) recordLast(source Source, label string) {
	handler.mu.Lock()
	defer handler.mu.Unlock()
	actor := source.Platform
	if source.UserID != "" {
		actor += ":" + source.UserID
	}
	handler.lastWake = fmt.Sprintf("最近唤醒：%s 由 %s 触发，目标：%s",
		time.Now().Format("2006-01-02 15:04:05"), actor, label)
	handler.logger.Info("wake command completed", "actor", actor, "target", label)
}

func normalizeCommand(command string) string {
	if at := strings.Index(command, "@"); at != -1 {
		command = command[:at]
	}
	return strings.TrimSpace(command)
}
