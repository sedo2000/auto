package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

var httpClient = &http.Client{Timeout: 8 * time.Second}
var mediaClient = &http.Client{Timeout: 30 * time.Second}

type CachedMessage struct {
	SenderName string
	SenderID   int64
	MediaType  string
	FileID     string
	Text       string
}

var messageCache = sync.Map{}

var translations = map[string]map[string]string{
	"ar": {
		"main_menu_title":      "القائمة الرئيسية 🤖:",
		"welcome":              "أهلاً بك في لوحة تحكم البوت 🤖\nاختر من الأزرار أدناه للتحكم الكامل:",
		"stop_btn":             "🛑 إيقاف الرد",
		"start_btn":            "🟢 تشغيل الرد",
		"edit_text_btn":        "📝 تعديل نص الرد",
		"exclude_btn":          "👤 استثناء حساب",
		"list_excluded_btn":    "📋 عرض المستثنين",
		"clear_excluded_btn":   "🧹 مسح المستثنين",
		"profile_menu_btn":     "🧑 إدارة الملف الشخصي",
		"post_story_btn":       "📖 نشر قصة",
		"lang_ar_btn":          "🇮🇶 العربية",
		"lang_en_btn":          "🇺🇸 English",
		"back_btn":             "🔙 رجوع",
		"stopped_msg":          "🛑 تم إيقاف الرد التلقائي بنجاح.",
		"started_msg":          "🟢 تم تشغيل الرد التلقائي بنجاح.",
		"edit_text_prompt":     "📝 أرسل الآن نص الرد التلقائي الجديد:",
		"saved_text_msg":       "✅ تم حفظ نص الرد التلقائي الجديد بنجاح!",
		"exclude_prompt":       "👤 أرسل ايدي الحساب المراد استثناؤه الآن:",
		"invalid_id_msg":       "❌ أرقام فقط! أرسل الايدي بشكل صحيح.",
		"id_added_msg":         "✅ تم إضافة الايدي `%d` إلى قائمة الاستثناء.",
		"list_excluded_title":  "📋 **قائمة الحسابات المستثناة:**\n",
		"no_excluded":          "لا يوجد حسابات مستثناة حالياً.",
		"cleared_excluded_msg": "🧹 تم مسح جميع الاستثناءات بنجاح.",
		"your_id_msg":          "الايدي الخاص بك هو:\n`%d`",
	},
	"en": {
		"main_menu_title":      "Main Menu 🤖:",
		"welcome":              "Welcome to the bot control panel 🤖",
		"stop_btn":             "🛑 Stop Auto-Reply",
		"start_btn":            "🟢 Start Auto-Reply",
		"edit_text_btn":        "📝 Edit Reply Text",
		"exclude_btn":          "👤 Exclude Account",
		"list_excluded_btn":    "📋 View Excluded",
		"clear_excluded_btn":   "🧹 Clear Excluded",
		"profile_menu_btn":     "🧑 Manage Profile",
		"post_story_btn":       "📖 Post Story",
		"lang_ar_btn":          "🇮🇶 العربية",
		"lang_en_btn":          "🇺🇸 English",
		"back_btn":             "🔙 Back",
		"stopped_msg":          "🛑 Auto-reply has been stopped.",
		"started_msg":          "🟢 Auto-reply has been started.",
		"edit_text_prompt":     "📝 Send the new auto-reply text now:",
		"saved_text_msg":       "✅ New auto-reply text saved successfully!",
		"exclude_prompt":       "👤 Send the account ID to exclude now:",
		"invalid_id_msg":       "❌ Numbers only! Please send a valid ID.",
		"id_added_msg":         "✅ ID `%d` added to the exclusion list.",
		"list_excluded_title":  "📋 **Excluded Accounts:**\n",
		"no_excluded":          "No excluded accounts currently.",
		"cleared_excluded_msg": "🧹 All exclusions cleared successfully.",
		"your_id_msg":          "Your ID is:\n`%d`",
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
		IsEnabled  bool  `json:"is_enabled"`
	} `json:"business_connection"`
}

