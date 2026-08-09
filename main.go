package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

type TelegramUpdate struct {
	UpdateID int `json:"update_id"`
	Message  struct {
		MessageID int `json:"message_id"`
		Chat      struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		From struct {
			ID int64 `json:"id"`
		} `json:"from"`
		Text string `json:"text"`
	} `json:"message"`
	BusinessMessage struct {
		MessageID            int    `json:"message_id"`
		IsOutgoing           bool   `json:"is_outgoing"`
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
}

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
		w.WriteHeader(http.StatusOK)
		return
	}

	apiURL := "https://api.telegram.org/bot" + botToken + "/sendMessage"

	// 1. استقبال الأوامر المباشرة داخل محادثة البوت (/stop أو /start_bot)
	if update.Message.Text != "" {
		cmd := strings.TrimSpace(update.Message.Text)
		chatID := update.Message.Chat.ID

		if cmd == "/stop" {
			isBotStopped = true
			sendSimpleMessage(apiURL, chatID, "🛑 تم إيقاف الرد التلقائي.")
		} else if cmd == "/start_bot" {
			isBotStopped = false
			sendSimpleMessage(apiURL, chatID, "🟢 تم تفعيل الرد التلقائي.")
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

		payload := map[string]interface{}{
			"chat_id":                chat,
			"text":                   replyText,
			"business_connection_id": msg.BusinessConnectionID,
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
