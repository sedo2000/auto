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

// قائمة الاقتباسات
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
	Language  string  `json:"language"` // "ar" or "en"
	IsStopped bool    `json:"is_stopped"`
	AutoReply string  `json:"auto_reply"`
	Excluded  []int64 `json:"excluded"`
	State     string  `json:"state"`
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

	// 1. معالجة الأزرار الشفافة (Callback Queries)
	if update.CallbackQuery != nil {
		cb := update.CallbackQuery
		answerCallback(botToken, cb.ID)

		// تغيير الاقتباس للعميل عند الضغط على الزر الشفاف
		if cb.Data == "change_quote" {
			rand.Seed(time.Now().UnixNano())
			newQuote := quotes[rand.Intn(len(quotes))]
			updateButtonQuote(botToken, cb.Message.Chat.ID, cb.Message.MessageID, newQuote)
			w.WriteHeader(http.StatusOK)
			return
		}

		adminID := cb.From.ID
		config, msgID := getConfig(botToken, adminID)

		// التعامل مع اختيار اللغة
		if cb.Data == "lang_ar" || cb.Data == "lang_en" {
			deleteMessage(botToken, cb.Message.Chat.ID, cb.Message.MessageID)
			if cb.Data == "lang_ar" {
				config.Language = "ar"
			} else {
				config.Language = "en"
			}
			config.State = ""
			saveConfig(botToken, adminID, config, msgID)
			sendMenu(botToken, adminID, getText(config.Language, "welcome_menu"), config.Language)
			w.WriteHeader(http.StatusOK)
			return
		}

		deleteMessage(botToken, cb.Message.Chat.ID, cb.Message.MessageID)

		switch cb.Data {
		case "change_lang":
			sendLangMenu(botToken, adminID)
		case "main_menu":
			config.State = ""
			saveConfig(botToken, adminID, config, msgID)
			sendMenu(botToken, adminID, getText(config.Language, "main_menu"), config.Language)
		case "stop":
			config.IsStopped = true
			config.State = ""
			saveConfig(botToken, adminID, config, msgID)
			sendMenu(botToken, adminID, getText(config.Language, "stopped_success"), config.Language)
		case "start":
			config.IsStopped = false
			config.State = ""
			saveConfig(botToken, adminID, config, msgID)
			sendMenu(botToken, adminID, getText(config.Language, "started_success"), config.Language)
		case "edit_text":
			config.State = "waiting_text"
			saveConfig(botToken, adminID, config, msgID)
			sendSubMenu(botToken, adminID, getText(config.Language, "prompt_edit_text"), config.Language)
		case "exclude":
			config.State = "waiting_id"
			saveConfig(botToken, adminID, config, msgID)
			sendSubMenu(botToken, adminID, getText(config.Language, "prompt_exclude_id"), config.Language)
		case "list_excluded":
			txt := getText(config.Language, "excluded_list_header") + "\n"
			if len(config.Excluded) == 0 {
				txt += getText(config.Language, "no_excluded")
			} else {
				for _, id := range config.Excluded {
					txt += fmt.Sprintf("- `%d`\n", id)
				}
			}
			sendSubMenu(botToken, adminID, txt, config.Language)
		case "clear_excluded":
			config.Excluded = []int64{}
			saveConfig(botToken, adminID, config, msgID)
			sendMenu(botToken, adminID, getText(config.Language, "cleared_excluded"), config.Language)
		}

		w.WriteHeader(http.StatusOK)
		return
	}

	// 2. معالجة محادثة التحكم للآدمن
	if update.Message != nil {
		msg := update.Message
		chatID := msg.Chat.ID

		config, msgID := getConfig(botToken, chatID)

		if msg.Text == "/start" {
			sendLangMenu(botToken, chatID)
			w.WriteHeader(http.StatusOK)
			return
		}

		if msg.Text == "/id" {
			sendMessage(botToken, chatID, fmt.Sprintf("Your ID / الايدي الخاص بك:\n`%d`", msg.From.ID))
			w.WriteHeader(http.StatusOK)
			return
		}

		if config.Language == "" {
			config.Language = "ar" // لغة افتراضية
		}

		if config.State == "waiting_text" {
			config.AutoReply = msg.Text
			config.State = ""
			saveConfig(botToken, chatID, config, msgID)
			sendMenu(botToken, chatID, getText(config.Language, "saved_text_success"), config.Language)
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
				sendMenu(botToken, chatID, fmt.Sprintf(getText(config.Language, "added_id_success"), id), config.Language)
			} else {
				sendSubMenu(botToken, chatID, getText(config.Language, "invalid_id_error"), config.Language)
			}
		}

		w.WriteHeader(http.StatusOK)
		return
	}

	// 3. معالجة رسائل العملاء (Business Messages)
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

		config, _ := getConfig(botToken, adminID)

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

		// حذف متسلسل لرسائل البوت السابقة للعميل
		currentMsgID := msg.MessageID
		if currentMsgID > 1 {
			for id := currentMsgID - 1; id >= currentMsgID-6 && id > 0; id-- {
				deleteMessage(botToken, customerChatID, id)
			}
		}

		// تجهيز اسم العميل
		customerName := msg.From.FirstName
		if customerName == "" {
			if config.Language == "en" {
				customerName = "Friend"
			} else {
				customerName = "صديقي"
			}
		}

		var replyText string
		if config.AutoReply == "" {
			if config.Language == "en" {
				replyText = "Welcome " + customerName + " 🌸\nI am currently unavailable, please leave your message and I will reply soon."
			} else {
				replyText = "أهلاً بك يا " + customerName + " 🌸\nأنا غير متوفر الآن، اترك رسالتك وسأرد عليك قريباً."
			}
		} else {
			if strings.Contains(config.AutoReply, "{name}") || strings.Contains(config.AutoReply, "{الاسم}") {
				replyText = strings.ReplaceAll(config.AutoReply, "{name}", customerName)
				replyText = strings.ReplaceAll(replyText, "{الاسم}", customerName)
			} else {
				if config.Language == "en" {
					replyText = "Welcome " + customerName + " 🌸\n" + config.AutoReply
				} else {
					replyText = "أهلاً بك يا " + customerName + " 🌸\n" + config.AutoReply
				}
			}
		}

		// إرسال الرد الجديد مع زر الاقتباس الشفاف
		sendBusinessReplyWithQuoteButton(botToken, customerChatID, replyText, msg.BusinessConnectionID)
	}

	w.WriteHeader(http.StatusOK)
}

