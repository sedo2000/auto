package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var quotes = []string{
	"قاوم ما تكره لتصل الى ما تحب",
	"الحرب بين أنت ضد أنت",
	"لا تسألني من أنا",
	"أبنِ نفسك بنفسك لنفسك",
	"ميخالف",
	"حتى لو متأخر تگدر..!",
	"من يعيش في خوف لن يكون حراً ابداً",
	"لا أبرح حتى أبلغ",
	"لا أجدني بينهم",
	"كل شيء يريدك عندما لاتريد شيئاً",
	"أنه مبرمج فحسب",
	"أنا لا افكر فيك ابداً",
	"المرء نتاج خلواته",
	"لا مزيد من الأصدقاء المزيفين",
}

type BotConfig struct {
	IsStopped   bool           `json:"is_stopped"`
	AutoReply   string         `json:"auto_reply"`
	Excluded    []int64        `json:"excluded"`
	State       string         `json:"state"`
	LastReplies map[string]int `json:"last_replies"`
}

type TelegramUpdate struct {
	Message       *Message       `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
	BusinessMessage *struct {
		MessageID int `json:"message_id"`
		Chat      struct {
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

type BusinessConnectionResponse struct {
	Ok     bool `json:"ok"`
	Result struct {
		User struct {
			ID int64 `json:"id"`
		} `json:"user"`
		UserChatID int64 `json:"user_chat_id"`
	} `json:"result"`
}

func Handler(w http.ResponseWriter, r *http.Request) {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var update TelegramUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 1. التعامل مع الأزرار الشفافة
	if update.CallbackQuery != nil {
		cb := update.CallbackQuery
		answerCallback(botToken, cb.ID)

		if cb.Data == "change_quote" {
			rand.Seed(time.Now().UnixNano())
			newQuote := quotes[rand.Intn(len(quotes))]
			updateButtonQuote(botToken, cb.Message.Chat.ID, cb.Message.MessageID, newQuote)
			w.WriteHeader(http.StatusOK)
			return
		}

		deleteMessage(botToken, cb.Message.Chat.ID, cb.Message.MessageID)
		adminID := cb.From.ID
		config, msgID := getConfig(botToken, adminID)

		switch cb.Data {
		case "main_menu":
			config.State = ""
			saveConfig(botToken, adminID, config, msgID)
			sendMenu(botToken, adminID, "القائمة الرئيسية 🤖:")
		case "stop":
			config.IsStopped = true
			config.State = ""
			saveConfig(botToken, adminID, config, msgID)
			sendMenu(botToken, adminID, "🛑 تم إيقاف الرد التلقائي بنجاح.")
		case "start":
			config.IsStopped = false
			config.State = ""
			saveConfig(botToken, adminID, config, msgID)
			sendMenu(botToken, adminID, "🟢 تم تشغيل الرد التلقائي بنجاح.")
		case "edit_text":
			config.State = "waiting_text"
			saveConfig(botToken, adminID, config, msgID)
			sendSubMenu(botToken, adminID, "📝 أرسل الآن نص الرد التلقائي الجديد:")
		case "exclude":
			config.State = "waiting_id"
			saveConfig(botToken, adminID, config, msgID)
			txt := "👤 أرسل ايدي الحساب المراد استثناؤه الآن:"
			sendSubMenu(botToken, adminID, txt)
		case "list_excluded":
			txt := "📋 **قائمة الحسابات المستثناة:**\n"
			if len(config.Excluded) == 0 {
				txt += "لا يوجد حسابات مستثناة حالياً."
			} else {
				for _, id := range config.Excluded {
					txt += fmt.Sprintf("- `%d`\n", id)
				}
			}
			sendSubMenu(botToken, adminID, txt)
		case "clear_excluded":
			config.Excluded = []int64{}
			saveConfig(botToken, adminID, config, msgID)
			sendMenu(botToken, adminID, "🧹 تم مسح جميع الاستثناءات بنجاح.")
		}

		w.WriteHeader(http.StatusOK)
		return
	}

	// 2. محادثة التحكم
	if update.Message != nil {
		msg := update.Message
		chatID := msg.Chat.ID

		config, msgID := getConfig(botToken, chatID)

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

		if config.State == "waiting_text" {
			config.AutoReply = msg.Text
			config.State = ""
			saveConfig(botToken, chatID, config, msgID)
			sendMenu(botToken, chatID, "✅ تم حفظ نص الرد التلقائي الجديد بنجاح!")
		} else if config.State == "waiting_id" {
			id, err := strconv.ParseInt(strings.TrimSpace(msg.Text), 10, 64)
			if err == nil {
				alreadyExists := false
				for _, ex := range config.Excluded {
					if ex == id {
						alreadyExists = true
						break
					}
				}
				if !alreadyExists {
					config.Excluded = append(config.Excluded, id)
				}
				config.State = ""
				saveConfig(botToken, chatID, config, msgID)
				sendMenu(botToken, chatID, fmt.Sprintf("✅ تم إضافة الايدي `%d` إلى قائمة الاستثناء.", id))
			} else {
				sendSubMenu(botToken, chatID, "❌ أرقام فقط! أرسل الايدي بشكل صحيح.")
			}
		}

		w.WriteHeader(http.StatusOK)
		return
	}

	// 3. رسائل العملاء (Business Messages)
	if update.BusinessMessage != nil {
		msg := update.BusinessMessage

		if msg.IsOutgoing {
			w.WriteHeader(http.StatusOK)
			return
		}

		adminID := getAdminIDFromBusinessConn(botToken, msg.BusinessConnectionID)
		if adminID == 0 {
			w.WriteHeader(http.StatusOK)
			return
		}

		config, configMsgID := getConfig(botToken, adminID)

		if config.IsStopped {
			w.WriteHeader(http.StatusOK)
			return
		}

		senderID := msg.From.ID
		customerChatID := msg.Chat.ID
		for _, exID := range config.Excluded {
			if exID == senderID || exID == customerChatID {
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		// مسح رسالة الترحيب والزر السابقة الخاصة بالبوت لهذا العميل
		custKey := strconv.FormatInt(customerChatID, 10)
		if prevMsgID, exists := config.LastReplies[custKey]; exists && prevMsgID > 0 {
			deleteMessage(botToken, customerChatID, prevMsgID)
		}

		// إدراج اسم العميل تلقائياً في النص
		customerName := msg.From.FirstName
		if customerName == "" {
			customerName = "صديقي"
		}

		var replyText string
		if config.AutoReply == "" {
			replyText = "أهلاً بك يا " + customerName + " 🌸\nأنا غير متوفر الآن، اترك رسالتك وسأرد عليك قريباً."
		} else {
			// إذا يحتوي النص على القالب يتم استبداله، وإلا يضاف اسم العميل في بداية النص
			if strings.Contains(config.AutoReply, "{name}") || strings.Contains(config.AutoReply, "{الاسم}") {
				replyText = strings.ReplaceAll(config.AutoReply, "{name}", customerName)
				replyText = strings.ReplaceAll(replyText, "{الاسم}", customerName)
			} else {
				replyText = "أهلاً بك يا " + customerName + " 🌸\n" + config.AutoReply
			}
		}

		// إرسال الرد الجديد مع الزر الشفاف وحفظ الايدي للمسح في المرة القادمة
		newMsgID := sendBusinessReplyWithQuoteButton(botToken, customerChatID, replyText, msg.BusinessConnectionID)
		if newMsgID != 0 {
			if config.LastReplies == nil {
				config.LastReplies = make(map[string]int)
			}
			config.LastReplies[custKey] = newMsgID
			saveConfig(botToken, adminID, config, configMsgID)
		}
	}

	w.WriteHeader(http.StatusOK)
}

func getAdminIDFromBusinessConn(token string, connID string) int64 {
	if connID == "" {
		return 0
	}
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getBusinessConnection?business_connection_id=%s", token, connID)
	resp, err := http.Get(url)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	var res BusinessConnectionResponse
	json.NewDecoder(resp.Body).Decode(&res)
	if res.Result.UserChatID != 0 {
		return res.Result.UserChatID
	}
	return res.Result.User.ID
}

func getConfig(token string, chatID int64) (BotConfig, int) {
	defaultCfg := BotConfig{
		IsStopped:   false,
		AutoReply:   "",
		Excluded:    []int64{},
		State:       "",
		LastReplies: make(map[string]int),
	}

	if chatID == 0 {
		return defaultCfg, 0
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/getChat?chat_id=%d", token, chatID)
	resp, err := http.Get(url)
	if err != nil {
		return defaultCfg, 0
	}
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
			if cfg.LastReplies == nil {
				cfg.LastReplies = make(map[string]int)
			}
			return cfg, res.Result.PinnedMessage.MessageID
		}
	}

	return defaultCfg, 0
}

func saveConfig(token string, chatID int64, cfg BotConfig, pinnedMsgID int) {
	if chatID == 0 {
		return
	}
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

func sendSubMenu(token string, chatID int64, text string) {
	keyboard := map[string]interface{}{
		"inline_keyboard": [][]map[string]string{
			{{"text": "🔙 رجوع", "callback_data": "main_menu"}},
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

func sendBusinessReplyWithQuoteButton(token string, chatID int64, text, bizID string) int {
	rand.Seed(time.Now().UnixNano())
	initialQuote := quotes[rand.Intn(len(quotes))]

	keyboard := map[string]interface{}{
		"inline_keyboard": [][]map[string]string{
			{{"text": "✨ " + initialQuote, "callback_data": "change_quote"}},
		},
	}

	payload := map[string]interface{}{
		"chat_id":                chatID,
		"text":                   text,
		"business_connection_id": bizID,
		"reply_markup":           keyboard,
	}
	b, _ := json.Marshal(payload)
	resp, err := http.Post("https://api.telegram.org/bot"+token+"/sendMessage", "application/json", bytes.NewBuffer(b))
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	var res struct {
		Result struct {
			MessageID int `json:"message_id"`
		} `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&res)
	return res.Result.MessageID
}

func updateButtonQuote(token string, chatID int64, msgID int, newQuote string) {
	keyboard := map[string]interface{}{
		"inline_keyboard": [][]map[string]string{
			{{"text": "✨ " + newQuote, "callback_data": "change_quote"}},
		},
	}

	payload := map[string]interface{}{
		"chat_id":      chatID,
		"message_id":   msgID,
		"reply_markup": keyboard,
	}
	b, _ := json.Marshal(payload)
	http.Post("https://api.telegram.org/bot"+token+"/editMessageReplyMarkup", "application/json", bytes.NewBuffer(b))
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
