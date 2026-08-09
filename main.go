package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// --- هيكلة البيانات الواردة من تيليجرام ---
type TelegramUpdate struct {
	UpdateID        int              `json:"update_id"`
	Message         *Message         `json:"message"`
	CallbackQuery   *CallbackQuery   `json:"callback_query"`
	BusinessMessage *BusinessMessage `json:"business_message"`
}

type Message struct {
	MessageID int    `json:"message_id"`
	Chat      Chat   `json:"chat"`
	From      User   `json:"from"`
	Text      string `json:"text"`
}

type CallbackQuery struct {
	ID      string  `json:"id"`
	From    User    `json:"from"`
	Message Message `json:"message"`
	Data    string  `json:"data"`
}

type BusinessMessage struct {
	MessageID            int    `json:"message_id"`
	IsOutgoing           bool   `json:"is_outgoing"`
	BusinessConnectionID string `json:"business_connection_id"`
	Chat                 Chat   `json:"chat"`
	From                 User   `json:"from"`
	Text                 string `json:"text"`
}

type Chat struct {
	ID int64 `json:"id"`
}

type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	IsBot     bool   `json:"is_bot"`
}

// --- هياكل إرسال البيانات ---
type SendMessagePayload struct {
	ChatID               int64       `json:"chat_id"`
	Text                 string      `json:"text"`
	BusinessConnectionID string      `json:"business_connection_id,omitempty"`
	ReplyMarkup          interface{} `json:"reply_markup,omitempty"`
}

type DeleteMessagePayload struct {
	ChatID    int64 `json:"chat_id"`
	MessageID int   `json:"message_id"`
}

type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

type InlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

// --- متغيرات لتخزين حالة البوت (تعمل في الذاكرة المؤقتة) ---
var (
	isBotStopped = false
	excludedIDs  = make(map[int64]bool)
	waitingForID = make(map[int64]bool)
)

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

	// 1. التعامل مع ضغطات الأزرار الشفافة (Callback Queries)
	if update.CallbackQuery != nil {
		handleCallbackQuery(botToken, update.CallbackQuery)
		w.WriteHeader(http.StatusOK)
		return
	}

	// 2. التعامل مع الأوامر المباشرة في محادثة البوت (/start, /id) وإدخال الايدي
	if update.Message != nil && update.Message.Text != "" {
		handleDirectMessage(botToken, update.Message)
		w.WriteHeader(http.StatusOK)
		return
	}

	// 3. التعامل مع رسائل العملاء (Business Messages)
	if update.BusinessMessage != nil && update.BusinessMessage.Text != "" {
		handleBusinessMessage(botToken, update.BusinessMessage)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// ==========================================
// دوال معالجة الأحداث (للحفاظ على نظافة الكود)
// ==========================================

func handleCallbackQuery(botToken string, cb *CallbackQuery) {
	chatID := cb.Message.Chat.ID
	msgID := cb.Message.MessageID
	data := cb.Data

	// حذف الرسالة التي تحتوي على الأزرار بعد الضغط عليها
	deleteMessage(botToken, chatID, msgID)

	switch data {
	case "stop_reply":
		isBotStopped = true
		sendMessage(botToken, chatID, "🛑 تم إيقاف الرد التلقائي بنجاح.", nil, "")
	case "start_reply":
		isBotStopped = false
		sendMessage(botToken, chatID, "🟢 تم تشغيل الرد التلقائي بنجاح.", nil, "")
	case "exclude_account":
		waitingForID[chatID] = true
		txt := "ارسل ايدي حسابك الان لتعطيل الرد التلقائي من الظهور في محادثتك فقط\nاذا لا تعرف كيف استخراج ايدي حسابك يمكنك الضغط على هذا الامر\n/id\nسيظهر لك رقم انسخه وارسله في هذه المحادثة لأستثناء حسابك"
		sendMessage(botToken, chatID, txt, nil, "")
	}

	// إبلاغ تيليجرام بنجاح الاستجابة للزر لمنع التحميل المستمر
	answerCallbackQuery(botToken, cb.ID)
}

func handleDirectMessage(botToken string, msg *Message) {
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)

	// إذا كان البوت ينتظر إرسال الايدي للاستثناء
	if waitingForID[chatID] && text != "/id" && text != "/start" {
		id, err := strconv.ParseInt(text, 10, 64)
		if err == nil {
			excludedIDs[id] = true
			waitingForID[chatID] = false
			sendMessage(botToken, chatID, fmt.Sprintf("✅ تم استثناء الحساب ذو الايدي %d بنجاح من الرد التلقائي.", id), nil, "")
		} else {
			sendMessage(botToken, chatID, "❌ عذراً، الرقم غير صحيح. يرجى إرسال أرقام فقط (الايدي).", nil, "")
		}
		return
	}

	if text == "/start" {
		txt := "أهلاً بك 🤖\nهذا البوت مخصص للرد التلقائي نيابة عنك لحسابك الشخصي.\nيمكنك من خلال هذه اللوحة التحكم الكامل بإيقاف أو تشغيل الردود، أو استثناء أشخاص محددين من الرد التلقائي نهائياً.\n\nاختر من الأزرار أدناه:"
		
		keyboard := InlineKeyboardMarkup{
			InlineKeyboard: [][]InlineKeyboardButton{
				{
					{Text: "ايقاف/الرد 🛑", CallbackData: "stop_reply"},
					{Text: "تشغيل/الرد 🟢", CallbackData: "start_reply"},
				},
				{
					{Text: "استثناء/حسابي 👤", CallbackData: "exclude_account"},
				},
			},
		}
		sendMessage(botToken, chatID, txt, keyboard, "")
	} else if text == "/id" {
		sendMessage(botToken, chatID, fmt.Sprintf("الايدي الخاص بك هو:\n`%d`", msg.From.ID), nil, "")
	}
}