// --- قاموس النصوص واللغات ---

func getText(lang string, key string) string {
	texts := map[string]map[string]string{
		"ar": {
			"select_lang":          "🌐 يرجى اختيار لغة التحكم / Please select a language:",
			"welcome_menu":          "تم تحديد اللغة بنجاح! 🇸🇦\nأهلاً بك في لوحة تحكم البوت 🤖\nاختر من الأزرار أدناه للتحكم الكامل:",
			"main_menu":             "القائمة الرئيسية 🤖:",
			"stopped_success":       "🛑 تم إيقاف الرد التلقائي بنجاح.",
			"started_success":       "🟢 تم تشغيل الرد التلقائي بنجاح.",
			"prompt_edit_text":     "📝 أرسل الآن نص الرد التلقائي الجديد (يمكنك استخدام {name} لذكر اسم العميل تلقائياً):",
			"prompt_exclude_id":    "👤 أرسل ايدي الحساب المراد استثناؤه الآن:",
			"excluded_list_header": "📋 **قائمة الحسابات المستثناة:**",
			"no_excluded":          "لا يوجد حسابات مستثناة حالياً.",
			"cleared_excluded":     "🧹 تم مسح جميع الاستثناءات بنجاح.",
			"saved_text_success":   "✅ تم حفظ نص الرد التلقائي الجديد بنجاح!",
			"added_id_success":     "✅ تم إضافة الايدي `%d` إلى قائمة الاستثناء.",
			"invalid_id_error":     "❌ أرقام فقط! أرسل الايدي بشكل صحيح.",
		},
		"en": {
			"select_lang":          "🌐 Please select a language / يرجى اختيار اللغة:",
			"welcome_menu":          "Language set successfully! 🇬🇧\nWelcome to the Bot Control Panel 🤖\nChoose from the options below:",
			"main_menu":             "Main Menu 🤖:",
			"stopped_success":       "🛑 Auto-reply has been paused.",
			"started_success":       "🟢 Auto-reply has been activated.",
			"prompt_edit_text":     "📝 Send the new auto-reply text now (you can use {name} to insert the client's name):",
			"prompt_exclude_id":    "👤 Send the ID of the user you want to exclude now:",
			"excluded_list_header": "📋 **Excluded Accounts List:**",
			"no_excluded":          "No excluded accounts at the moment.",
			"cleared_excluded":     "🧹 All exclusions cleared successfully.",
			"saved_text_success":   "✅ New auto-reply text saved successfully!",
			"added_id_success":     "✅ User ID `%d` added to exclusion list.",
			"invalid_id_error":     "❌ Numbers only! Send a valid User ID.",
		},
	}

	if lMap, exists := texts[lang]; exists {
		if val, ok := lMap[key]; ok {
			return val
		}
	}
	return texts["ar"][key]
}

