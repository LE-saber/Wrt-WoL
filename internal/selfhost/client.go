package selfhost

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/LE-saber/Wrt-Wol/internal/commands"
	"github.com/LE-saber/Wrt-Wol/internal/config"
)

type Client struct {
	cfg     config.SelfHostClientConfig
	logger  *slog.Logger
	handler *commands.Handler
	client  *http.Client
}

func NewClient(cfg config.SelfHostClientConfig, logger *slog.Logger, handler *commands.Handler) *Client {
	return &Client{
		cfg:     cfg,
		logger:  logger,
		handler: handler,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (client *Client) Run(ctx context.Context) error {
	interval, err := time.ParseDuration(client.cfg.PollInterval)
	if err != nil || interval <= 0 {
		interval = 5 * time.Second
	}

	client.logger.Info("selfhost relay client starting", "server_url", client.cfg.ServerURL, "poll_interval", interval)
	var lastID int64

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		poll, err := client.poll(ctx, lastID)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			client.logger.Warn("selfhost poll failed", "err", err)
			sleep(ctx, interval)
			continue
		}

		if poll.Command == "wake" && poll.CommandID > lastID {
			target := strings.TrimSpace(poll.Target)
			if target == "" {
				target = "all"
			}
			client.logger.Info("selfhost wake command received", "command_id", poll.CommandID, "target", target)
			message := client.handler.Handle("/on "+target, commands.Source{Platform: "selfhost", UserID: "relay", ChatID: "server"})
			ok := !strings.HasPrefix(message, "❌") && !strings.HasPrefix(message, "未知")
			if err := client.report(ctx, poll.CommandID, ok, message); err != nil {
				client.logger.Warn("selfhost report failed", "err", err)
			}
			lastID = poll.CommandID
		}

		sleep(ctx, interval)
	}
}

func (client *Client) poll(ctx context.Context, lastID int64) (pollResponse, error) {
	values := url.Values{}
	values.Set("token", client.cfg.DeviceToken)
	values.Set("last_id", strconv.FormatInt(lastID, 10))

	endpoint := client.endpoint("/api/poll") + "?" + values.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return pollResponse{}, err
	}

	var result pollResponse
	if err := client.do(request, &result); err != nil {
		return pollResponse{}, err
	}
	return result, nil
}

func (client *Client) report(ctx context.Context, commandID int64, ok bool, message string) error {
	payload := map[string]any{
		"device_token": client.cfg.DeviceToken,
		"command_id":   commandID,
		"ok":           ok,
		"message":      message,
	}
	body, _ := json.Marshal(payload)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint("/api/report"), bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	var status apiStatus
	return client.do(request, &status)
}

func (client *Client) do(request *http.Request, out any) error {
	response, err := client.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(data)))
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("parse selfhost response: %w", err)
	}
	return nil
}

func (client *Client) endpoint(path string) string {
	return strings.TrimRight(client.cfg.ServerURL, "/") + path
}

func sleep(ctx context.Context, duration time.Duration) {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
