package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration structure.
type Config struct {
	Feishu   FeishuConfig   `yaml:"feishu"`
	Telegram TelegramConfig `yaml:"telegram"`
	SelfHost SelfHostConfig `yaml:"selfhost"`
	WoL      WoLConfig      `yaml:"wol"`
	Security SecurityConfig `yaml:"security"`
	Log      LogConfig      `yaml:"log"`
}

// FeishuConfig holds Feishu open-platform credentials.
// Long-connection mode only needs AppID and AppSecret.
type FeishuConfig struct {
	Enabled   bool   `yaml:"enabled"`
	AppID     string `yaml:"app_id"`
	AppSecret string `yaml:"app_secret"`
}

type TelegramConfig struct {
	Enabled        bool     `yaml:"enabled"`
	BotToken       string   `yaml:"bot_token"`
	AllowedUserIDs []string `yaml:"allowed_user_ids"`
	AllowedChatIDs []string `yaml:"allowed_chat_ids"`
	ProxyURL       string   `yaml:"proxy_url"`
	DropPending    bool     `yaml:"drop_pending_updates"`
}

type SelfHostConfig struct {
	Server SelfHostServerConfig `yaml:"server"`
	Client SelfHostClientConfig `yaml:"client"`
}

type SelfHostServerConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Host        string `yaml:"host"`
	Port        int    `yaml:"port"`
	AdminToken  string `yaml:"admin_token"`
	DeviceToken string `yaml:"device_token"`
}

type SelfHostClientConfig struct {
	Enabled      bool   `yaml:"enabled"`
	ServerURL    string `yaml:"server_url"`
	DeviceToken  string `yaml:"device_token"`
	PollInterval string `yaml:"poll_interval"`
}

// WoLConfig holds Wake-on-LAN target settings.
type WoLConfig struct {
	MACAddresses []string       `yaml:"mac_addresses"`
	Devices      []DeviceConfig `yaml:"devices"`
	Interface    string         `yaml:"interface"`
	Port         int            `yaml:"port"`
	BroadcastIP  string         `yaml:"broadcast_ip"`
}

type DeviceConfig struct {
	Name        string `yaml:"name"`
	MAC         string `yaml:"mac"`
	IP          string `yaml:"ip"`
	Interface   string `yaml:"interface"`
	BroadcastIP string `yaml:"broadcast_ip"`
	Port        int    `yaml:"port"`
}

// SecurityConfig controls who may trigger WoL.
type SecurityConfig struct {
	AllowedOpenIDs []string `yaml:"allowed_open_ids"`
	AllowedChatIDs []string `yaml:"allowed_chat_ids"`
}

// LogConfig controls logging behaviour.
type LogConfig struct {
	Level string `yaml:"level"`
	File  string `yaml:"file"`
}

// Load reads a YAML config file and applies environment variable overrides.
// If the file does not exist, only environment variables are used (OpenWrt UCI mode).
// Environment variables always win over YAML values.
//
// OpenWrt init.d injects UCI values as env vars; YAML is the fallback for
// Docker / standalone deployments.
func Load(path string) (*Config, error) {
	var cfg Config

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}

	applyDefaults(&cfg)
	applyEnv(&cfg)
	normalizeDevices(&cfg)
	return &cfg, validate(&cfg)
}

