package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type DokployPayload struct {
	Title           string  `json:"title"`
	Message         string  `json:"message"`
	Timestamp       string  `json:"timestamp"`
	Date            string  `json:"date"`
	Status          string  `json:"status"`
	Type            string  `json:"type"`
	ProjectName     string  `json:"projectName"`
	ApplicationName string  `json:"applicationName"`
	ApplicationType string  `json:"applicationType"`
	EnvironmentName string  `json:"environmentName"`
	BuildLink       string  `json:"buildLink"`
	Domains         string  `json:"domains"`
	ErrorMessage    string  `json:"errorMessage"`
	DatabaseType    string  `json:"databaseType"`
	DatabaseName    string  `json:"databaseName"`
	ServerName      string  `json:"serverName"`
	AlertType       string  `json:"alertType"`
	CurrentValue    float64 `json:"currentValue"`
	Threshold       float64 `json:"threshold"`
}

type DingTalkMessage struct {
	MsgType    string                 `json:"msgtype"`
	ActionCard map[string]string      `json:"actionCard,omitempty"`
	Markdown   map[string]string      `json:"markdown,omitempty"`
	At         map[string]interface{} `json:"at,omitempty"`
}

type DingTalkResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func sign(timestamp int64, secret string) string {
	raw := fmt.Sprintf("%d\n%s", timestamp, secret)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(raw))
	return url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
}

func dingtalkURL(accessToken, secret string) string {
	u := fmt.Sprintf("https://oapi.dingtalk.com/robot/send?access_token=%s", accessToken)
	if secret != "" {
		ts := time.Now().UnixMilli()
		u += fmt.Sprintf("&timestamp=%d&sign=%s", ts, sign(ts, secret))
	}
	return u
}

func formatTimestamp(p DokployPayload) string {
	if p.Date != "" {
		return p.Date
	}
	if p.Timestamp != "" {
		if t, err := time.Parse(time.RFC3339Nano, p.Timestamp); err == nil {
			loc, _ := time.LoadLocation("Asia/Shanghai")
			return t.In(loc).Format("2006-01-02 15:04:05")
		}
	}
	return ""
}

func detectNotificationType(p DokployPayload) string {
	if p.AlertType == "server-threshold" {
		return "server-threshold"
	}
	switch p.Type {
	case "build":
		if p.Status == "error" {
			return "build-error"
		}
		return "build-success"
	}
	if p.DatabaseType != "" || p.DatabaseName != "" {
		if p.Status == "error" {
			return "database-backup-error"
		}
		return "database-backup-success"
	}
	titleLower := strings.ToLower(p.Title)
	if strings.Contains(titleLower, "restart") {
		return "dokploy-restart"
	}
	if strings.Contains(titleLower, "cleanup") || strings.Contains(titleLower, "clean up") {
		return "docker-cleanup"
	}
	if strings.Contains(titleLower, "backup") && strings.Contains(titleLower, "volume") {
		return "volume-backup"
	}
	return "generic"
}

func statusIcon(status string) string {
	switch status {
	case "success":
		return "✅"
	case "error":
		return "❌"
	case "alert":
		return "⚠️"
	default:
		return "📋"
	}
}

func buildNotificationCard(p DokployPayload) DingTalkMessage {
	notifType := detectNotificationType(p)
	title := p.Title
	if title == "" {
		title = "Dokploy 通知"
	}
	ts := formatTimestamp(p)

	var lines []string

	switch notifType {
	case "build-success":
		lines = buildDeployCard(p, ts, true)
	case "build-error":
		lines = buildDeployCard(p, ts, false)
	case "database-backup-success":
		lines = buildDatabaseBackupCard(p, ts, true)
	case "database-backup-error":
		lines = buildDatabaseBackupCard(p, ts, false)
	case "server-threshold":
		lines = buildServerThresholdCard(p, ts)
	default:
		lines = buildGenericCard(p, ts)
	}

	text := strings.Join(lines, "\n")

	if p.BuildLink != "" {
		return DingTalkMessage{
			MsgType: "actionCard",
			ActionCard: map[string]string{
				"title":          title,
				"text":           text,
				"singleTitle":    "查看构建详情 →",
				"singleURL":      p.BuildLink,
				"btnOrientation": "0",
			},
		}
	}

	return DingTalkMessage{
		MsgType:  "markdown",
		Markdown: map[string]string{"title": title, "text": text},
	}
}

func buildDeployCard(p DokployPayload, ts string, success bool) []string {
	icon := "✅"
	statusText := "部署成功"
	if !success {
		icon = "❌"
		statusText = "构建失败"
	}

	lines := []string{fmt.Sprintf("### %s %s", icon, statusText)}
	lines = append(lines, "", "---", "")

	if p.ProjectName != "" {
		lines = append(lines, fmt.Sprintf("**项目**: %s", p.ProjectName))
	}
	if p.ApplicationName != "" {
		lines = append(lines, fmt.Sprintf("**应用**: %s", p.ApplicationName))
	}
	if p.ApplicationType != "" {
		lines = append(lines, fmt.Sprintf("**类型**: %s", p.ApplicationType))
	}
	if p.EnvironmentName != "" {
		lines = append(lines, fmt.Sprintf("**环境**: %s", p.EnvironmentName))
	}
	if p.Domains != "" {
		lines = append(lines, fmt.Sprintf("**域名**: %s", p.Domains))
	}

	if !success && p.ErrorMessage != "" {
		errMsg := p.ErrorMessage
		if len(errMsg) > 500 {
			errMsg = errMsg[:500] + "..."
		}
		lines = append(lines, "", "**错误信息**:", "", fmt.Sprintf("> %s", errMsg))
	}

	if p.Message != "" && p.Message != "Build completed successfully" && p.Message != "Build failed with errors" {
		lines = append(lines, "", fmt.Sprintf("> %s", p.Message))
	}

	if ts != "" {
		lines = append(lines, "", fmt.Sprintf("⏱ %s", ts))
	}

	return lines
}

