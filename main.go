package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var httpClient = &http.Client{Timeout: 8 * time.Second}
var mediaClient = &http.Client{Timeout: 30 * time.Second}

// هيكل لتخزين معلومات الوسائط المحذوفة وإعادة إرسالها
type CachedMessage struct {
	SenderName string
	SenderID   int64
	MediaType  string // text, photo, video, animation, sticker, voice, document
	FileID     string
	Text       string
}

var messageCache = sync.Map{}

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

var translations = map[string]map[string]string{
	"ar": {
		"main_menu_title":        "القائمة الرئيسية 🤖:",
		"welcome":                "أهلاً بك في لوحة تحكم البوت 🤖\nاختر من الأزرار أدناه للتحكم الكامل:",
		"stop_btn":                "🛑 إيقاف الرد",
		"start_btn":               "🟢 تشغيل الرد",
		"edit_text_btn":           "📝 تعديل نص الرد",
		"exclude_btn":             "👤 استثناء حساب",
		"list_excluded_btn":       "📋 عرض المستثنين",
		"clear_excluded_btn":      "🧹 مسح المستثنين",
		"profile_menu_btn":        "🧑 إدارة الملف الشخصي",
		"post_story_btn":          "📖 نشر قصة",
		"lang_ar_btn":             "🇮🇶 العربية",
		"lang_en_btn":             "🇺🇸 English",
		"back_btn":                "🔙 رجوع",
		"stopped_msg":             "🛑 تم إيقاف الرد التلقائي بنجاح.",
		"started_msg":             "🟢 تم تشغيل الرد التلقائي بنجاح.",
		"edit_text_prompt":        "📝 أرسل الآن نص الرد التلقائي الجديد:",
		"saved_text_msg":          "✅ تم حفظ نص الرد التلقائي الجديد بنجاح!",
		"exclude_prompt":          "👤 أرسل ايدي الحساب المراد استثناؤه الآن:",
		"invalid_id_msg":          "❌ أرقام فقط! أرسل الايدي بشكل صحيح.",
		"id_added_msg":            "✅ تم إضافة الايدي `%d` إلى قائمة الاستثناء.",
		"list_excluded_title":     "📋 **قائمة الحسابات المستثناة:**\n",
		"no_excluded":             "لا يوجد حسابات مستثناة حالياً.",
		"cleared_excluded_msg":    "🧹 تم مسح جميع الاستثناءات بنجاح.",
		"profile_menu_title":      "🧑 إدارة الملف الشخصي - اختر ما تريد تعديله:",
		"edit_first_name_btn":     "✏️ تعديل الاسم",
		"edit_bio_btn":            "📝 تعديل النبذة",
		"edit_photo_btn":          "🖼️ تعديل الصورة",
		"edit_username_btn":       "🔗 تعديل اليوزر",
		"no_business_connection":  "❌ لم يتم ربط حساب تجاري بعد بالبوت.",
		"first_name_prompt":       "✏️ أرسل الآن الاسم الأول الجديد (والاسم الأخير بعده بمسافة، اختياري):",
		"bio_prompt":              "📝 أرسل الآن النبذة الجديدة (حد أقصى 140 حرف):",
		"username_prompt":         "🔗 أرسل الآن اسم المستخدم الجديد (بدون @):",
		"photo_prompt":            "🖼️ أرسل الآن الصورة الجديدة لملفك الشخصي:",
		"name_updated":            "✅ تم تعديل الاسم بنجاح!",
		"bio_updated":             "✅ تم تعديل النبذة بنجاح!",
		"username_updated":        "✅ تم تعديل اسم المستخدم بنجاح!",
		"photo_updated":           "✅ تم تعديل صورة الملف الشخصي بنجاح!",
		"select_story_duration":   "⏱️ اختر مدة ظهور القصة المطلوبة:",
		"dur_6h":                  "6 ساعات",
		"dur_12h":                 "12 ساعة",
		"dur_24h":                 "24 ساعة",
		"dur_48h":                 "48 ساعة",
		"story_prompt":            "📖 أرسل الآن صورة أو فيديو (حد أقصى 60 ثانية) لنشره كقصة (ستبقى ظاهرة لمدة %s):",
		"story_updated":           "✅ تم نشر القصة بنجاح! ستبقى ظاهرة لمدة %s.",
		"your_id_msg":             "الايدي الخاص بك هو:\n`%d`",
		"fail_name":               "❌ فشل تعديل الاسم: %s",
		"fail_bio":                "❌ فشل تعديل النبذة: %s",
		"fail_username":           "❌ فشل تعديل اليوزر: %s",
		"fail_photo":              "❌ فشل تعديل الصورة: %s",
		"fail_story":              "❌ فشل نشر القصة: %s",
		"need_real_photo":         "❌ أرسل صورة فعلية (لا يقبل ملفات أو نصوص).",
		"need_real_media_story":   "❌ أرسل صورة أو فيديو فعلي لنشره كقصة.",
		"video_too_long_error":    "الفيديو أطول من 60 ثانية، وهذا الحد الأقصى المسموح لقصص تليجرام",
	},
	"en": {
		"main_menu_title":        "Main Menu 1🤖:",
		"welcome":                "Welcome to the bot control panel 🤖",
		"stop_btn":                "🛑 Stop Auto-Reply",
		"start_btn":               "🟢 Start Auto-Reply",
		"edit_text_btn":           "📝 Edit Reply Text",
		"exclude_btn":             "👤 Exclude Account",
		"list_excluded_btn":       "📋 View Excluded",
		"clear_excluded_btn":      "🧹 Clear Excluded",
		"profile_menu_btn":        "🧑 Manage Profile",
		"post_story_btn":          "📖 Post Story",
		"lang_ar_btn":             "🇮🇶 العربية",
		"lang_en_btn":             "🇺🇸 English",
		"back_btn":                "🔙 Back",
		"stopped_msg":             "🛑 Auto-reply has been stopped.",
		"started_msg":             "🟢 Auto-reply has been started.",
		"edit_text_prompt":        "📝 Send the new auto-reply text now:",
		"saved_text_msg":          "✅ New auto-reply text saved successfully!",
		"exclude_prompt":          "👤 Send the account ID to exclude now:",
		"invalid_id_msg":          "❌ Numbers only! Please send a valid ID.",
		"id_added_msg":            "✅ ID `%d` added to the exclusion list.",
		"list_excluded_title":     "📋 **Excluded Accounts:**\n",
		"no_excluded":             "No excluded accounts currently.",
		"cleared_excluded_msg":    "🧹 All exclusions cleared successfully.",
		"profile_menu_title":      "🧑 Manage Profile - choose what to edit:",
		"edit_first_name_btn":     "✏️ Edit Name",
		"edit_bio_btn":            "📝 Edit Bio",
		"edit_photo_btn":          "🖼️ Edit Photo",
		"edit_username_btn":       "🔗 Edit Username",
		"no_business_connection":  "❌ No business account connected to the bot yet.",
		"first_name_prompt":       "✏️ Send the new first name now:",
		"bio_prompt":              "📝 Send the new bio now:",
		"username_prompt":         "🔗 Send the new username now:",
		"photo_prompt":            "🖼️ Send the new profile photo now:",
		"name_updated":            "✅ Name updated successfully!",
		"bio_updated":             "✅ Bio updated successfully!",
		"username_updated":        "✅ Username updated successfully!",
		"photo_updated":           "✅ Profile photo updated successfully!",
		"select_story_duration":   "⏱️ Select story duration:",
		"dur_6h":                  "6 Hours",
		"dur_12h":                 "12 Hours",
		"dur_24h":                 "24 Hours",
		"dur_48h":                 "48 Hours",
		"story_prompt":            "📖 Send a photo or video now to post as a story:",
		"story_updated":           "✅ Story posted successfully!",
		"your_id_msg":             "Your ID is:\n`%d`",
		"fail_name":               "❌ Failed to update name: %s",
		"fail_bio":                "❌ Failed to update bio: %s",
		"fail_username":           "❌ Failed to update username: %s",
		"fail_photo":              "❌ Failed to update photo: %s",
		"fail_story":              "❌ Failed to post story: %s",
		"need_real_photo":         "❌ Please send an actual photo.",
		"need_real_media_story":   "❌ Please send an actual photo or video.",
		"video_too_long_error":    "The video is longer than 60 seconds.",
	},
}

