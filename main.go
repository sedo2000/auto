package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
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

	// تحقق من أن الرسالة الواردة تحتوي على نص
	if update.BusinessMessage.Text != "" {
		chat := update.BusinessMessage.Chat.ID
		if chat == 0 {
			chat = update.BusinessMessage.From.ID
		}

		// تجهيز الرد
		replyText := "مرحباً! لقد تلقيت رسالتك بشكل آلي."
		payload := SendMessagePayload{
			ChatID:               chat,
			Text:                 replyText,
			BusinessConnectionID: update.BusinessMessage.BusinessConnectionID,
		}

		jsonPayload, _ := json.Marshal(payload)

		// إرسال الطلب إلى Telegram API
		apiURL := "https://api.telegram.org/bot" + botToken + "/sendMessage"
		http.Post(apiURL, "application/json", bytes.NewBuffer(jsonPayload))
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