func handleBusinessMessage(botToken string, msg *BusinessMessage) {
	// 1. التحقق من حالة الإيقاف العام أو الرسائل الصادرة
	if isBotStopped || msg.IsOutgoing {
		return
	}

	// 2. التحقق من أن حساب الشخص المُراسل غير موجود في قائمة الاستثناء
	senderID := msg.From.ID
	if excludedIDs[senderID] {
		return
	}

	// 3. إرسال الرد التلقائي
	senderName := msg.From.FirstName
	if senderName == "" {
		senderName = "صديقي"
	}

	replyText := "مرحبا بك يا " + senderName + "\nانا غير متوفر الان يرجى ترك رسالتك\nوسأرد عليك قريبا"
	
	sendMessage(botToken, msg.Chat.ID, replyText, nil, msg.BusinessConnectionID)
}

// ==========================================
// دوال الاتصال بـ API تيليجرام
// ==========================================

func sendMessage(botToken string, chatID int64, text string, replyMarkup interface{}, bizConnID string) {
	apiURL := "https://api.telegram.org/bot" + botToken + "/sendMessage"
	payload := SendMessagePayload{
		ChatID:               chatID,
		Text:                 text,
		BusinessConnectionID: bizConnID,
		ReplyMarkup:          replyMarkup,
	}
	jsonPayload, _ := json.Marshal(payload)
	http.Post(apiURL, "application/json", bytes.NewBuffer(jsonPayload))
}

func deleteMessage(botToken string, chatID int64, messageID int) {
	apiURL := "https://api.telegram.org/bot" + botToken + "/deleteMessage"
	payload := DeleteMessagePayload{
		ChatID:    chatID,
		MessageID: messageID,
	}
	jsonPayload, _ := json.Marshal(payload)
	http.Post(apiURL, "application/json", bytes.NewBuffer(jsonPayload))
}

func answerCallbackQuery(botToken string, callbackQueryID string) {
	apiURL := "https://api.telegram.org/bot" + botToken + "/answerCallbackQuery"
	payload := map[string]string{"callback_query_id": callbackQueryID}
	jsonPayload, _ := json.Marshal(payload)
	http.Post(apiURL, "application/json", bytes.NewBuffer(jsonPayload))
}
