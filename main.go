package handler

import (
	"encoding/json"
	"net/http"
)

// هيكل بيانات رسالة تلجرام البسيطة (Business Message)
type TelegramUpdate struct {
	UpdateID int `json:"update_id"`
	BusinessMessage struct {
		MessageID int `json:"message_id"`
		From struct {
			ID int64 `json:"id"`
			FirstName string `json:"first_name"`
		} `json:"from"`
		Text string `json:"text"`
		BusinessConnectionID string `json:"business_connection_id"`
	} `json:"business_message"`
}

// دالة الـ Handler التي تستقبل طلبات Vercel (Serverless Function)
func Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var update TelegramUpdate
	err := json.NewDecoder(r.Body).Decode(&update)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// تحقق من وجود رسالة أعمال واردة ورّد عليها
	if update.BusinessMessage.Text != "" {
		// هنا يمكنك كتابة منطق الرد التلقائي باستخدام Telegram Bot API
		// عن طريق إرسال طلب HTTP POST إلى:
		// https://api.telegram.org/bot<TOKEN>/sendMessage
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
