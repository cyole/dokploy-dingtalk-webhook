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
	Title     string `json:"title"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

type DingTalkMarkdown struct {
	MsgType  string            `json:"msgtype"`
	Markdown map[string]string `json:"markdown"`
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

func buildMarkdown(p DokployPayload) DingTalkMarkdown {
	title := p.Title
	if title == "" {
		title = "Dokploy 通知"
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("### %s", title))
	if p.Message != "" {
		lines = append(lines, "", p.Message)
	}
	if p.Timestamp != "" {
		if t, err := time.Parse(time.RFC3339Nano, p.Timestamp); err == nil {
			loc, _ := time.LoadLocation("Asia/Shanghai")
			lines = append(lines, "", fmt.Sprintf("> %s", t.In(loc).Format("2006-01-02 15:04:05")))
		}
	}

	return DingTalkMarkdown{
		MsgType:  "markdown",
		Markdown: map[string]string{"title": title, "text": strings.Join(lines, "\n")},
	}
}

func sendToDingTalk(accessToken, secret string, body DingTalkMarkdown) (*DingTalkResponse, error) {
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

	md := buildMarkdown(payload)
	result, err := sendToDingTalk(accessToken, secret, md)
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
