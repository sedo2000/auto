package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// --- هيكلة البيانات ---
type TelegramUpdate struct {
	Message         *Message         `json:"message"`
	CallbackQuery   *CallbackQuery   `json:"callback_query"`
	BusinessMessage *BusinessMessage `json:"business_message"`
}

type Message struct {
	MessageID int  `json:"message_id"`
	Chat      Chat `json:"chat"`
	From      User `json:"from"`
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
}

// --- متغيرات الحالة ---
var (
	autoReplyText      = "مرحباً! أنا غير متوفر حالياً، سأرد عليك قريباً."
	isStopped          = false
	workingHoursOnly   = false // ميزة 4: تفعيل ساعات العمل
	excludedIDs        = make(map[int64]bool)
	keywords           = map[string]string{"سعر": "الأسعار تبدأ من 50 دولار.", "موقع": "موقعنا في العراق."}
	
	// حالة انتظار المدخلات
	waitingForState = make(map[int64]string) 
	waitingKey      = make(map[int64]string)
)

func Handler(w http.ResponseWriter, r *http.Request) {
	var update TelegramUpdate
	json.NewDecoder(r.Body).Decode(&update)
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")

	if update.CallbackQuery != nil {
		handleCallbacks(botToken, update.CallbackQuery)
	} else if update.Message != nil {
		handleMessages(botToken, update.Message)
	} else if update.BusinessMessage != nil && !update.BusinessMessage.IsOutgoing {
		handleBusiness(botToken, update.BusinessMessage)
	}
	w.WriteHeader(http.StatusOK)
}

func handleBusiness(botToken string, msg *BusinessMessage) {
	// 1. التحقق من الاستثناءات
	if isStopped || excludedIDs[msg.From.ID] { return }

	// 2. ميزة ساعات العمل (من 22:00 إلى 08:00)
	if workingHoursOnly {
		hour := time.Now().Hour()
		if hour >= 8 && hour < 22 { return }
	}

	// 3. الردود الذكية (كلمات مفتاحية)
	reply := autoReplyText
	for k, v := range keywords {
		if strings.Contains(strings.ToLower(msg.Text), strings.ToLower(k)) {
			reply = v; break
		}
	}
	sendMessage(botToken, msg.Chat.ID, reply, nil, msg.BusinessConnectionID)
}

func handleMessages(botToken string, msg *Message) {
	chatID := msg.Chat.ID
	text := msg.Text

	// معالجة المدخلات من المستخدم
	if state, ok := waitingForState[chatID]; ok {
		switch state {
		case "edit_text":
			autoReplyText = text
			delete(waitingForState, chatID)
			sendMessage(botToken, chatID, "✅ تم تحديث النص بنجاح!", nil, "")
		case "add_key":
			waitingKey[chatID] = text
			waitingForState[chatID] = "add_val"
			sendMessage(botToken, chatID, "أدخل الرد الخاص بهذه الكلمة:", nil, "")
		case "add_val":
			key := waitingKey[chatID]
			keywords[key] = text
			delete(waitingForState, chatID)
			sendMessage(botToken, chatID, "✅ تمت إضافة الكلمة والرد بنجاح!", nil, "")
		case "exclude_id":
			id, _ := strconv.ParseInt(text, 10, 64)
			excludedIDs[id] = true
			delete(waitingForState, chatID)
			sendMessage(botToken, chatID, "✅ تم استثناء الحساب.", nil, "")
		}
		return
	}

	if text == "/start" {
		sendMainMenu(botToken, chatID)
	}
}

func handleCallbacks(botToken string, cb *CallbackQuery) {
	chatID := cb.Message.Chat.ID
	switch cb.Data {
	case "edit_text":
		waitingForState[chatID] = "edit_text"
		sendMessage(botToken, chatID, "أرسل النص الجديد للرد التلقائي الآن:", nil, "")
	case "add_key":
		waitingForState[chatID] = "add_key"
		sendMessage(botToken, chatID, "أرسل الكلمة المفتاحية:", nil, "")
	case "toggle_hours":
		workingHoursOnly = !workingHoursOnly
		sendMessage(botToken, chatID, fmt.Sprintf("⚙️ وضع العمل خارج الساعات: %v", workingHoursOnly), nil, "")
	case "list_excluded":
		txt := "قائمة المستثنين:\n"
		for id := range excludedIDs { txt += fmt.Sprintf("- %d\n", id) }
		sendMessage(botToken, chatID, txt, nil, "")
	case "toggle_bot":
		isStopped = !isStopped
		sendMessage(botToken, chatID, fmt.Sprintf("حالة البوت: %v", !isStopped), nil, "")
	}
}

func sendMainMenu(botToken string, chatID int64) {
	keyboard := map[string]interface{}{
		"inline_keyboard": [][]map[string]string{
			{{"text": "تعديل النص 📝", "callback_data": "edit_text"}, {"text": "كلمة مفتاحية ➕", "callback_data": "add_key"}},
			{{"text": "ساعات العمل ⏰", "callback_data": "toggle_hours"}, {"text": "المستثنون 👥", "callback_data": "list_excluded"}},
			{{"text": "تشغيل/إيقاف ⚙️", "callback_data": "toggle_bot"}},
		},
	}
	sendMessage(botToken, chatID, "لوحة التحكم الرئيسية:", keyboard, "")
}

func sendMessage(botToken string, chatID int64, text string, replyMarkup interface{}, bizID string) {
	payload := map[string]interface{}{
		"chat_id": chatID, "text": text, "business_connection_id": bizID, "reply_markup": replyMarkup,
	}
	b, _ := json.Marshal(payload)
	http.Post("https://api.telegram.org/bot"+botToken+"/sendMessage", "application/json", bytes.NewBuffer(b))
}
