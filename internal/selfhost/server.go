package selfhost

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LE-saber/Wrt-Wol/internal/config"
)

type Server struct {
	cfg    config.SelfHostServerConfig
	logger *slog.Logger
	state  serverState
}

type serverState struct {
	mu             sync.Mutex
	commandID      int64
	target         string
	lastReport     string
	lastQueued     time.Time
	lastReportTime time.Time
}

type pollResponse struct {
	CommandID int64  `json:"command_id"`
	Command   string `json:"command"`
	Target    string `json:"target,omitempty"`
}

type apiStatus struct {
	OK        bool   `json:"ok"`
	CommandID int64  `json:"command_id,omitempty"`
	Message   string `json:"message,omitempty"`
}

func NewServer(cfg config.SelfHostServerConfig, logger *slog.Logger) *Server {
	return &Server{cfg: cfg, logger: logger}
}

func (server *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleIndex)
	mux.HandleFunc("/wake", server.handleWakePage)
	mux.HandleFunc("/api/wake", server.handleWakeAPI)
	mux.HandleFunc("/api/poll", server.handlePoll)
	mux.HandleFunc("/api/report", server.handleReport)
	mux.HandleFunc("/healthz", server.handleHealth)

	address := fmt.Sprintf("%s:%d", server.cfg.Host, server.cfg.Port)
	httpServer := &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		server.logger.Info("selfhost relay server listening", "addr", address)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (server *Server) handleIndex(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	server.state.mu.Lock()
	data := map[string]any{
		"CommandID":      server.state.commandID,
		"Target":         server.state.target,
		"LastReport":     server.state.lastReport,
		"LastQueued":     formatTime(server.state.lastQueued),
		"LastReportTime": formatTime(server.state.lastReportTime),
	}
	server.state.mu.Unlock()

	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = indexTemplate.Execute(response, data)
}

func (server *Server) handleWakePage(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Redirect(response, request, "/", http.StatusSeeOther)
		return
	}
	if err := request.ParseForm(); err != nil {
		http.Error(response, "bad form", http.StatusBadRequest)
		return
	}
	if !server.validAdminToken(request.FormValue("token")) {
		http.Error(response, "invalid token", http.StatusUnauthorized)
		return
	}

	target := request.FormValue("target")
	commandID := server.queueWake(target)
	server.logger.Info("selfhost wake queued from web", "command_id", commandID, "target", normalizeTarget(target), "remote", request.RemoteAddr)
	http.Redirect(response, request, fmt.Sprintf("/?queued=%d", commandID), http.StatusSeeOther)
}

func (server *Server) handleWakeAPI(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !server.validAdminToken(tokenFromRequest(request, "token")) {
		writeJSON(response, http.StatusUnauthorized, apiStatus{OK: false, Message: "invalid token"})
		return
	}

	target := valueFromRequest(request, "target")
	commandID := server.queueWake(target)
	server.logger.Info("selfhost wake queued from API", "command_id", commandID, "target", normalizeTarget(target), "remote", request.RemoteAddr)
	writeJSON(response, http.StatusOK, apiStatus{OK: true, CommandID: commandID})
}

func (server *Server) handlePoll(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !server.validDeviceToken(request.URL.Query().Get("token")) {
		writeJSON(response, http.StatusUnauthorized, apiStatus{OK: false, Message: "invalid token"})
		return
	}

	lastID, _ := strconv.ParseInt(request.URL.Query().Get("last_id"), 10, 64)

	server.state.mu.Lock()
	commandID := server.state.commandID
	target := server.state.target
	server.state.mu.Unlock()

	if commandID > lastID {
		writeJSON(response, http.StatusOK, pollResponse{CommandID: commandID, Command: "wake", Target: target})
		return
	}
	writeJSON(response, http.StatusOK, pollResponse{})
}