// --- دوال القوائم والمواجهة ---

func sendLangMenu(token string, chatID int64) {
	keyboard := map[string]interface{}{
		"inline_keyboard": [][]map[string]string{
			{
				{"text": "🇸🇦 العربية", "callback_data": "lang_ar"},
				{"text": "🇬🇧 English", "callback_data": "lang_en"},
			},
		},
	}

	payload := map[string]interface{}{
		"chat_id":      chatID,
		"text":         getText("ar", "select_lang"),
		"reply_markup": keyboard,
	}
	b, _ := json.Marshal(payload)
	http.Post("https://api.telegram.org/bot"+token+"/sendMessage", "application/json", bytes.NewBuffer(b))
}

func sendMenu(token string, chatID int64, text string, lang string) {
	btnStop := "🛑 إيقاف الرد"
	btnStart := "🟢 تشغيل الرد"
	btnEditText := "📝 تعديل نص الرد"
	btnExclude := "👤 استثناء حساب"
	btnList := "📋 عرض المستثنين"
	btnClear := "🧹 مسح المستثنين"
	btnLang := "🌐 تغيير اللغة / Language"

	if lang == "en" {
		btnStop = "🛑 Pause Auto-Reply"
		btnStart = "🟢 Start Auto-Reply"
		btnEditText = "📝 Edit Reply Text"
		btnExclude = "👤 Exclude User"
		btnList = "📋 View Excluded List"
		btnClear = "🧹 Clear Excluded"
	}

	keyboard := map[string]interface{}{
		"inline_keyboard": [][]map[string]string{
			{{"text": btnStop, "callback_data": "stop"}, {"text": btnStart, "callback_data": "start"}},
			{{"text": btnEditText, "callback_data": "edit_text"}},
			{{"text": btnExclude, "callback_data": "exclude"}, {"text": btnList, "callback_data": "list_excluded"}},
			{{"text": btnClear, "callback_data": "clear_excluded"}},
			{{"text": btnLang, "callback_data": "change_lang"}},
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

func sendSubMenu(token string, chatID int64, text string, lang string) {
	btnBack := "🔙 رجوع"
	if lang == "en" {
		btnBack = "🔙 Back"
	}

	keyboard := map[string]interface{}{
		"inline_keyboard": [][]map[string]string{
			{{"text": btnBack, "callback_data": "main_menu"}},
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
		Language:  "ar",
		IsStopped: false,
		AutoReply: "",
		Excluded:  []int64{},
		State:     "",
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

func sendMessage(token string, chatID int64, text string) {
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}
	b, _ := json.Marshal(payload)
	http.Post("https://api.telegram.org/bot"+token+"/sendMessage", "application/json", bytes.NewBuffer(b))
}

func sendBusinessReplyWithQuoteButton(token string, chatID int64, text, bizID string) {
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
	http.Post("https://api.telegram.org/bot"+token+"/sendMessage", "application/json", bytes.NewBuffer(b))
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
