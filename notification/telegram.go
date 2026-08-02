package notification

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

// TelegramService ساختاری برای مدیریت ارسال پیام تلگرام
type TelegramService struct {
	BotToken string
	Client   *http.Client
}

// NewTelegramService یک نمونه جدید از سرویس تلگرام را می‌سازد
func NewTelegramService(botToken string) *TelegramService {
	return &TelegramService{
		BotToken: botToken,
		Client: &http.Client{
			Timeout: 10 * time.Second, // تایم‌اوت برای جلوگیری از مسدود شدن گوروتین
		},
	}
}

// SendMessage پیام را به صورت غیرهمزمان (Non-blocking) به تلگرام ارسال می‌کند
func (s *TelegramService) SendMessage(chatID, text, parseMode string) {
	go func() {
		// ساخت URL اختصاصی بات
		apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.BotToken)

		// آماده‌سازی داده‌ها به صورت Form Data (استاندارد و امن)
		data := url.Values{}
		data.Set("chat_id", chatID)
		data.Set("text", text)

		// اگر حالت قالب‌بندی (مثل HTML یا Markdown) مشخص شده بود، اضافه کن
		if parseMode != "" {
			data.Set("parse_mode", parseMode)
		}

		// ارسال درخواست POST
		resp, err := s.Client.PostForm(apiURL, data)
		if err != nil {
			log.Printf("[TELEGRAM ERROR] Failed to send message to %s: %v", chatID, err)
			return
		}
		defer resp.Body.Close()

		// تلگرام معمولاً در صورت خطا، کد وضعیت 400 یا 401 برمی‌گرداند
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			log.Printf("[TELEGRAM ERROR] API returned status %d for %s. Body: %s", resp.StatusCode, chatID, string(body))
			return
		}

		// بررسی پاسخ موفقیت‌آمیز (اختیاری ولی توصیه می‌شود)
		// تلگرام در صورت موفقیت، فیلد "ok": true را برمی‌گرداند
		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
			if ok, exists := result["ok"].(bool); exists && ok {
				log.Printf("[TELEGRAM SUCCESS] Message sent successfully to %s", chatID)
			} else {
				log.Printf("[TELEGRAM WARNING] API responded with 200 OK but 'ok' field is false. Response: %v", result)
			}
		}
	}()
}
