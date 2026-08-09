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

// هيكلة الإعدادات التي يتم حفظها في الرسالة المثبتة
type BotConfig struct {
	IsStopped bool    `json:"is_stopped"`
	AutoReply string  `json:"auto_reply"`
	Excluded  []int64 `json:"excluded"`
	State     string  `json:"state"` // "waiting_text" | "waiting_id"
}

type TelegramUpdate struct {
	Message       *Message       `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
	BusinessMessage *struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		From struct {
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
		} `json:"from"`
		Text                 string `json:"text"`
		IsOutgoing           bool   `json:"is_outgoing"`
		BusinessConnectionID string `json:"business_connection_id"`
	} `json:"business_message"`
}

type Message struct {
	MessageID int `json:"message_id"`
	Chat      struct {
		ID int64 `json:"id"`
	} `json:"chat"`
	From struct {
		ID int64 `json:"id"`
	} `json:"from"`
	Text string `json:"text"`
}

type CallbackQuery struct {
	ID      string  `json:"id"`
	Message Message `json:"message"`
	Data    string  `json:"data"`
	From    struct {
		ID int64 `json:"id"`
	} `json:"from"`
}

func Handler(w http.ResponseWriter, r *http.Request) {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	adminIDStr := os.Getenv("ADMIN_ID") // ايدي حسابك في vercel (اختياري)

	var update TelegramUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	var adminID int64
	if adminIDStr != "" {
		adminID, _ = strconv.ParseInt(adminIDStr, 10, 64)
	}

	// 1. التعامل مع الأزرار الشفافة
	if update.CallbackQuery != nil {
		cb := update.CallbackQuery
		answerCallback(botToken, cb.ID)
		deleteMessage(botToken, cb.Message.Chat.ID, cb.Message.MessageID)

		config, msgID := getConfig(botToken, cb.From.ID)

		switch cb.Data {
		case "stop":
			config.IsStopped = true
			config.State = ""
			saveConfig(botToken, cb.From.ID, config, msgID)
			sendMenu(botToken, cb.From.ID, "🛑 تم إيقاف الرد التلقائي.")
		case "start":
			config.IsStopped = false
			config.State = ""
			saveConfig(botToken, cb.From.ID, config, msgID)
			sendMenu(botToken, cb.From.ID, "🟢 تم تشغيل الرد التلقائي.")
		case "edit_text":
			config.State = "waiting_text"
			saveConfig(botToken, cb.From.ID, config, msgID)
			sendMessage(botToken, cb.From.ID, "📝 أرسل الآن نص الرد التلقائي الجديد الذي تريده:")
		case "exclude":
			config.State = "waiting_id"
			saveConfig(botToken, cb.From.ID, config, msgID)
			txt := "👤 أرسل ايدي الحساب المراد استثناؤه الآن:\n(إذا كنت لا تعرف الايدي، اضغط /id في محادثة الشخص وانسخ الرقم)"
			sendMessage(botToken, cb.From.ID, txt)
		case "list_excluded":
			txt := "📋 **قائمة الحسابات المستثناة:**\n"
			if len(config.Excluded) == 0 {
				txt += "لا يوجد حسابات مستثناة حالياً."
			} else {
				for _, id := range config.Excluded {
					txt += fmt.Sprintf("- `%d`\n", id)
				}
			}
			sendMenu(botToken, cb.From.ID, txt)
		case "clear_excluded":
			config.Excluded = []int64{}
			saveConfig(botToken, cb.From.ID, config, msgID)
			sendMenu(botToken, cb.From.ID, "🧹 تم مسح جميع الاستثناءات بنجاح.")
		}

		w.WriteHeader(http.StatusOK)
		return
	}

	// 2. التعامل مع الأوامر والنصوص المباشرة في محادثة البوت
	if update.Message != nil {
		msg := update.Message
		chatID := msg.Chat.ID

		if msg.Text == "/start" {
			sendMenu(botToken, chatID, "أهلاً بك في لوحة تحكم البوت 🤖\nاختر من الأزرار أدناه للتحكم الكامل:")
			w.WriteHeader(http.StatusOK)
			return
		}

		if msg.Text == "/id" {
			sendMessage(botToken, chatID, fmt.Sprintf("الايدي الخاص بك هو:\n`%d`", msg.From.ID))
			w.WriteHeader(http.StatusOK)
			return
		}

		// فحص حالة انتظار المدخلات (تعديل نص أو إضافة ID)
		config, msgID := getConfig(botToken, chatID)
		if config.State == "waiting_text" {
			config.AutoReply = msg.Text
			config.State = ""
			saveConfig(botToken, chatID, config, msgID)
			sendMenu(botToken, chatID, "✅ تم حفظ نص الرد التلقائي الجديد بنجاح!")
		} else if config.State == "waiting_id" {
			id, err := strconv.ParseInt(strings.TrimSpace(msg.Text), 10, 64)
			if err == nil {
				config.Excluded = append(config.Excluded, id)
				config.State = ""
				saveConfig(botToken, chatID, config, msgID)
				sendMenu(botToken, chatID, fmt.Sprintf("✅ تم إضافة الايدي `%d` إلى قائمة الاستثناء.", id))
			} else {
				sendMessage(botToken, chatID, "❌ أرقام فقط! أرسل الايدي بشكل صحيح.")
			}
		}

		w.WriteHeader(http.StatusOK)
		return
	}

	// 3. الرد التلقائي على العملاء (Business Messages)
	if update.BusinessMessage != nil && !update.BusinessMessage.IsOutgoing {
		msg := update.BusinessMessage
		
		targetAdminID := adminID
		if targetAdminID == 0 {
			targetAdminID = msg.Chat.ID
		}

		config, _ := getConfig(botToken, targetAdminID)

		if config.IsStopped {
			w.WriteHeader(http.StatusOK)
			return
		}

		// فحص قائمة الاستثناء
		for _, exID := range config.Excluded {
			if exID == msg.From.ID {
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		replyText := config.AutoReply
		if replyText == "" {
			name := msg.From.FirstName
			if name == "" { name = "صديقي" }
			replyText = "مرحبا بك يا " + name + "\nانا غير متوفر الان يرجى ترك رسالتك\nوسأرد عليك قريبا"
		}

		sendBusinessReply(botToken, msg.Chat.ID, replyText, msg.BusinessConnectionID)
	}

	w.WriteHeader(http.StatusOK)
}

// --- دوال إدارة الإعدادات عبر الرسالة المثبتة ---

func getConfig(token string, chatID int64) (BotConfig, int) {
	defaultCfg := BotConfig{
		IsStopped: false,
		AutoReply: "",
		Excluded:  []int64{},
		State:     "",
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/getChat?chat_id=%d", token, chatID)
	resp, err := http.Get(url)
	if err != nil { return defaultCfg, 0 }
	defer resp.Body.Close()

	var res struct {
		Result struct {
			PinnedMessage struct {
				MessageID int    `json:"message_id"`
				Text      string `json:"text"`
			} `json:"pinned_message"`
		} `json:"result"`
	}

	json.NewDecoder(resp.Body).Decode(&res)

	if res.Result.PinnedMessage.MessageID != 0 {
		var cfg BotConfig
		if err := json.Unmarshal([]byte(res.Result.PinnedMessage.Text), &cfg); err == nil {
			return cfg, res.Result.PinnedMessage.MessageID
		}
	}

	return defaultCfg, 0
}

func saveConfig(token string, chatID int64, cfg BotConfig, pinnedMsgID int) {
	b, _ := json.Marshal(cfg)
	cfgText := string(b)

	if pinnedMsgID > 0 {
		url := fmt.Sprintf("https://api.telegram.org/bot%s/editMessageText", token)
		payload := map[string]interface{}{
			"chat_id":    chatID,
			"message_id": pinnedMsgID,
			"text":       cfgText,
		}
		pBytes, _ := json.Marshal(payload)
		http.Post(url, "application/json", bytes.NewBuffer(pBytes))
	} else {
		url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
		payload := map[string]interface{}{
			"chat_id": chatID,
			"text":    cfgText,
		}
		pBytes, _ := json.Marshal(payload)
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(pBytes))
		if err == nil {
			var res struct {
				Result struct {
					MessageID int `json:"message_id"`
				} `json:"result"`
			}
			json.NewDecoder(resp.Body).Decode(&res)
			if res.Result.MessageID != 0 {
				pinUrl := fmt.Sprintf("https://api.telegram.org/bot%s/pinChatMessage", token)
				pinPayload := map[string]interface{}{
					"chat_id":              chatID,
					"message_id":           res.Result.MessageID,
					"disable_notification": true,
				}
				pPinBytes, _ := json.Marshal(pinPayload)
				http.Post(pinUrl, "application/json", bytes.NewBuffer(pPinBytes))
			}
		}
	}
}

// --- دوال الواجهة والأزرار ---

func sendMenu(token string, chatID int64, text string) {
	keyboard := map[string]interface{}{
		"inline_keyboard": [][]map[string]string{
			{{"text": "🛑 إيقاف الرد", "callback_data": "stop"}, {"text": "🟢 تشغيل الرد", "callback_data": "start"}},
			{{"text": "📝 تعديل نص الرد", "callback_data": "edit_text"}},
			{{"text": "👤 استثناء حساب", "callback_data": "exclude"}, {"text": "📋 عرض المستثنين", "callback_data": "list_excluded"}},
			{{"text": "🧹 مسح المستثنين", "callback_data": "clear_excluded"}},
		},
	}

	payload := map[string]interface{}{
		"chat_id":      chatID,
		"text":         text,
		"parse_mode":   "Markdown",
		"reply_markup": keyboard,
	}
	b, _ := json.Marshal(payload)
	http.Post("https://api.telegram.org/bot"+token+"/sendMessage", "application/json", bytes.NewBuffer(b))
}

func sendMessage(token string, chatID int64, text string) {
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}
	b, _ := json.Marshal(payload)
	http.Post("https://api.telegram.org/bot"+token+"/sendMessage", "application/json", bytes.NewBuffer(b))
}

func sendBusinessReply(token string, chatID int64, text, bizID string) {
	payload := map[string]interface{}{
		"chat_id":                chatID,
		"text":                   text,
		"business_connection_id": bizID,
	}
	b, _ := json.Marshal(payload)
	http.Post("https://api.telegram.org/bot"+token+"/sendMessage", "application/json", bytes.NewBuffer(b))
}

func deleteMessage(token string, chatID int64, msgID int) {
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": msgID,
	}
	b, _ := json.Marshal(payload)
	http.Post("https://api.telegram.org/bot"+token+"/deleteMessage", "application/json", bytes.NewBuffer(b))
}

func answerCallback(token, callbackID string) {
	payload := map[string]string{"callback_query_id": callbackID}
	b, _ := json.Marshal(payload)
	http.Post("https://api.telegram.org/bot"+token+"/answerCallbackQuery", "application/json", bytes.NewBuffer(b))
}
