package notify

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Remindal/scout/internal/domain"
)

// TelegramNotifier 直调 Telegram Bot API，不引 SDK。
type TelegramNotifier struct {
	botToken string
	chatID   string
	panelURL string // Web 面板地址，用于消息里的详情链接
	http     *http.Client
}

func NewTelegram(botToken, chatID, panelURL string) *TelegramNotifier {
	return &TelegramNotifier{
		botToken: botToken,
		chatID:   chatID,
		panelURL: strings.TrimRight(panelURL, "/"),
		http:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (n *TelegramNotifier) Name() string { return "telegram" }

// FormatMessage 消息排版，供测试与发送复用。
func (n *TelegramNotifier) FormatMessage(j domain.Job) string {
	var b strings.Builder
	fmt.Fprintf(&b, "🎯 [%d分] %s\n", j.Score, j.Title)
	if j.Budget != "" || len(j.Skills) > 0 {
		fmt.Fprintf(&b, "💰 %s · skills: %s\n", j.Budget, strings.Join(j.Skills, ", "))
	}
	if j.Reason != "" {
		fmt.Fprintf(&b, "📝 %s\n", j.Reason)
	}
	fmt.Fprintf(&b, "🔗 %s\n", j.URL)
	if n.panelURL != "" {
		fmt.Fprintf(&b, "📋 面板: %s/jobs/%d", n.panelURL, j.ID)
	}
	return b.String()
}

func (n *TelegramNotifier) Notify(ctx context.Context, j domain.Job) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.botToken)
	form := url.Values{
		"chat_id":                  {n.chatID},
		"text":                     {n.FormatMessage(j)},
		"disable_web_page_preview": {"true"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := n.http.Do(req)
	if err != nil {
		return fmt.Errorf("telegram send: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("telegram status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
