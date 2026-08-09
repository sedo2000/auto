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
			IsBot     bool   `json:"is_bot"`
		} `json:"from"`
		Text                 string `json:"text"`
		BusinessConnectionID string `json:"business_connection_id"`
		IsOutgoing           bool   `json:"is_outgoing"` // معرفة ما إذا كانت الرسالة صادرة منك
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

	msg := update.BusinessMessage

	// 1. الشرط الأهم: إذا كانت الرسالة صادرة منك أنت (IsOutgoing == true)، يتوقف البوت فوراً ولا يرد
	if msg.IsOutgoing {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		return
	}

	// 2. التأكد من أن الرسالة تحتوي على نص وليست فارغة أو من بوت آخر
	if msg.Text != "" && !msg.From.IsBot {
		chat := msg.Chat.ID
		if chat == 0 {
			chat = msg.From.ID
		}

		// استخراج اسم الشخص المراسل (First Name)
		senderName := msg.From.FirstName
		if senderName == "" {
			senderName = "صديقي"
		}

		// صياغة الرسالة المطلوبة مع تضمين اسم الشخص
		replyText := "مرحباً بك يا " + senderName + "\nانا غير متوفر الان عد في وقت اخر\nهذا رد تلقائي ."

		payload := SendMessagePayload{
			ChatID:               chat,
			Text:                 replyText,
			BusinessConnectionID: msg.BusinessConnectionID,
		}

		jsonPayload, _ := json.Marshal(payload)
		apiURL := "https://api.telegram.org/bot" + botToken + "/sendMessage"
		http.Post(apiURL, "application/json", bytes.NewBuffer(jsonPayload))
	}

I	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