// applyEnv overrides config fields from environment variables.
// Variable names follow FEISHU_* convention, set by the OpenWrt init.d script.
//
//	FEISHU_APP_ID            – feishu.app_id
//	FEISHU_APP_SECRET        – feishu.app_secret
//	FEISHU_WOL_MACS          – comma-separated MAC addresses
//	FEISHU_WOL_INTERFACE     – wol.interface
//	FEISHU_WOL_BROADCAST     – wol.broadcast_ip
//	FEISHU_WOL_PORT          – wol.port (integer)
//	FEISHU_WOL_DEVICES       – semicolon-separated name|mac|ip|interface|broadcast|port records
//	FEISHU_LOG_LEVEL         – log.level
//	FEISHU_LOG_FILE          – log.file
//	FEISHU_ALLOWED_OPEN_IDS  – comma-separated open_id list ("" = no restriction)
//	FEISHU_ALLOWED_CHAT_IDS  – comma-separated chat_id list ("" = no restriction)
//	FEISHU_TELEGRAM_ENABLED  – enable Telegram long polling
//	FEISHU_TELEGRAM_BOT_TOKEN – Telegram Bot API token
//	FEISHU_TELEGRAM_ALLOWED_USER_IDS – comma-separated Telegram user IDs
//	FEISHU_TELEGRAM_ALLOWED_CHAT_IDS – comma-separated Telegram chat IDs
//	FEISHU_TELEGRAM_PROXY_URL – optional HTTP(S)/SOCKS proxy URL
//	FEISHU_SELFHOST_CLIENT_ENABLED – enable self-host relay client
//	FEISHU_SELFHOST_SERVER_URL – self-host relay server base URL
//	FEISHU_SELFHOST_DEVICE_TOKEN – shared device token for relay client/server
//	FEISHU_SELFHOST_POLL_INTERVAL – relay polling interval, e.g. "5s"
//	FEISHU_SELFHOST_SERVER_ENABLED – enable built-in self-host relay server
//	FEISHU_SELFHOST_SERVER_HOST – relay server listen host
//	FEISHU_SELFHOST_SERVER_PORT – relay server listen port
//	FEISHU_SELFHOST_ADMIN_TOKEN – admin token for web/API wake requests
func applyEnv(cfg *Config) {
	envBool("FEISHU_ENABLED", &cfg.Feishu.Enabled)
	if v := os.Getenv("FEISHU_APP_ID"); v != "" {
		cfg.Feishu.AppID = v
	}
	if v := os.Getenv("FEISHU_APP_SECRET"); v != "" {
		cfg.Feishu.AppSecret = v
	}
	if v := os.Getenv("FEISHU_WOL_MACS"); v != "" {
		cfg.WoL.MACAddresses = splitCSV(v)
	}
	if v := os.Getenv("FEISHU_WOL_INTERFACE"); v != "" {
		cfg.WoL.Interface = v
	}
	if v := os.Getenv("FEISHU_WOL_BROADCAST"); v != "" {
		cfg.WoL.BroadcastIP = v
	}
	if v := os.Getenv("FEISHU_WOL_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			cfg.WoL.Port = p
		}
	}
	if v := os.Getenv("FEISHU_WOL_DEVICES"); v != "" {
		cfg.WoL.Devices = parseDevices(v)
	}
	if v := os.Getenv("FEISHU_LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}
	if v := os.Getenv("FEISHU_LOG_FILE"); v != "" {
		cfg.Log.File = v
	}
	if v := os.Getenv("FEISHU_ALLOWED_OPEN_IDS"); v != "" {
		cfg.Security.AllowedOpenIDs = splitCSV(v)
	}
	if v := os.Getenv("FEISHU_ALLOWED_CHAT_IDS"); v != "" {
		cfg.Security.AllowedChatIDs = splitCSV(v)
	}
	envBool("FEISHU_TELEGRAM_ENABLED", &cfg.Telegram.Enabled)
	if v := os.Getenv("FEISHU_TELEGRAM_BOT_TOKEN"); v != "" {
		cfg.Telegram.BotToken = v
	}
	if v := os.Getenv("FEISHU_TELEGRAM_ALLOWED_USER_IDS"); v != "" {
		cfg.Telegram.AllowedUserIDs = splitCSV(v)
	}
	if v := os.Getenv("FEISHU_TELEGRAM_ALLOWED_CHAT_IDS"); v != "" {
		cfg.Telegram.AllowedChatIDs = splitCSV(v)
	}
	if v := os.Getenv("FEISHU_TELEGRAM_PROXY_URL"); v != "" {
		cfg.Telegram.ProxyURL = v
	}
	envBool("FEISHU_TELEGRAM_DROP_PENDING", &cfg.Telegram.DropPending)
	envBool("FEISHU_SELFHOST_CLIENT_ENABLED", &cfg.SelfHost.Client.Enabled)
	if v := os.Getenv("FEISHU_SELFHOST_SERVER_URL"); v != "" {
		cfg.SelfHost.Client.ServerURL = v
	}
	if v := os.Getenv("FEISHU_SELFHOST_DEVICE_TOKEN"); v != "" {
		cfg.SelfHost.Client.DeviceToken = v
		cfg.SelfHost.Server.DeviceToken = v
	}
	if v := os.Getenv("FEISHU_SELFHOST_POLL_INTERVAL"); v != "" {
		cfg.SelfHost.Client.PollInterval = v
	}
	envBool("FEISHU_SELFHOST_SERVER_ENABLED", &cfg.SelfHost.Server.Enabled)
	if v := os.Getenv("FEISHU_SELFHOST_SERVER_HOST"); v != "" {
		cfg.SelfHost.Server.Host = v
	}
	if v := os.Getenv("FEISHU_SELFHOST_SERVER_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			cfg.SelfHost.Server.Port = p
		}
	}
	if v := os.Getenv("FEISHU_SELFHOST_ADMIN_TOKEN"); v != "" {
		cfg.SelfHost.Server.AdminToken = v
	}
}

func parseDevices(value string) []DeviceConfig {
	var devices []DeviceConfig
	for _, record := range strings.Split(value, ";") {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}
		fields := strings.Split(record, "|")
		device := DeviceConfig{}
		if len(fields) > 0 {
			device.Name = strings.TrimSpace(fields[0])
		}
		if len(fields) > 1 {
			device.MAC = strings.TrimSpace(fields[1])
		}
		if len(fields) > 2 {
			device.IP = strings.TrimSpace(fields[2])
		}
		if len(fields) > 3 {
			device.Interface = strings.TrimSpace(fields[3])
		}
		if len(fields) > 4 {
			device.BroadcastIP = strings.TrimSpace(fields[4])
		}
		if len(fields) > 5 {
			if p, err := strconv.Atoi(strings.TrimSpace(fields[5])); err == nil && p > 0 {
				device.Port = p
			}
		}
		if device.MAC != "" {
			devices = append(devices, device)
		}
	}
	return devices
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func envBool(name string, dst *bool) {
	v := os.Getenv(name)
	if v == "" {
		return
	}
	if b, err := strconv.ParseBool(v); err == nil {
		*dst = b
	}
}

