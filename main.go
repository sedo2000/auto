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

type SendMessagePayload struct {
	ChatID               int64  `json:"chat_id"`
	Text                 string `json:"text"`
	BusinessConnectionID string `json:"business_connection_id,omitempty"`
}

type DeleteMessagePayload struct {
	ChatID    int64 `json:"chat_id"`
	MessageID int   `json:"message_id"`
}

// تخزين مؤقت لحالة المحادثات المتوقفة (مفتاح خارجي لرقم المحادثة)
// ملاحظة: في بيئة السيرفرليس قد تُعاد ذاكرة الكود، ولكنها تفيد في الجلسات النشطة
var stoppedChats = make(map[int64]bool)

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
	chat := msg.Chat.ID
	if chat == 0 {
		chat = msg.From.ID
	}

	// 1. إذا كانت الرسالة صادرة منك أنت (أوامر التحكم أو الردود العادية)
	if msg.IsOutgoing {
		textTrimmed := strings.TrimSpace(msg.Text)

		// إذا أرسلت أنت كلمة "إيقاف"
		if textTrimmed == "إيقاف" {
			stoppedChats[chat] = true
			deleteMessage(botToken, chat, msg.MessageID)
		} else if textTrimmed == "تشغيل" {
			// إذا أرسلت أنت كلمة "تشغيل"
			delete(stoppedChats, chat)
			deleteMessage(botToken, chat, msg.MessageID)
		} else {
			// أي رسالة عادية ترد بها أنت على الشخص، سيقوم البوت تلقائياً بإيقاف الرد الآلي مؤقتاً لهذه المحادثة وحذف الحاجة لتكرار الرد
			stoppedChats[chat] = true
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		return
	}

	// 2. إذا كانت المحادثة متوقفة بناءً على طلبك، لا تقم بالرد نهائياً
	if stoppedChats[chat] {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		return
	}

	// 3. الرد الآلي على الشخص المراسل
	if msg.Text != "" && !msg.From.IsBot {
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
		apiURL := "https://api.telegram.org/bot" + botToken + "/sendMessage"
		http.Post(apiURL, "application/json", bytes.NewBuffer(jsonPayload))
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// دالة لحذف رسائلك (مثل أمر إيقاف أو تشغيل) تلقائياً لكي تبقى المحادثة نظيفة
func deleteMessage(botToken string, chatID int64, messageID int) {
	payload := DeleteMessagePayload{
		ChatID:    chatID,
		MessageID: messageID,
	}
	jsonPayload, _ := json.Marshal(payload)
	apiURL := "https://api.telegram.org/bot" + botToken + "/deleteMessage"
	http.Post(apiURL, "application/json", bytes.NewBuffer(jsonPayload))
}
