package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"sync"
)

type TelegramUpdate struct {
	UpdateID int `json:"update_id"`
	BusinessMessage struct {
		MessageID int `json:"message_id"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		From struct {
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
		} `json:"from"`
		Text                 string `json:"text"`
		BusinessConnectionID string `json:"business_connection_id"`
	} `json:"business_message"`
}

type SendMessagePayload struct {
	ChatID               int64  `json:"chat_id"`
	Text                 string `json:"text"`
	BusinessConnectionID string `json:"business_connection_id,omitempty"`
}

type TelegramUserResponse struct {
	Ok     bool `json:"ok"`
	Result struct {
		FirstName string `json:"first_name"`
		Username  string `json:"username"`
	} `json:"result"`
}

// متغيرات لتخزين الاسم تلقائياً في الذاكرة المؤقتة لمنع التكرار
var (
	cachedOwnerName string
	mu              sync.Mutex
)

// دالة لجلب اسم البوت/الحساب تلقائياً من تيليجرام
func getOwnerName(botToken string) string {
	mu.Lock()
	defer mu.Unlock()

	if cachedOwnerName != "" {
		return cachedOwnerName
	}

	apiURL := "https://api.telegram.org/bot" + botToken + "/getMe"
	resp, err := http.Get(apiURL)
	if err != nil {
		return "صاحب الحساب"
	}
	defer resp.Body.Close()

	var userResp TelegramUserResponse
	err = json.NewDecoder(resp.Body).Decode(&userResp)
	if err != nil || !userResp.Ok {
		return "صاحب الحساب"
	}

	cachedOwnerName = userResp.Result.FirstName
	return cachedOwnerName
}

func Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		http.Error(w, "Bot token not configured", http.StatusInternalServerError)
		return
	}

	var update TelegramUpdate
	err := json.NewDecoder(r.Body).Decode(&update)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	text := update.BusinessMessage.Text
	if text != "" {
		chat := update.BusinessMessage.Chat.ID
		if chat == 0 {
			chat = update.BusinessMessage.From.ID
		}

		// اكتشاف اسم صاحب الحساب تلقائياً عبر API
		ownerName := getOwnerName(botToken)

		// صياغة الرد التلقائي بالاسم المكتشف تلقائياً
		replyText := "أهلاً وسهلاً، " + ownerName + " غير موجود حالياً. اترك رسالتك وسيرد عليك في أقرب وقت."

		payload := SendMessagePayload{
			ChatID:               chat,
			Text:                 replyText,
			BusinessConnectionID: update.BusinessMessage.BusinessConnectionID,
		}

		jsonPayload, _ := json.Marshal(payload)
		apiURL := "https://api.telegram.org/bot" + botToken + "/sendMessage"
		http.Post(apiURL, "application/json", bytes.NewBuffer(jsonPayload))
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
