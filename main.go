package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// عميل HTTP مشترك مع timeout حتى لا يتجمد الطلب لو تأخر تليجرام
var httpClient = &http.Client{Timeout: 8 * time.Second}

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
	IsStopped      bool    `json:"is_stopped"`
	AutoReply      string  `json:"auto_reply"`
	Excluded       []int64 `json:"excluded"`
	State          string  `json:"state"`
	BusinessConnID string  `json:"business_conn_id"`
}

type TelegramUpdate struct {
	Message         *Message       `json:"message"`
	CallbackQuery   *CallbackQuery `json:"callback_query"`
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
	// يصل هذا التحديث عند تفعيل أو تعديل أو إيقاف ربط حساب تجاري بالبوت
	BusinessConnection *struct {
		ID   string `json:"id"`
		User struct {
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Username  string `json:"username"`
		} `json:"user"`
		UserChatID int64 `json:"user_chat_id"`
		Date       int64 `json:"date"`
		IsEnabled  bool  `json:"is_enabled"`
	} `json:"business_connection"`
}

type PhotoSize struct {
	FileID string `json:"file_id"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type Message struct {
	MessageID int `json:"message_id"`
	Chat      struct {
		ID int64 `json:"id"`
	} `json:"chat"`
	From struct {
		ID int64 `json:"id"`
	} `json:"from"`
	Text  string      `json:"text"`
	Photo []PhotoSize `json:"photo"`
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

	// --- فحص أمني: التحقق من Secret Token الخاص بالـ Webhook ---
	// اضبطه عند استدعاء setWebhook بالبراميتر secret_token، وخزّنه بنفس القيمة هنا.
	if secret := os.Getenv("TELEGRAM_WEBHOOK_SECRET"); secret != "" {
		if r.Header.Get("X-Telegram-Bot-Api-Secret-Token") != secret {
			log.Println("رفض طلب: secret token غير مطابق")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	var update TelegramUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		log.Println("خطأ في قراءة التحديث:", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	// 1. معالجة الضغط على الأزرار الشفافة
	if update.CallbackQuery != nil {
		cb := update.CallbackQuery
		answerCallback(botToken, cb.ID)

		if cb.Data == "change_quote" {
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
		case "profile_menu":
			config.State = ""
			saveConfig(botToken, adminID, config, msgID)
			sendProfileMenu(botToken, adminID, "🧑 إدارة الملف الشخصي - اختر ما تريد تعديله:")
		case "edit_first_name":
			if config.BusinessConnID == "" {
				sendProfileMenu(botToken, adminID, "❌ لم يتم ربط حساب تجاري بعد بالبوت.")
				break
			}
			config.State = "waiting_first_name"
			saveConfig(botToken, adminID, config, msgID)
			sendSubMenu(botToken, adminID, "✏️ أرسل الآن الاسم الأول الجديد (والاسم الأخير بعده بمسافة، اختياري):")
		case "edit_bio":
			if config.BusinessConnID == "" {
				sendProfileMenu(botToken, adminID, "❌ لم يتم ربط حساب تجاري بعد بالبوت.")
				break
			}
			config.State = "waiting_bio"
			saveConfig(botToken, adminID, config, msgID)
			sendSubMenu(botToken, adminID, "📝 أرسل الآن النبذة الجديدة (حد أقصى 140 حرف):")
		case "edit_username":
			if config.BusinessConnID == "" {
				sendProfileMenu(botToken, adminID, "❌ لم يتم ربط حساب تجاري بعد بالبوت.")
				break
			}
			config.State = "waiting_username"
			saveConfig(botToken, adminID, config, msgID)
			sendSubMenu(botToken, adminID, "🔗 أرسل الآن اسم المستخدم الجديد (بدون @):")
		case "edit_photo":
			if config.BusinessConnID == "" {
				sendProfileMenu(botToken, adminID, "❌ لم يتم ربط حساب تجاري بعد بالبوت.")
				break
			}
			config.State = "waiting_photo"
			saveConfig(botToken, adminID, config, msgID)
			sendSubMenu(botToken, adminID, "🖼️ أرسل الآن الصورة الجديدة لملفك الشخصي:")
		case "post_story":
			if config.BusinessConnID == "" {
				sendMenu(botToken, adminID, "❌ لم يتم ربط حساب تجاري بعد بالبوت.")
				break
			}
			config.State = "waiting_story"
			saveConfig(botToken, adminID, config, msgID)
			sendSubMenu(botToken, adminID, "📖 أرسل الآن الصورة التي تريد نشرها كقصة (ستُنشر لمدة 24 ساعة):")
		}

		w.WriteHeader(http.StatusOK)
		return
	}

	// 2. معالجة محادثة التحكم الخاصة بك
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
		} else if config.State == "waiting_first_name" {
			parts := strings.SplitN(strings.TrimSpace(msg.Text), " ", 2)
			firstName := parts[0]
			lastName := ""
			if len(parts) > 1 {
				lastName = parts[1]
			}
			if err := setBusinessAccountName(botToken, config.BusinessConnID, firstName, lastName); err != nil {
				sendSubMenu(botToken, chatID, "❌ فشل تعديل الاسم: "+err.Error())
			} else {
				config.State = ""
				saveConfig(botToken, chatID, config, msgID)
				sendMenu(botToken, chatID, "✅ تم تعديل الاسم بنجاح!")
			}
		} else if config.State == "waiting_bio" {
			if err := setBusinessAccountBio(botToken, config.BusinessConnID, msg.Text); err != nil {
				sendSubMenu(botToken, chatID, "❌ فشل تعديل النبذة: "+err.Error())
			} else {
				config.State = ""
				saveConfig(botToken, chatID, config, msgID)
				sendMenu(botToken, chatID, "✅ تم تعديل النبذة بنجاح!")
			}
		} else if config.State == "waiting_username" {
			username := strings.TrimPrefix(strings.TrimSpace(msg.Text), "@")
			if err := setBusinessAccountUsername(botToken, config.BusinessConnID, username); err != nil {
				sendSubMenu(botToken, chatID, "❌ فشل تعديل اليوزر: "+err.Error())
			} else {
				config.State = ""
				saveConfig(botToken, chatID, config, msgID)
				sendMenu(botToken, chatID, "✅ تم تعديل اسم المستخدم بنجاح!")
			}
		} else if config.State == "waiting_photo" {
			if len(msg.Photo) == 0 {
				sendSubMenu(botToken, chatID, "❌ أرسل صورة فعلية (لا يقبل ملفات أو نصوص).")
			} else {
				fileID := msg.Photo[len(msg.Photo)-1].FileID
				if err := setBusinessAccountProfilePhoto(botToken, config.BusinessConnID, fileID); err != nil {
					sendSubMenu(botToken, chatID, "❌ فشل تعديل الصورة: "+err.Error())
				} else {
					config.State = ""
					saveConfig(botToken, chatID, config, msgID)
					sendMenu(botToken, chatID, "✅ تم تعديل صورة الملف الشخصي بنجاح!")
				}
			}
		} else if config.State == "waiting_story" {
			if len(msg.Photo) == 0 {
				sendSubMenu(botToken, chatID, "❌ أرسل صورة فعلية لنشرها كقصة.")
			} else {
				fileID := msg.Photo[len(msg.Photo)-1].FileID
				if err := postBusinessStory(botToken, config.BusinessConnID, fileID); err != nil {
					sendSubMenu(botToken, chatID, "❌ فشل نشر القصة: "+err.Error())
				} else {
					config.State = ""
					saveConfig(botToken, chatID, config, msgID)
					sendMenu(botToken, chatID, "✅ تم نشر القصة بنجاح! ستبقى ظاهرة لمدة 24 ساعة.")
				}
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

		// تجهيز اسم العميل
		customerName := msg.From.FirstName
		if customerName == "" {
			customerName = "صديقي"
		}

		var replyText string
		if strings.TrimSpace(msg.Text) == "" {
			// رسالة غير نصية (صورة/ملصق/صوت...) - رد عام مختصر
			replyText = "شكراً لتواصلك يا " + customerName + " 🌸\nاستلمت رسالتك وسأرد عليك قريباً."
		} else if config.AutoReply == "" {
			replyText = "أهلاً بك يا " + customerName + " 🌸\nأنا غير متوفر الآن، اترك رسالتك وسأرد عليك قريباً."
		} else if strings.Contains(config.AutoReply, "{name}") || strings.Contains(config.AutoReply, "{الاسم}") {
			replyText = strings.ReplaceAll(config.AutoReply, "{name}", customerName)
			replyText = strings.ReplaceAll(replyText, "{الاسم}", customerName)
		} else {
			replyText = "أهلاً بك يا " + customerName + " 🌸\n" + config.AutoReply
		}

		sendBusinessReplyWithQuoteButton(botToken, customerChatID, replyText, msg.BusinessConnectionID)
		w.WriteHeader(http.StatusOK)
		return
	}

	// 4. رصد تفعيل/تعديل ربط حساب تجاري جديد بالبوت وإشعار المطوّر
	if update.BusinessConnection != nil {
		bc := update.BusinessConnection
		if bc.IsEnabled {
			notifyDeveloper(botToken, bc.User.ID, bc.User.FirstName, bc.User.LastName, bc.User.Username)

			// حفظ business_connection_id في إعدادات صاحب الحساب حتى نستخدمه
			// لاحقاً في تعديل الملف الشخصي ونشر القصص
			if bc.UserChatID != 0 {
				cfg, msgID := getConfig(botToken, bc.UserChatID)
				cfg.BusinessConnID = bc.ID
				saveConfig(botToken, bc.UserChatID, cfg, msgID)
			}
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func getAdminIDFromBusinessConn(token string, connID string) int64 {
	if connID == "" {
		return 0
	}
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getBusinessConnection?business_connection_id=%s", token, connID)
	resp, err := httpClient.Get(url)
	if err != nil {
		log.Println("خطأ getBusinessConnection:", err)
		return 0
	}
	defer resp.Body.Close()

	var res BusinessConnectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		log.Println("خطأ فك تشفير getBusinessConnection:", err)
		return 0
	}
	if res.Result.UserChatID != 0 {
		return res.Result.UserChatID
	}
	return res.Result.User.ID
}

func getConfig(token string, chatID int64) (BotConfig, int) {
	defaultCfg := BotConfig{
		IsStopped:      false,
		AutoReply:      "",
		Excluded:       []int64{},
		State:          "",
		BusinessConnID: "",
	}

	if chatID == 0 {
		return defaultCfg, 0
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/getChat?chat_id=%d", token, chatID)
	resp, err := httpClient.Get(url)
	if err != nil {
		log.Println("خطأ getChat:", err)
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

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		log.Println("خطأ فك تشفير getChat:", err)
		return defaultCfg, 0
	}

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
		if _, err := httpClient.Post(url, "application/json", bytes.NewBuffer(pBytes)); err != nil {
			log.Println("خطأ editMessageText:", err)
		}
	} else {
		url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
		payload := map[string]interface{}{
			"chat_id": chatID,
			"text":    cfgText,
		}
		pBytes, _ := json.Marshal(payload)
		resp, err := httpClient.Post(url, "application/json", bytes.NewBuffer(pBytes))
		if err != nil {
			log.Println("خطأ sendMessage (saveConfig):", err)
			return
		}
		defer resp.Body.Close()
		var res struct {
			Result struct {
				MessageID int `json:"message_id"`
			} `json:"result"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			log.Println("خطأ فك تشفير sendMessage:", err)
			return
		}
		if res.Result.MessageID != 0 {
			pinUrl := fmt.Sprintf("https://api.telegram.org/bot%s/pinChatMessage", token)
			pinPayload := map[string]interface{}{
				"chat_id":              chatID,
				"message_id":           res.Result.MessageID,
				"disable_notification": true,
			}
			pPinBytes, _ := json.Marshal(pinPayload)
			if _, err := httpClient.Post(pinUrl, "application/json", bytes.NewBuffer(pPinBytes)); err != nil {
				log.Println("خطأ pinChatMessage:", err)
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
			{{"text": "🧑 إدارة الملف الشخصي", "callback_data": "profile_menu"}},
			{{"text": "📖 نشر قصة", "callback_data": "post_story"}},
		},
	}

	payload := map[string]interface{}{
		"chat_id":      chatID,
		"text":         text,
		"parse_mode":   "Markdown",
		"reply_markup": keyboard,
	}
	b, _ := json.Marshal(payload)
	if _, err := httpClient.Post("https://api.telegram.org/bot"+token+"/sendMessage", "application/json", bytes.NewBuffer(b)); err != nil {
		log.Println("خطأ sendMenu:", err)
	}
}