func (server *Server) handleReport(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var report struct {
		DeviceToken string `json:"device_token"`
		CommandID   int64  `json:"command_id"`
		OK          bool   `json:"ok"`
		Message     string `json:"message"`
	}
	if err := json.NewDecoder(request.Body).Decode(&report); err != nil {
		writeJSON(response, http.StatusBadRequest, apiStatus{OK: false, Message: "bad json"})
		return
	}
	if !server.validDeviceToken(report.DeviceToken) {
		writeJSON(response, http.StatusUnauthorized, apiStatus{OK: false, Message: "invalid token"})
		return
	}

	status := "failed"
	if report.OK {
		status = "ok"
	}
	message := fmt.Sprintf("command %d %s: %s", report.CommandID, status, report.Message)

	server.state.mu.Lock()
	server.state.lastReport = message
	server.state.lastReportTime = time.Now()
	server.state.mu.Unlock()

	server.logger.Info("selfhost report received", "command_id", report.CommandID, "ok", report.OK, "message", report.Message)
	writeJSON(response, http.StatusOK, apiStatus{OK: true})
}

func (server *Server) handleHealth(response http.ResponseWriter, request *http.Request) {
	writeJSON(response, http.StatusOK, apiStatus{OK: true, Message: "ok"})
}

func (server *Server) queueWake(target string) int64 {
	server.state.mu.Lock()
	defer server.state.mu.Unlock()
	server.state.commandID++
	server.state.target = normalizeTarget(target)
	server.state.lastQueued = time.Now()
	return server.state.commandID
}

func normalizeTarget(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return "all"
	}
	return target
}

func (server *Server) validAdminToken(token string) bool {
	return constantStringEqual(token, server.cfg.AdminToken)
}

func (server *Server) validDeviceToken(token string) bool {
	return constantStringEqual(token, server.cfg.DeviceToken)
}

func tokenFromRequest(request *http.Request, field string) string {
	auth := request.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[len("Bearer "):])
	}
	return valueFromRequest(request, field)
}

func valueFromRequest(request *http.Request, field string) string {
	if token := request.URL.Query().Get(field); token != "" {
		return token
	}
	_ = request.ParseForm()
	return request.FormValue(field)
}

func constantStringEqual(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "暂无"
	}
	return value.Format("2006-01-02 15:04:05")
}

var indexTemplate = template.Must(template.New("index").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Wrt-Wol Selfhost</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 2rem; background: #f6f7f9; color: #1f2937; }
    main { max-width: 720px; margin: auto; background: #fff; border-radius: 14px; padding: 2rem; box-shadow: 0 10px 30px rgba(0,0,0,.08); }
    label, input, button { display: block; width: 100%; box-sizing: border-box; }
    input { margin: .5rem 0 1rem; padding: .75rem; border: 1px solid #d1d5db; border-radius: 8px; }
    button { padding: .8rem 1rem; border: 0; border-radius: 8px; background: #2563eb; color: #fff; font-size: 1rem; cursor: pointer; }
    .meta { margin-top: 1.5rem; line-height: 1.8; color: #4b5563; }
  </style>
</head>
<body>
<main>
  <h1>Wrt-Wol 自建远程开机</h1>
  <p>输入管理 Token 后点击按钮，云端会下发一次开机指令，OpenWrt 端轮询到指令后发送 WoL。</p>
  <form method="post" action="/wake">
    <label for="token">管理 Token</label>
    <input id="token" name="token" type="password" autocomplete="current-password" required>
    <label for="target">目标设备</label>
    <input id="target" name="target" type="text" placeholder="all、1、NAS、设备名" value="all">
    <button type="submit">发送远程开机指令</button>
  </form>
  <div class="meta">
    <div>当前指令 ID：{{.CommandID}}</div>
    <div>当前目标：{{.Target}}</div>
    <div>最近下发：{{.LastQueued}}</div>
    <div>最近回报：{{.LastReportTime}}</div>
    <div>回报内容：{{.LastReport}}</div>
  </div>
</main>
</body>
</html>`))