type PhotoSize struct{ FileID string `json:"file_id"` }
type Video struct{ FileID string `json:"file_id"` }
type Animation struct{ FileID string `json:"file_id"` }
type Voice struct{ FileID string `json:"file_id"` }
type Sticker struct{ FileID string `json:"file_id"` }
type Document struct{ FileID string `json:"file_id"` }

type Message struct {
	MessageID int `json:"message_id"`
	Chat      struct{ ID int64 `json:"id"` } `json:"chat"`
	From      struct{ ID int64 `json:"id"` } `json:"from"`
	Text      string                      `json:"text"`
}

type CallbackQuery struct {
	ID      string  `json:"id"`
	Message Message `json:"message"`
	Data    string  `json:"data"`
	From    struct{ ID int64 `json:"id"` } `json:"from"`
}

type BusinessConnectionResponse struct {
	Ok     bool `json:"ok"`
	Result struct {
		User       struct{ ID int64 `json:"id"` } `json:"user"`
		UserChatID int64                          `json:"user_chat_id"`
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

	if update.CallbackQuery != nil {
		cb := update.CallbackQuery
		answerCallback(botToken, cb.ID)
		deleteMessage(botToken, cb.Message.Chat.ID, cb.Message.MessageID)
		adminID := cb.From.ID
		config, _ := getConfig(botToken, adminID)
		lang := config.Lang

		switch cb.Data {
		case "main_menu":
			config.State = ""
			saveConfig(botToken, adminID, config)
			sendMenu(botToken, adminID, lang, tr(lang, "main_menu_title"))
		case "stop":
			config.IsStopped = true
			config.State = ""
			saveConfig(botToken, adminID, config)
			sendMenu(botToken, adminID, lang, tr(lang, "stopped_msg"))
		case "start":
			config.IsStopped = false
			config.State = ""
			saveConfig(botToken, adminID, config)
			sendMenu(botToken, adminID, lang, tr(lang, "started_msg"))
		}
		w.WriteHeader(http.StatusOK)
		return
	}

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

	if update.DeletedBusinessMessages != nil {
		del := update.DeletedBusinessMessages
		adminID := getAdminIDFromBusinessConn(botToken, del.BusinessConnectionID)
		if adminID != 0 {
			for _, id := range del.MessageIDs {
				cacheKey := fmt.Sprintf("%d_%d", del.Chat.ID, id)
				if val, ok := messageCache.Load(cacheKey); ok {
					item := val.(CachedMessage)
					headerText := fmt.Sprintf("🗑️ *تم حذف رسالة/وسائط من العميل:*\n👤 الاسم: %s (`%d`)", item.SenderName, item.SenderID)

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

func saveConfig(token string, chatID int64, cfg BotConfig) {
	if chatID == 0 {
		return
	}
	b, _ := json.Marshal(cfg)
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	payload := map[string]interface{}{"chat_id": chatID, "text": string(b)}
	pBytes, _ := json.Marshal(payload)
	httpClient.Post(url, "application/json", bytes.NewBuffer(pBytes))
}

func sendMenu(token string, chatID int64, lang, text string) {
	keyboard := map[string]interface{}{
		"inline_keyboard": [][]map[string]string{
			{{"text": tr(lang, "stop_btn"), "callback_data": "stop"}, {"text": tr(lang, "start_btn"), "callback_data": "start"}},
		},
	}
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
		"chat_id":    chatID,
		fieldName:    fileID,
		"caption":    caption,
		"parse_mode": "Markdown",
	}
	b, _ := json.Marshal(payload)
	httpClient.Post(fmt.Sprintf("https://api.telegram.org/bot%s/%s", token, method), "application/json", bytes.NewBuffer(b))
}

func sendStickerFile(token string, chatID int64, fileID, caption string) {
	sendMessage(token, chatID, caption)
	payload := map[string]interface{}{"chat_id": chatID, "sticker": fileID}
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

func main() {
	http.HandleFunc("/", Handler)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	http.ListenAndServe(":"+port, nil)
}