func sendProfileMenu(token string, chatID int64, text string) {
	keyboard := map[string]interface{}{
		"inline_keyboard": [][]map[string]string{
			{{"text": "✏️ تعديل الاسم", "callback_data": "edit_first_name"}},
			{{"text": "📝 تعديل النبذة", "callback_data": "edit_bio"}},
			{{"text": "🖼️ تعديل الصورة", "callback_data": "edit_photo"}},
			{{"text": "🔗 تعديل اليوزر", "callback_data": "edit_username"}},
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
	if _, err := httpClient.Post("https://api.telegram.org/bot"+token+"/sendMessage", "application/json", bytes.NewBuffer(b)); err != nil {
		log.Println("خطأ sendProfileMenu:", err)
	}
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
	if _, err := httpClient.Post("https://api.telegram.org/bot"+token+"/sendMessage", "application/json", bytes.NewBuffer(b)); err != nil {
		log.Println("خطأ sendSubMenu:", err)
	}
}

func sendMessage(token string, chatID int64, text string) {
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}
	b, _ := json.Marshal(payload)
	if _, err := httpClient.Post("https://api.telegram.org/bot"+token+"/sendMessage", "application/json", bytes.NewBuffer(b)); err != nil {
		log.Println("خطأ sendMessage:", err)
	}
}

func sendBusinessReplyWithQuoteButton(token string, chatID int64, text, bizID string) {
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
	if _, err := httpClient.Post("https://api.telegram.org/bot"+token+"/sendMessage", "application/json", bytes.NewBuffer(b)); err != nil {
		log.Println("خطأ sendBusinessReplyWithQuoteButton:", err)
	}
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
	if _, err := httpClient.Post("https://api.telegram.org/bot"+token+"/editMessageReplyMarkup", "application/json", bytes.NewBuffer(b)); err != nil {
		log.Println("خطأ updateButtonQuote:", err)
	}
}

// إرسال إشعار للمطوّر عند تفعيل البوت على حساب تجاري جديد
// يقرأ ايدي المطور من متغير البيئة DEVELOPER_CHAT_ID في فيرسل
func notifyDeveloper(token string, userID int64, firstName, lastName, username string) {
	devChatID := os.Getenv("DEVELOPER_CHAT_ID")
	if devChatID == "" {
		log.Println("تحذير: DEVELOPER_CHAT_ID غير مضبوط، لن يتم إرسال إشعار التفعيل")
		return
	}
	devID, err := strconv.ParseInt(devChatID, 10, 64)
	if err != nil {
		log.Println("خطأ: DEVELOPER_CHAT_ID غير صالح:", err)
		return
	}

	fullName := firstName
	if lastName != "" {
		fullName += " " + lastName
	}
	if fullName == "" {
		fullName = "غير معروف"
	}

	usernameLine := "لا يوجد يوزر"
	if username != "" {
		usernameLine = "@" + username
	}

	text := fmt.Sprintf(
		"🔔 *تفعيل جديد للبوت*\n\n👤 الاسم: %s\n🆔 الايدي: `%d`\n🔗 اليوزر: %s",
		fullName, userID, usernameLine,
	)

	sendMessage(token, devID, text)
}

// --- دوال إدارة الملف الشخصي والقصص عبر Telegram Business API ---

// نتيجة عامة من تليجرام للتحقق من نجاح الطلب وقراءة رسالة الخطأ إن وُجدت
type apiResult struct {
	Ok          bool   `json:"ok"`
	Description string `json:"description"`
}

func callBusinessAPI(token, method string, payload map[string]interface{}) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/%s", token, method)
	b, _ := json.Marshal(payload)
	resp, err := httpClient.Post(url, "application/json", bytes.NewBuffer(b))
	if err != nil {
		log.Println("خطأ استدعاء", method, ":", err)
		return fmt.Errorf("تعذر الاتصال بتليجرام")
	}
	defer resp.Body.Close()

	var res apiResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		log.Println("خطأ فك تشفير رد", method, ":", err)
		return fmt.Errorf("رد غير متوقع من تليجرام")
	}
	if !res.Ok {
		log.Println("فشل", method, ":", res.Description)
		return fmt.Errorf(res.Description)
	}
	return nil
}

