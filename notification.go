package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
    "time"
)

type ChannelChange struct {
	ChannelID     int      `json:"channel_id"`
	ChannelName   string   `json:"channel_name"`
	OldModels     []string `json:"old_models"`
	NewModels     []string `json:"new_models"`
	AddedModels   []string `json:"added_models"`
	RemovedModels []string `json:"removed_models"`
}

func sendNotification(change ChannelChange) error {
	if config.Notification.SMTP.Enabled {
		if err := sendEmailNotification(change); err != nil {
			return fmt.Errorf("发送邮件通知失败: %v", err)
		}
	}

	if config.Notification.Webhook.Enabled {
		if err := sendWebhookNotification(change); err != nil {
			return fmt.Errorf("发送Webhook通知失败: %v", err)
		}
	}

	return nil
}

func sendEmailNotification(change ChannelChange) error {
	smtpConfig := config.Notification.SMTP
	auth := smtp.PlainAuth("", smtpConfig.Username, smtpConfig.Password, smtpConfig.Host)

	subject := "渠道模型变更通知"
	body := fmt.Sprintf(`
渠道ID: %d
渠道名称: %s
新增模型: %v
移除模型: %v
最新可用模型: %v
`, change.ChannelID, change.ChannelName, change.AddedModels, change.RemovedModels, change.NewModels)

	msg := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"\r\n"+
		"%s\r\n", smtpConfig.From, smtpConfig.To, subject, body)

	addr := fmt.Sprintf("%s:%d", smtpConfig.Host, smtpConfig.Port)
	return smtp.SendMail(addr, auth, smtpConfig.From, []string{smtpConfig.To}, []byte(msg))
}

func sendWebhookNotification(change ChannelChange) error {
	msg := fmt.Sprintf(`
渠道ID: %d
渠道名称: %s
新增模型: %v
移除模型: %v
最新可用模型: %v
`, change.ChannelID, change.ChannelName, change.AddedModels, change.RemovedModels, change.NewModels)

	if config.Notification.Webhook.Type == "telegram" {
		if err := sendTelegramNotification(msg); err != nil {
			return fmt.Errorf("发送Telegram通知失败: %v", err)
		}
	}

	return nil
}

func sendTelegramNotification(msg string) error {
	telegramConfig := config.Notification.Webhook.Telegram
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", config.Notification.Webhook.Secret)

	data := map[string]string{
		"chat_id": telegramConfig.ChatID,
		"text":    msg,
	}

	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("序列化数据失败: %v", err)
	}
    // 重试逻辑
    var lastErr error
    for attempt := 1; attempt <= telegramConfig.Retry; attempt++ {
        resp, err := http.Post(url, "application/json", bytes.NewReader(body))
        if err != nil {
            lastErr = fmt.Errorf("发送请求失败 (尝试 %d/%d): %v", 
                attempt, telegramConfig.Retry, err)
            if attempt < telegramConfig.Retry {
                time.Sleep(time.Second * 2) // 重试前等待2秒
                continue
            }
            return lastErr
        }
        defer resp.Body.Close()

        if resp.StatusCode == http.StatusOK {
            return nil
        }
        
        lastErr = fmt.Errorf("请求失败 (尝试 %d/%d): %v", 
            attempt, telegramConfig.Retry, resp.Status)
        if attempt < telegramConfig.Retry {
            time.Sleep(time.Second * 2)
            continue
        }
    }

    return lastErr
}

func compareModels(old, new []string) ([]string, []string) {
	oldMap := make(map[string]bool)
	for _, model := range old {
		oldMap[model] = true
	}

	newMap := make(map[string]bool)
	for _, model := range new {
		newMap[model] = true
	}

	var added, removed []string

	// 查找新增的模型
	for model := range newMap {
		if !oldMap[model] {
			added = append(added, model)
		}
	}

	// 查找移除的模型
	for model := range oldMap {
		if !newMap[model] {
			removed = append(removed, model)
		}
	}

	return added, removed
}
