package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"

	"github.com/LE-saber/Wrt-Wol/internal/commands"
	"github.com/LE-saber/Wrt-Wol/internal/config"
)

// Bot connects to Feishu via WebSocket long connection and handles /on commands.
type Bot struct {
	client  *lark.Client
	cfg     *config.Config
	logger  *slog.Logger
	handler *commands.Handler
}

// New creates a Bot backed by the official Feishu SDK.
func New(cfg *config.Config, logger *slog.Logger, handler *commands.Handler) *Bot {
	client := lark.NewClient(cfg.Feishu.AppID, cfg.Feishu.AppSecret)
	return &Bot{client: client, cfg: cfg, logger: logger, handler: handler}
}

// Run starts the WebSocket long connection and blocks until ctx is cancelled.
// The SDK handles authentication, heartbeat, and automatic reconnection.
func (b *Bot) Run(ctx context.Context) error {
	logLevel := larkcore.LogLevelInfo
	if b.cfg.Log.Level == "debug" {
		logLevel = larkcore.LogLevelDebug
	}

	eventDispatcher := dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(b.handleMessage)

	wsClient := larkws.NewClient(
		b.cfg.Feishu.AppID,
		b.cfg.Feishu.AppSecret,
		larkws.WithEventHandler(eventDispatcher),
		larkws.WithLogLevel(logLevel),
	)

	b.logger.Info("connecting to Feishu via WebSocket long connection")
	return wsClient.Start(ctx)
}

// handleMessage is called by the SDK for every im.message.receive_v1 event.
func (b *Bot) handleMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	msg := event.Event.Message
	sender := event.Event.Sender

	if msg == nil || sender == nil || msg.MessageId == nil {
		return nil
	}

	openID := ""
	if sender.SenderId != nil && sender.SenderId.OpenId != nil {
		openID = *sender.SenderId.OpenId
	}
	chatID := ""
	if msg.ChatId != nil {
		chatID = *msg.ChatId
	}
	msgType := ""
	if msg.MessageType != nil {
		msgType = *msg.MessageType
	}

	b.logger.Info("message received",
		"open_id", openID,
		"chat_id", chatID,
		"msg_type", msgType,
	)

	// Only process text messages.
	if msgType != "text" {
		return nil
	}

	// ACL: if allowlists are configured, enforce them.
	if !b.isAllowed(openID, chatID) {
		b.logger.Warn("access denied", "open_id", openID, "chat_id", chatID)
		b.reply(ctx, *msg.MessageId, "⛔ 您没有权限触发此操作。")
		return nil
	}

	text, err := extractText(*msg.Content)
	if err != nil {
		b.logger.Error("extract text", "err", err)
		return nil
	}

	cmd := strings.TrimSpace(text)
	b.logger.Info("command", "cmd", cmd, "open_id", openID)

	reply := b.handler.Handle(cmd, commands.Source{
		Platform: "feishu",
		UserID:   openID,
		ChatID:   chatID,
	})
	b.reply(ctx, *msg.MessageId, reply)

	return nil
}

// isAllowed returns true when the sender/chat is permitted.
// Empty allowlists mean everyone is allowed.
func (b *Bot) isAllowed(openID, chatID string) bool {
	sec := b.cfg.Security
	if len(sec.AllowedOpenIDs) == 0 && len(sec.AllowedChatIDs) == 0 {
		return true
	}
	for _, id := range sec.AllowedOpenIDs {
		if id == openID {
			return true
		}
	}
	for _, id := range sec.AllowedChatIDs {
		if id == chatID {
			return true
		}
	}
	return false
}

// reply sends a text reply to the given message.
func (b *Bot) reply(ctx context.Context, messageID, text string) {
	content, _ := json.Marshal(map[string]string{"text": text})
	req := larkim.NewReplyMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType(larkim.MsgTypeText).
			Content(string(content)).
			Build()).
		Build()

	resp, err := b.client.Im.V1.Message.Reply(ctx, req)
	if err != nil {
		b.logger.Error("reply failed", "err", err, "message_id", messageID)
		return
	}
	if !resp.Success() {
		b.logger.Warn("reply API error",
			"code", resp.Code,
			"msg", resp.Msg,
			"message_id", messageID,
		)
	}
}

// extractText parses the Feishu text message content JSON.
// In group chats the SDK delivers the full text including @mentions;
// we strip any leading @mention to get the bare command.
func extractText(content string) (string, error) {
	var v struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(content), &v); err != nil {
		return "", fmt.Errorf("parse content JSON: %w", err)
	}
	text := v.Text
	// Strip leading @mention (e.g. "@bot /on" → "/on").
	if at := strings.Index(text, " "); at != -1 && strings.HasPrefix(text, "@") {
		text = strings.TrimSpace(text[at+1:])
	}
	return text, nil
}