func setBusinessAccountName(token, businessConnID, firstName, lastName string) error {
	payload := map[string]interface{}{
		"business_connection_id": businessConnID,
		"first_name":             firstName,
	}
	if lastName != "" {
		payload["last_name"] = lastName
	}
	return callBusinessAPI(token, "setBusinessAccountName", payload)
}

func setBusinessAccountBio(token, businessConnID, bio string) error {
	payload := map[string]interface{}{
		"business_connection_id": businessConnID,
		"bio":                    bio,
	}
	return callBusinessAPI(token, "setBusinessAccountBio", payload)
}

func setBusinessAccountUsername(token, businessConnID, username string) error {
	payload := map[string]interface{}{
		"business_connection_id": businessConnID,
		"username":               username,
	}
	return callBusinessAPI(token, "setBusinessAccountUsername", payload)
}

func setBusinessAccountProfilePhoto(token, businessConnID, fileID string) error {
	payload := map[string]interface{}{
		"business_connection_id": businessConnID,
		"photo": map[string]interface{}{
			"type":  "static",
			"photo": fileID,
		},
	}
	return callBusinessAPI(token, "setBusinessAccountProfilePhoto", payload)
}

func postBusinessStory(token, businessConnID, fileID string) error {
	payload := map[string]interface{}{
		"business_connection_id": businessConnID,
		"content": map[string]interface{}{
			"type":  "photo",
			"photo": fileID,
		},
		// المدة المسموحة: 21600 (6س) / 43200 (12س) / 86400 (24س) / 172800 (48س)
		"active_period": 86400,
	}
	return callBusinessAPI(token, "postStory", payload)
}

func deleteMessage(token string, chatID int64, msgID int) {
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": msgID,
	}
	b, _ := json.Marshal(payload)
	if _, err := httpClient.Post("https://api.telegram.org/bot"+token+"/deleteMessage", "application/json", bytes.NewBuffer(b)); err != nil {
		log.Println("خطأ deleteMessage:", err)
	}
}

func answerCallback(token, callbackID string) {
	payload := map[string]string{"callback_query_id": callbackID}
	b, _ := json.Marshal(payload)
	if _, err := httpClient.Post("https://api.telegram.org/bot"+token+"/answerCallbackQuery", "application/json", bytes.NewBuffer(b)); err != nil {
		log.Println("خطأ answerCallback:", err)
	}
}
