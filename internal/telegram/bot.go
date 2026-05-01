package telegram

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

type Bot struct {
	cfg     config.TelegramConfig
	logger  *slog.Logger
	handler *commands.Handler
	client  *http.Client
}

type apiResponse[T any] struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
	Result      T      `json:"result"`
}

type update struct {
	UpdateID int64    `json:"update_id"`
	Message  *message `json:"message"`
}

type message struct {
	MessageID int64  `json:"message_id"`
	From      *user  `json:"from"`
	Chat      chat   `json:"chat"`
	Text      string `json:"text"`
}

type user struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type chat struct {
	ID    int64  `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
}

func New(cfg config.TelegramConfig, logger *slog.Logger, handler *commands.Handler) (*Bot, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.ProxyURL != "" {
		proxyURL, err := url.Parse(cfg.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("parse telegram proxy_url: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	return &Bot{
		cfg:     cfg,
		logger:  logger,
		handler: handler,
		client: &http.Client{
			Transport: transport,
			Timeout:   65 * time.Second,
		},
	}, nil
}

func (b *Bot) Run(ctx context.Context) error {
	b.logger.Info("connecting to Telegram via long polling")

	var offset int64
	if b.cfg.DropPending {
		updates, err := b.getUpdates(ctx, 0, 0)
		if err != nil {
			b.logger.Warn("telegram drop pending failed", "err", err)
		}
		for _, upd := range updates {
			if upd.UpdateID >= offset {
				offset = upd.UpdateID + 1
			}
		}
		if offset > 0 {
			b.logger.Info("telegram pending updates skipped", "next_offset", offset)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		updates, err := b.getUpdates(ctx, offset, 50)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			b.logger.Warn("telegram getUpdates failed", "err", err)
			sleep(ctx, 5*time.Second)
			continue
		}

		for _, upd := range updates {
			if upd.UpdateID >= offset {
				offset = upd.UpdateID + 1
			}
			b.handleUpdate(ctx, upd)
		}
	}
}

func (b *Bot) handleUpdate(ctx context.Context, upd update) {
	if upd.Message == nil || upd.Message.Text == "" {
		return
	}

	msg := upd.Message
	userID := ""
	username := ""
	if msg.From != nil {
		userID = strconv.FormatInt(msg.From.ID, 10)
		username = msg.From.Username
	}
	chatID := strconv.FormatInt(msg.Chat.ID, 10)

	b.logger.Info("telegram message received",
		"user_id", userID,
		"username", username,
		"chat_id", chatID,
		"chat_type", msg.Chat.Type,
	)

	if !b.isAllowed(userID, chatID) {
		b.logger.Warn("telegram access denied", "user_id", userID, "chat_id", chatID)
		b.reply(ctx, msg.Chat.ID, msg.MessageID, "⛔ 您没有权限触发此操作。")
		return
	}

	cmd := normalizeMessageCommand(msg.Text)
	b.logger.Info("telegram command", "cmd", cmd, "user_id", userID)

	reply := b.handler.Handle(cmd, commands.Source{
		Platform: "telegram",
		UserID:   userID,
		ChatID:   chatID,
	})
	b.reply(ctx, msg.Chat.ID, msg.MessageID, reply)
}

func (b *Bot) isAllowed(userID, chatID string) bool {
	if len(b.cfg.AllowedUserIDs) == 0 && len(b.cfg.AllowedChatIDs) == 0 {
		return true
	}
	for _, id := range b.cfg.AllowedUserIDs {
		if id == userID {
			return true
		}
	}
	for _, id := range b.cfg.AllowedChatIDs {
		if id == chatID {
			return true
		}
	}
	return false
}

func normalizeMessageCommand(text string) string {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return ""
	}
	cmd := fields[0]
	if at := strings.Index(cmd, "@"); at != -1 {
		cmd = cmd[:at]
	}
	fields[0] = cmd
	return strings.Join(fields, " ")
}

func (b *Bot) getUpdates(ctx context.Context, offset int64, timeout int) ([]update, error) {
	values := url.Values{}
	values.Set("timeout", strconv.Itoa(timeout))
	if offset > 0 {
		values.Set("offset", strconv.FormatInt(offset, 10))
	}
	endpoint := b.endpoint("getUpdates") + "?" + values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var resp apiResponse[[]update]
	if err := b.do(req, &resp); err != nil {
		return nil, err
	}
	if !resp.OK {
		return nil, fmt.Errorf("telegram API error: %s", resp.Description)
	}
	return resp.Result, nil
}

func (b *Bot) reply(ctx context.Context, chatID, replyTo int64, text string) {
	payload := map[string]any{
		"chat_id":             chatID,
		"text":                text,
		"reply_to_message_id": replyTo,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.endpoint("sendMessage"), bytes.NewReader(body))
	if err != nil {
		b.logger.Error("telegram build reply request failed", "err", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	var resp apiResponse[json.RawMessage]
	if err := b.do(req, &resp); err != nil {
		b.logger.Error("telegram reply failed", "err", err)
		return
	}
	if !resp.OK {
		b.logger.Warn("telegram reply API error", "msg", resp.Description)
	}
}

func (b *Bot) do(req *http.Request, out any) error {
	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("parse Telegram response: %w", err)
	}
	return nil
}

func (b *Bot) endpoint(method string) string {
	return fmt.Sprintf("https://api.telegram.org/bot%s/%s", b.cfg.BotToken, method)
}

func sleep(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