func tr(lang, key string) string {
	if lang != "en" {
		lang = "ar"
	}
	if val, ok := translations[lang][key]; ok {
		return val
	}
	return key
}

func getDurationLabel(lang, period string) string {
	switch period {
	case "21600":
		return tr(lang, "dur_6h")
	case "43200":
		return tr(lang, "dur_12h")
	case "86400":
		return tr(lang, "dur_24h")
	case "172800":
		return tr(lang, "dur_48h")
	default:
		return tr(lang, "dur_24h")
	}
}

func translateText(text, targetLang string) (string, string, error) {
	if strings.TrimSpace(text) == "" {
		return "", "", nil
	}
	endpoint := fmt.Sprintf(
		"https://translate.googleapis.com/translate_a/single?client=gtx&sl=auto&tl=%s&dt=t&q=%s",
		targetLang, url.QueryEscape(text),
	)

	resp, err := httpClient.Get(endpoint)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var result []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}

	if len(result) == 0 {
		return "", "", fmt.Errorf("فشل الترجمة")
	}

	translatedText := ""
	if sentences, ok := result[0].([]interface{}); ok {
		for _, sentence := range sentences {
			if s, ok := sentence.([]interface{}); ok && len(s) > 0 {
				if tText, ok := s[0].(string); ok {
					translatedText += tText
				}
			}
		}
	}

	detectedLang := ""
	if len(result) > 2 {
		if lang, ok := result[2].(string); ok {
			detectedLang = lang
		}
	}

	return translatedText, detectedLang, nil
}

