package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/LE-saber/Wrt-Wol/internal/commands"
	"github.com/LE-saber/Wrt-Wol/internal/config"
	"github.com/LE-saber/Wrt-Wol/internal/feishu"
	"github.com/LE-saber/Wrt-Wol/internal/selfhost"
	"github.com/LE-saber/Wrt-Wol/internal/telegram"
	"github.com/LE-saber/Wrt-Wol/internal/wol"
)

var version = "dev" // overridden by -ldflags at build time

func main() {
	var (
		cfgPath = flag.String("config", "/etc/feishu-wol/config.yaml", "path to config file")
		showVer = flag.Bool("version", false, "print version and exit")
		testWoL = flag.Bool("test-wol", false, "send a test WoL packet and exit (no bot)")
	)
	flag.Parse()

	if *showVer {
		fmt.Println("feishu-wol", version)
		return
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	logger := buildLogger(cfg.Log)
	logger.Info("feishu-wol starting", "version", version)

	// --test-wol: fire one packet and exit, useful for verifying WoL setup.
	if *testWoL {
		if err := sendWoLDevices(cfg.WoL.Devices, logger); err != nil {
			logger.Error("test WoL failed", "err", err)
			os.Exit(1)
		}
		logger.Info("WoL packet sent — check if the target machine powers on")
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	wake := func(devices []config.DeviceConfig) (string, error) {
		if err := sendWoLDevices(devices, logger); err != nil {
			return "", err
		}
		return fmt.Sprintf("✅ 已向 %s 发送 Wake-on-LAN 开机指令！", deviceNames(devices)), nil
	}
	commandHandler := commands.New(cfg, logger, wake)

	runners, err := buildRunners(cfg, logger, commandHandler)
	if err != nil {
		logger.Error("build runners failed", "err", err)
		os.Exit(1)
	}
	if len(runners) == 0 {
		logger.Error("no runtime enabled; configure Feishu, Telegram, selfhost client, or selfhost server")
		os.Exit(1)
	}

	errCh := make(chan runnerResult, len(runners))
	for _, runner := range runners {
		runner := runner
		go func() {
			errCh <- runnerResult{name: runner.name, err: runner.run(ctx)}
		}()
	}

	select {
	case <-ctx.Done():
	case result := <-errCh:
		if result.err != nil && result.err != context.Canceled {
			logger.Error("runtime exited with error", "name", result.name, "err", result.err)
			stop()
			os.Exit(1)
		}
		logger.Warn("runtime exited", "name", result.name, "err", result.err)
		stop()
	}
	logger.Info("stopped")
}

type runner struct {
	name string
	run  func(context.Context) error
}

type runnerResult struct {
	name string
	err  error
}

func buildRunners(cfg *config.Config, logger *slog.Logger, commandHandler *commands.Handler) ([]runner, error) {
	var runners []runner

	if cfg.Feishu.Active() {
		bot := feishu.New(cfg, logger, commandHandler)
		runners = append(runners, runner{name: "feishu", run: bot.Run})
	}

	if cfg.Telegram.Active() {
		bot, err := telegram.New(cfg.Telegram, logger, commandHandler)
		if err != nil {
			return nil, err
		}
		runners = append(runners, runner{name: "telegram", run: bot.Run})
	}

	if cfg.SelfHost.Client.Active() {
		client := selfhost.NewClient(cfg.SelfHost.Client, logger, commandHandler)
		runners = append(runners, runner{name: "selfhost-client", run: client.Run})
	}

	if cfg.SelfHost.Server.Active() {
		server := selfhost.NewServer(cfg.SelfHost.Server, logger)
		runners = append(runners, runner{name: "selfhost-server", run: server.Run})
	}

	return runners, nil
}

func sendWoLDevices(devices []config.DeviceConfig, logger *slog.Logger) error {
	for _, device := range devices {
		opts := wol.SendOptions{
			Interface:   device.Interface,
			BroadcastIP: device.BroadcastIP,
			Port:        device.Port,
		}
		logger.Info("sending WoL packet",
			"device", device.Name,
			"mac", device.MAC,
			"interface", opts.Interface,
			"broadcast", opts.BroadcastIP,
			"port", opts.Port,
		)
		if err := wol.Send([]string{device.MAC}, opts); err != nil {
			return fmt.Errorf("%s(%s): %w", device.Name, device.MAC, err)
		}
	}
	return nil
}

func deviceNames(devices []config.DeviceConfig) string {
	names := make([]string, 0, len(devices))
	for _, device := range devices {
		names = append(names, fmt.Sprintf("%s(%s)", device.Name, device.MAC))
	}
	return fmt.Sprintf("%v", names)
}

func buildLogger(cfg config.LogConfig) *slog.Logger {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	writers := []io.Writer{os.Stderr}

	if cfg.File != "" {
		f, err := os.OpenFile(cfg.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open log file %q: %v (stderr only)\n", cfg.File, err)
		} else {
			writers = append(writers, f)
		}
	}

	var w io.Writer
	if len(writers) == 1 {
		w = writers[0]
	} else {
		w = io.MultiWriter(writers...)
	}

	return slog.New(slog.NewTextHandler(w, opts))
}
