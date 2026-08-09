package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

type TelegramUpdate struct {
	UpdateID        int `json:"update_id"`
	BusinessMessage struct {
		MessageID            int    `json:"message_id"`
		BusinessConnectionID string `json:"business_connection_id"`
		Chat                 struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		From struct {
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
			IsBot     bool   `json:"is_bot"`
		} `json:"from"`
		Text string `json:"text"`
	} `json:"business_message"`
	Message struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message"`
}

type SendMessagePayload struct {
	ChatID               int64  `json:"chat_id"`
	Text                 string `json:"text"`
	BusinessConnectionID string `json:"business_connection_id,omitempty"`
}

// تخزين حالة الإيقاف والتشغيل العامة للبوت
var isBotStopped = false

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

	apiURL := "https://api.telegram.org/bot" + botToken + "/sendMessage"

	// 1. التحكم المباشر عبر مراسلة البوت الخاص بك (Bot Chat)
	if update.Message.Text != "" {
		cmd := strings.TrimSpace(update.Message.Text)
		chatID := update.Message.Chat.ID

		if cmd == "/stop" {
			isBotStopped = true
			replyText := "🛑 تم إيقاف الرد التلقائي العام."
			sendSimpleMessage(apiURL, chatID, replyText)
		} else if cmd == "/start_bot" {
			isBotStopped = false
			replyText := "🟢 تم تفعيل الرد التلقائي العام."
			sendSimpleMessage(apiURL, chatID, replyText)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		return
	}

	// 2. الرد على رسائل العملاء (Business Messages)
	msg := update.BusinessMessage
	if msg.Text != "" && !msg.IsOutgoing && !isBotStopped {
		chat := msg.Chat.ID
		if chat == 0 {
			chat = msg.From.ID
		}

		senderName := msg.From.FirstName
		if senderName == "" {
			senderName = "صديقي"
		}

		replyText := "مرحبا بك يا " + senderName + "\nانا غير متوفر الان يرجى ترك رسالتك\nوسأرد عليك قريبا"

		payload := SendMessagePayload{
			ChatID:               chat,
			Text:                 replyText,
			BusinessConnectionID: msg.BusinessConnectionID,
		}

		jsonPayload, _ := json.Marshal(payload)
		http.Post(apiURL, "application/json", bytes.NewBuffer(jsonPayload))
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func sendSimpleMessage(apiURL string, chatID int64, text string) {
	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}
	jsonPayload, _ := json.Marshal(payload)
	http.Post(apiURL, "application/json", bytes.NewBuffer(jsonPayload))
}