type BotConfig struct {
	IsStopped      bool    `json:"is_stopped"`
	AutoReply      string  `json:"auto_reply"`
	Excluded       []int64 `json:"excluded"`
	State          string  `json:"state"`
	BusinessConnID string  `json:"business_conn_id"`
	Lang           string  `json:"lang"`
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
		Text                 string      `json:"text"`
		Photo                []PhotoSize `json:"photo"`
		Video                *Video      `json:"video"`
		Animation            *Animation  `json:"animation"`
		Voice                *Voice      `json:"voice"`
		Sticker              *Sticker    `json:"sticker"`
		Document             *Document   `json:"document"`
		IsOutgoing           bool        `json:"is_outgoing"`
		BusinessConnectionID string      `json:"business_connection_id"`
	} `json:"business_message"`
	DeletedBusinessMessages *struct {
		BusinessConnectionID string `json:"business_connection_id"`
		Chat                 struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		MessageIDs []int `json:"message_ids"`
	} `json:"deleted_business_messages"`
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
}

type Video struct {
	FileID string `json:"file_id"`
}

type Animation struct {
	FileID string `json:"file_id"`
}

type Voice struct {
	FileID string `json:"file_id"`
}

type Sticker struct {
	FileID string `json:"file_id"`
}

type Document struct {
	FileID string `json:"file_id"`
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

	// 1. معالجة الأزرار
	if update.CallbackQuery != nil {
		cb := update.CallbackQuery
		answerCallback(botToken, cb.ID)
		deleteMessage(botToken, cb.Message.Chat.ID, cb.Message.MessageID)
		adminID := cb.From.ID
		config, msgID := getConfig(botToken, adminID)
		lang := config.Lang

		switch cb.Data {
		case "main_menu":
			config.State = ""
			saveConfig(botToken, adminID, config, msgID)
			sendMenu(botToken, adminID, lang, tr(lang, "main_menu_title"))
		case "stop":
			config.IsStopped = true
			config.State = ""
			saveConfig(botToken, adminID, config, msgID)
			sendMenu(botToken, adminID, lang, tr(lang, "stopped_msg"))
		case "start":
			config.IsStopped = false
			config.State = ""
			saveConfig(botToken, adminID, config, msgID)
			sendMenu(botToken, adminID, lang, tr(lang, "started_msg"))
		case "edit_text":
			config.State = "waiting_text"
			saveConfig(botToken, adminID, config, msgID)
			sendSubMenu(botToken, adminID, lang, tr(lang, "edit_text_prompt"))
		case "exclude":
			config.State = "waiting_id"
			saveConfig(botToken, adminID, config, msgID)
			sendSubMenu(botToken, adminID, lang, tr(lang, "exclude_prompt"))
		case "list_excluded":
			txt := tr(lang, "list_excluded_title")
			if len(config.Excluded) == 0 {
				txt += tr(lang, "no_excluded")
			} else {
				for _, id := range config.Excluded {
					txt += fmt.Sprintf("- `%d`\n", id)
				}
			}
			sendSubMenu(botToken, adminID, lang, txt)
		case "clear_excluded":
			config.Excluded = []int64{}
			saveConfig(botToken, adminID, config, msgID)
			sendMenu(botToken, adminID, lang, tr(lang, "cleared_excluded_msg"))
		case "profile_menu":
			config.State = ""
			saveConfig(botToken, adminID, config, msgID)
			sendProfileMenu(botToken, adminID, lang, tr(lang, "profile_menu_title"))
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	// 2. رسائل محادثة التحكم
	if update.Message != nil {
		msg := update.Message
		chatID := msg.Chat.ID
		config, msgID := getConfig(botToken, chatID)
		lang := config.Lang

		if msg.Text == "/start" {
			sendMenu(botToken, chatID, lang, tr(lang, "welcome"))
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

		// تحديد نوع الوسائط وحفظها في الكاش لإعادة إرسالها عند الحذف
		cached := CachedMessage{
			SenderName: msg.From.FirstName,
			SenderID:   msg.From.ID,
		}

		if msg.Text != "" {
			cached.MediaType = "text"
			cached.Text = msg.Text
		} else if len(msg.Photo) > 0 {
			cached.MediaType = "photo"
			cached.FileID = msg.Photo[len(msg.Photo)-1].FileID
		} else if msg.Video != nil {
			cached.MediaType = "video"
			cached.FileID = msg.Video.FileID
		} else if msg.Animation != nil {
			cached.MediaType = "animation"
			cached.FileID = msg.Animation.FileID
		} else if msg.Sticker != nil {
			cached.MediaType = "sticker"
			cached.FileID = msg.Sticker.FileID
		} else if msg.Voice != nil {
			cached.MediaType = "voice"
			cached.FileID = msg.Voice.FileID
		} else if msg.Document != nil {
			cached.MediaType = "document"
			cached.FileID = msg.Document.FileID
		}

		cacheKey := fmt.Sprintf("%d_%d", msg.Chat.ID, msg.MessageID)
		messageCache.Store(cacheKey, cached)

		config, _ := getConfig(botToken, adminID)
		senderID := msg.From.ID
		customerChatID := msg.Chat.ID
		customerName := msg.From.FirstName
		if customerName == "" {
			customerName = "صديقي"
		}

		for _, exID := range config.Excluded {
			if exID == senderID || exID == customerChatID {
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		if config.IsStopped {
			w.WriteHeader(http.StatusOK)
			return
		}

		replyText := "أهلاً بك يا " + customerName + " 🌸\n"
		if config.AutoReply != "" {
			replyText += config.AutoReply
		} else {
			replyText += "أنا غير متوفر الآن، سأرد عليك قريباً."
		}

		sendBusinessReply(botToken, customerChatID, replyText, msg.BusinessConnectionID)
		w.WriteHeader(http.StatusOK)
		return
	}

	// 4. معالجة الحذف وإعادة إرسال الوسائط أو النصوص المحذوفة فوراً
	if update.DeletedBusinessMessages != nil {
		del := update.DeletedBusinessMessages
		adminID := getAdminIDFromBusinessConn(botToken, del.BusinessConnectionID)
		if adminID != 0 {
			for _, id := range del.MessageIDs {
				cacheKey := fmt.Sprintf("%d_%d", del.Chat.ID, id)
				if val, ok := messageCache.Load(cacheKey); ok {
					item := val.(CachedMessage)

					headerText := fmt.Sprintf("🗑️ *تم حذف رسالة/وسائط من العميل:*\n👤 الاسم: %s (`%d`)", item.SenderName, item.SenderID)

					// إعادة إرسال الوسائط نفسها التي تم حذفها
					switch item.MediaType {
					case "text":
						sendMessage(botToken, adminID, headerText+"\n\n💬 النص المحذوف:\n"+item.Text)
					case "photo":
						sendMediaFile(botToken, adminID, "sendPhoto", "photo", item.FileID, headerText)
					case "video":
						sendMediaFile(botToken, adminID, "sendVideo", "video", item.FileID, headerText)
					case "animation":
						sendMediaFile(botToken, adminID, "sendAnimation", "animation", item.FileID, headerText)
					case "sticker":
						sendStickerFile(botToken, adminID, item.FileID, headerText)
					case "voice":
						sendMediaFile(botToken, adminID, "sendVoice", "voice", item.FileID, headerText)
					case "document":
						sendMediaFile(botToken, adminID, "sendDocument", "document", item.FileID, headerText)
					}
				}
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
	defaultCfg := BotConfig{IsStopped: false, AutoReply: "", Excluded: []int64{}, Lang: "ar"}
	if chatID == 0 {
		return defaultCfg, 0
	}
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getChat?chat_id=%d", token, chatID)
	resp, err := httpClient.Get(url)
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
		if json.Unmarshal([]byte(res.Result.PinnedMessage.Text), &cfg) == nil {
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
		payload := map[string]interface{}{"chat_id": chatID, "message_id": pinnedMsgID, "text": cfgText}
		pBytes, _ := json.Marshal(payload)
		httpClient.Post(url, "application/json", bytes.NewBuffer(pBytes))
	}
}

func sendMenu(token string, chatID int64, lang, text string) {
	keyboard := map[string]interface{}{
		"inline_keyboard": [][]map[string]string{
			{{"text": tr(lang, "stop_btn"), "callback_data": "stop"}, {"text": tr(lang, "start_btn"), "callback_data": "start"}},
			{{"text": tr(lang, "edit_text_btn"), "callback_data": "edit_text"}},
			{{"text": tr(lang, "exclude_btn"), "callback_data": "exclude"}, {"text": tr(lang, "list_excluded_btn"), "callback_data": "list_excluded"}},
			{{"text": tr(lang, "clear_excluded_btn"), "callback_data": "clear_excluded"}},
			{{"text": tr(lang, "profile_menu_btn"), "callback_data": "profile_menu"}},
		},
	}
	payload := map[string]interface{}{"chat_id": chatID, "text": text, "parse_mode": "Markdown", "reply_markup": keyboard}
	b, _ := json.Marshal(payload)
	httpClient.Post("https://api.telegram.org/bot"+token+"/sendMessage", "application/json", bytes.NewBuffer(b))
}

func sendStoryDurationMenu(token string, chatID int64, lang string) {}
func sendProfileMenu(token string, chatID int64, lang, text string)    {}
func sendSubMenu(token string, chatID int64, lang, text string) {
	keyboard := map[string]interface{}{"inline_keyboard": [][]map[string]string{{{"text": tr(lang, "back_btn"), "callback_data": "main_menu"}}}}
	payload := map[string]interface{}{"chat_id": chatID, "text": text, "parse_mode": "Markdown", "reply_markup": keyboard}
	b, _ := json.Marshal(payload)
	httpClient.Post("https://api.telegram.org/bot"+token+"/sendMessage", "application/json", bytes.NewBuffer(b))
}

func sendMessage(token string, chatID int64, text string) {
	payload := map[string]interface{}{"chat_id": chatID, "text": text, "parse_mode": "Markdown"}
	b, _ := json.Marshal(payload)
	httpClient.Post("https://api.telegram.org/bot"+token+"/sendMessage", "application/json", bytes.NewBuffer(b))
}

func sendMediaFile(token string, chatID int64, method, fieldName, fileID, caption string) {
	payload := map[string]interface{}{
		"chat_id": chatID,
		fieldName: fileID,
		"caption": caption,
		"parse_mode": "Markdown",
	}
	b, _ := json.Marshal(payload)
	httpClient.Post(fmt.Sprintf("https://api.telegram.org/bot%s/%s", token, method), "application/json", bytes.NewBuffer(b))
}

func sendStickerFile(token string, chatID int64, fileID, caption string) {
	sendMessage(token, chatID, caption)
	payload := map[string]interface{}{
		"chat_id": chatID,
		"sticker": fileID,
	}
	b, _ := json.Marshal(payload)
	httpClient.Post(fmt.Sprintf("https://api.telegram.org/bot%s/sendSticker", token), "application/json", bytes.NewBuffer(b))
}

func sendBusinessReply(token string, chatID int64, text, bizID string) {
	payload := map[string]interface{}{
		"chat_id":                chatID,
		"text":                   text,
		"business_connection_id": bizID,
	}
	b, _ := json.Marshal(payload)
	httpClient.Post("https://api.telegram.org/bot"+token+"/sendMessage", "application/json", bytes.NewBuffer(b))
}

func deleteMessage(token string, chatID int64, msgID int) {
	payload := map[string]interface{}{"chat_id": chatID, "message_id": msgID}
	b, _ := json.Marshal(payload)
	httpClient.Post("https://api.telegram.org/bot"+token+"/deleteMessage", "application/json", bytes.NewBuffer(b))
}

func answerCallback(token, callbackID string) {
	payload := map[string]string{"callback_query_id": callbackID}
	b, _ := json.Marshal(payload)
	httpClient.Post("https://api.telegram.org/bot"+token+"/answerCallbackQuery", "application/json", bytes.NewBuffer(b))
}