func applyDefaults(cfg *Config) {
	if cfg.WoL.Port == 0 {
		cfg.WoL.Port = 9
	}
	if cfg.WoL.Interface == "" {
		cfg.WoL.Interface = "br-lan"
	}
	if cfg.WoL.BroadcastIP == "" {
		cfg.WoL.BroadcastIP = "255.255.255.255"
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	cfg.Telegram.DropPending = true
	if cfg.SelfHost.Server.Host == "" {
		cfg.SelfHost.Server.Host = "0.0.0.0"
	}
	if cfg.SelfHost.Server.Port == 0 {
		cfg.SelfHost.Server.Port = 8080
	}
	if cfg.SelfHost.Client.PollInterval == "" {
		cfg.SelfHost.Client.PollInterval = "5s"
	}
}

func normalizeDevices(cfg *Config) {
	if len(cfg.WoL.Devices) == 0 {
		for i, mac := range cfg.WoL.MACAddresses {
			mac = strings.TrimSpace(mac)
			if mac == "" {
				continue
			}
			cfg.WoL.Devices = append(cfg.WoL.Devices, DeviceConfig{
				Name: fmt.Sprintf("设备%d", i+1),
				MAC:  mac,
			})
		}
	}

	cfg.WoL.MACAddresses = cfg.WoL.MACAddresses[:0]
	for i := range cfg.WoL.Devices {
		if strings.TrimSpace(cfg.WoL.Devices[i].Name) == "" {
			cfg.WoL.Devices[i].Name = fmt.Sprintf("设备%d", i+1)
		}
		if cfg.WoL.Devices[i].Interface == "" {
			cfg.WoL.Devices[i].Interface = cfg.WoL.Interface
		}
		if cfg.WoL.Devices[i].BroadcastIP == "" {
			cfg.WoL.Devices[i].BroadcastIP = cfg.WoL.BroadcastIP
		}
		if cfg.WoL.Devices[i].Port == 0 {
			cfg.WoL.Devices[i].Port = cfg.WoL.Port
		}
		if cfg.WoL.Devices[i].MAC != "" {
			cfg.WoL.MACAddresses = append(cfg.WoL.MACAddresses, cfg.WoL.Devices[i].MAC)
		}
	}
}

func validate(cfg *Config) error {
	if cfg.Feishu.Active() && cfg.Feishu.AppID == "" {
		return fmt.Errorf("feishu app_id is required (set feishu.app_id in config or FEISHU_APP_ID env)")
	}
	if cfg.Feishu.Active() && cfg.Feishu.AppSecret == "" {
		return fmt.Errorf("feishu app_secret is required (set feishu.app_secret in config or FEISHU_APP_SECRET env)")
	}
	if cfg.Telegram.Active() && cfg.Telegram.BotToken == "" {
		return fmt.Errorf("telegram bot_token is required (set telegram.bot_token or FEISHU_TELEGRAM_BOT_TOKEN env)")
	}
	if cfg.SelfHost.Client.Active() && cfg.SelfHost.Client.ServerURL == "" {
		return fmt.Errorf("selfhost.client.server_url is required")
	}
	if cfg.SelfHost.Client.Active() && cfg.SelfHost.Client.DeviceToken == "" {
		return fmt.Errorf("selfhost.client.device_token is required")
	}
	if cfg.SelfHost.Server.Active() && cfg.SelfHost.Server.AdminToken == "" {
		return fmt.Errorf("selfhost.server.admin_token is required")
	}
	if cfg.SelfHost.Server.Active() && cfg.SelfHost.Server.DeviceToken == "" {
		return fmt.Errorf("selfhost.server.device_token is required")
	}
	if cfg.NeedsWoL() && len(cfg.WoL.Devices) == 0 {
		return fmt.Errorf("at least one device is required (wol.devices, wol.mac_addresses, FEISHU_WOL_DEVICES, or FEISHU_WOL_MACS env)")
	}
	return nil
}

func (c FeishuConfig) Active() bool {
	return c.Enabled || c.AppID != "" || c.AppSecret != ""
}

func (c TelegramConfig) Active() bool {
	return c.Enabled || c.BotToken != ""
}

func (c SelfHostServerConfig) Active() bool {
	return c.Enabled
}

func (c SelfHostClientConfig) Active() bool {
	return c.Enabled || c.ServerURL != ""
}

func (c *Config) NeedsWoL() bool {
	return c.Feishu.Active() || c.Telegram.Active() || c.SelfHost.Client.Active()
}

func (c *Config) HasRuntime() bool {
	return c.Feishu.Active() || c.Telegram.Active() || c.SelfHost.Client.Active() || c.SelfHost.Server.Active()
}