func buildDatabaseBackupCard(p DokployPayload, ts string, success bool) []string {
	icon := "✅"
	statusText := "数据库备份成功"
	if !success {
		icon = "❌"
		statusText = "数据库备份失败"
	}

	lines := []string{fmt.Sprintf("### %s %s", icon, statusText)}
	lines = append(lines, "", "---", "")

	if p.ProjectName != "" {
		lines = append(lines, fmt.Sprintf("**项目**: %s", p.ProjectName))
	}
	if p.ApplicationName != "" {
		lines = append(lines, fmt.Sprintf("**应用**: %s", p.ApplicationName))
	}
	if p.DatabaseType != "" {
		lines = append(lines, fmt.Sprintf("**数据库类型**: %s", p.DatabaseType))
	}
	if p.DatabaseName != "" {
		lines = append(lines, fmt.Sprintf("**数据库名称**: %s", p.DatabaseName))
	}

	if !success && p.ErrorMessage != "" {
		errMsg := p.ErrorMessage
		if len(errMsg) > 500 {
			errMsg = errMsg[:500] + "..."
		}
		lines = append(lines, "", "**错误信息**:", "", fmt.Sprintf("> %s", errMsg))
	}

	if ts != "" {
		lines = append(lines, "", fmt.Sprintf("⏱ %s", ts))
	}

	return lines
}

func buildServerThresholdCard(p DokployPayload, ts string) []string {
	typeLabel := p.Type
	if typeLabel == "" {
		typeLabel = "Unknown"
	}
	lines := []string{fmt.Sprintf("### ⚠️ 服务器 %s 告警", typeLabel)}
	lines = append(lines, "", "---", "")

	if p.ServerName != "" {
		lines = append(lines, fmt.Sprintf("**服务器**: %s", p.ServerName))
	}
	lines = append(lines, fmt.Sprintf("**告警类型**: %s", typeLabel))
	if p.CurrentValue > 0 {
		lines = append(lines, fmt.Sprintf("**当前值**: %.2f%%", p.CurrentValue))
	}
	if p.Threshold > 0 {
		lines = append(lines, fmt.Sprintf("**阈值**: %.2f%%", p.Threshold))
	}
	if p.Message != "" {
		lines = append(lines, "", fmt.Sprintf("> %s", p.Message))
	}
	if ts != "" {
		lines = append(lines, "", fmt.Sprintf("⏱ %s", ts))
	}

	return lines
}

func buildGenericCard(p DokployPayload, ts string) []string {
	title := p.Title
	if title == "" {
		title = "Dokploy 通知"
	}

	icon := statusIcon(p.Status)
	lines := []string{fmt.Sprintf("### %s %s", icon, title)}
	lines = append(lines, "", "---", "")

	if p.ProjectName != "" {
		lines = append(lines, fmt.Sprintf("**项目**: %s", p.ProjectName))
	}
	if p.ApplicationName != "" {
		lines = append(lines, fmt.Sprintf("**应用**: %s", p.ApplicationName))
	}
	if p.ApplicationType != "" {
		lines = append(lines, fmt.Sprintf("**类型**: %s", p.ApplicationType))
	}
	if p.ServerName != "" {
		lines = append(lines, fmt.Sprintf("**服务器**: %s", p.ServerName))
	}

	if p.Message != "" {
		lines = append(lines, "", fmt.Sprintf("> %s", p.Message))
	}

	if p.ErrorMessage != "" {
		errMsg := p.ErrorMessage
		if len(errMsg) > 500 {
			errMsg = errMsg[:500] + "..."
		}
		lines = append(lines, "", "**错误信息**:", "", fmt.Sprintf("> %s", errMsg))
	}

	if ts != "" {
		lines = append(lines, "", fmt.Sprintf("⏱ %s", ts))
	}

	return lines
}

func sendToDingTalk(accessToken, secret string, body DingTalkMessage) (*DingTalkResponse, error) {
	data, _ := json.Marshal(body)
	resp, err := http.Post(dingtalkURL(accessToken, secret), "application/json", strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var result DingTalkResponse
	json.Unmarshal(raw, &result)
	return &result, nil
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := r.PathValue("access_token")
	if accessToken == "" {
		http.Error(w, "access_token is required", http.StatusBadRequest)
		return
	}
	secret := r.URL.Query().Get("secret")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	log.Printf("[Dokploy ➜ DingTalk] received: %s", string(body))

	var payload DokployPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	msg := buildNotificationCard(payload)
	result, err := sendToDingTalk(accessToken, secret, msg)
	if err != nil {
		log.Printf("[DingTalk error] %v", err)
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}

	log.Printf("[DingTalk response] errcode=%d errmsg=%s", result.ErrCode, result.ErrMsg)

	w.Header().Set("Content-Type", "application/json")
	if result.ErrCode != 0 {
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "errcode": result.ErrCode, "errmsg": result.ErrMsg})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9119"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /webhook/{access_token}", webhookHandler)
	mux.HandleFunc("/health", healthHandler)

	addr := ":" + port
	log.Printf("dokploy-dingtalk-webhook listening on %s", addr)

	if err := http.ListenAndServe(addr, logMiddleware(mux)); err != nil {
		log.Fatal(err)
	}
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}
