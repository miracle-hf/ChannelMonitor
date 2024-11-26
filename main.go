package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Channel struct {
	ID      int
	Type    int
	Name    string
	BaseURL string
	Key     string
	Status  int
}

var (
	db     *gorm.DB
	config *Config
)

func fetchChannels() ([]Channel, error) {
	query := "SELECT id, type, name, base_url, `key`, status FROM channels"
	rows, err := db.Raw(query).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []Channel
	for rows.Next() {
		var c Channel
		if err := rows.Scan(&c.ID, &c.Type, &c.Name, &c.BaseURL, &c.Key, &c.Status); err != nil {
			return nil, err
		}

		switch c.Type {
		case 40:
			c.BaseURL = "https://api.siliconflow.cn"
		case 999:
			c.BaseURL = "https://api.siliconflow.cn"
		case 1:
			if c.BaseURL == "" {
				c.BaseURL = "https://api.openai.com"
			}
		}
		// 检查是否在排除列表中
		if contains(config.ExcludeChannel, c.ID) {
			log.Printf("渠道 %s(ID:%d) 在排除列表中，跳过\n", c.Name, c.ID)
			continue
		}
		channels = append(channels, c)
	}
	log.Printf("获取到 %d 个渠道\n", len(channels))
	log.Printf("准备测试的渠道：%v\n", channels)
	return channels, nil
}

func contains(slice []int, item int) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

func containsString(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

func testModels(channel Channel) ([]string, error) {
	var availableModels []string
	modelList := []string{}
	if config.ForceModels {
		log.Println("强制使用自定义模型列表")
		modelList = config.Models
	} else {
		// 从/v1/models接口获取模型列表
		req, err := http.NewRequest("GET", channel.BaseURL+"/v1/models", nil)
		if err != nil {
			return nil, fmt.Errorf("创建请求失败：%v", err)
		}
		req.Header.Set("Authorization", "Bearer "+channel.Key)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Println("获取模型列表失败：", err, "尝试自定义模型列表")
			modelList = config.Models
		} else {
			defer resp.Body.Close()
			body, _ := ioutil.ReadAll(resp.Body)
			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("获取模型列表失败，状态码：%d，响应：%s", resp.StatusCode, string(body))
			}

			// 解析响应JSON
			var response struct {
				Data []struct {
					ID string `json:"id"`
				} `json:"data"`
			}

			if err := json.Unmarshal(body, &response); err != nil {
				return nil, fmt.Errorf("解析模型列表失败：%v", err)
			}
			// 提取模型ID列表
			for _, model := range response.Data {
				if containsString(config.ExcludeModel, model.ID) {
					log.Printf("模型 %s 在排除列表中，跳过\n", model.ID)
					continue
				}
				modelList = append(modelList, model.ID)
			}
		}
	}
	// 测试模型
	for _, model := range modelList {
		url := channel.BaseURL
		if !strings.Contains(channel.BaseURL, "/v1/chat/completions") {
			if !strings.HasSuffix(channel.BaseURL, "/chat") {
				if !strings.HasSuffix(channel.BaseURL, "/v1") {
					url += "/v1"
				}
				url += "/chat"
			}
			url += "/completions"
		}

		// 构造请求
		reqBody := map[string]interface{}{
			"model": model,
			"messages": []map[string]string{
				{"role": "user", "content": "Hello! Reply in short"},
			},
		}
		jsonData, _ := json.Marshal(reqBody)

		log.Printf("测试渠道 %s(ID:%d) 的模型 %s\n", channel.Name, channel.ID, model)

		req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))
		if err != nil {
			log.Println("创建请求失败：", err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+channel.Key)

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("\033[31m请求失败：%v\033[0m\n", err)
			continue
		}
		defer resp.Body.Close()

		body, _ := ioutil.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusOK {
			// 根据返回内容判断是否成功
			availableModels = append(availableModels, model)
			log.Printf("\033[32m渠道 %s(ID:%d) 的模型 %s 测试成功\033[0m\n", channel.Name, channel.ID, model)
		} else {
			log.Printf("\033[31m渠道 %s(ID:%d) 的模型 %s 测试失败，状态码：%d，响应：%s\033[0m\n", channel.Name, channel.ID, model, resp.StatusCode, string(body))
		}
	}
	return availableModels, nil
}

func updateModels(channelID int, models []string) error {
	// 开始事务
	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	// 更新channels表
	modelsStr := strings.Join(models, ",")
	query := "UPDATE channels SET models = ? WHERE id = ?"
	result := tx.Exec(query, modelsStr, channelID)
	if result.Error != nil {
		tx.Rollback()
		return result.Error
	}

	// 如果有名为refresh的渠道，删除
	query = "DELETE FROM channels WHERE name = 'refresh'"
	result = tx.Exec(query)
	if result.Error != nil {
		tx.Rollback()
		return result.Error
	}

	// 更新abilities表
	// 硬删除
	query = "DELETE FROM abilities WHERE channel_id = ? AND model NOT IN (?)"
	result = tx.Exec(query, channelID, models)
	if result.Error != nil {
		tx.Rollback()
		return result.Error
	}
	// 修改
	query = "UPDATE abilities SET enabled = 1 WHERE channel_id = ? AND model IN (?)"
	result = tx.Exec(query, channelID, models)
	if result.Error != nil {
		tx.Rollback()
		return result.Error
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}

	// 如果类型是onehub，post刷新
	if config.OneAPIType == "onehub" {
		// 添加渠道进行刷新
		url := config.BaseURL + "/api/channel"
		payload := map[string]interface{}{
			"name":          "refresh",
			"type":          1,
			"key":           "test",
			"base_url":      "",
			"other":         "",
			"proxy":         "",
			"test_model":    "",
			"model_mapping": "{}",
			"models":        "test",
			"groups":        []string{"default"},
			"plugin":        map[string]interface{}{},
			"tag":           "",
			"only_chat":     false,
			"pre_cost":      1,
			"is_edit":       false,
			"group":         "default",
		}
		payloadBytes, _ := json.Marshal(payload)

		req, err := http.NewRequest("POST", url, strings.NewReader(string(payloadBytes)))
		if err != nil {
			return fmt.Errorf("创建请求失败：%v", err)
		}
		req.Header.Set("Authorization", "Bearer "+config.SystemToken)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("刷新失败：%v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("刷新失败，状态码：%d", resp.StatusCode)
		}

		log.Println("刷新成功")

		// 获取刷新渠道
		url = config.BaseURL + "/api/channel/?name=refresh"
		req, err = http.NewRequest("GET", url, nil)
		if err != nil {
			return fmt.Errorf("创建请求失败：%v", err)
		}
		req.Header.Set("Authorization", "Bearer "+config.SystemToken)

		resp, err = client.Do(req)

		if err != nil {
			return fmt.Errorf("获取刷新渠道失败：%v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("获取刷新渠道失败，状态码：%d", resp.StatusCode)
		}

		body, _ := ioutil.ReadAll(resp.Body)
		var response struct {
			Data struct {
				Data []struct {
					ID int `json:"id"`
				} `json:"data"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &response); err != nil {
			return fmt.Errorf("解析刷新渠道失败：%v", err)
		}

		for _, data := range response.Data.Data {
			// 删除刷新渠道
			url = config.BaseURL + "/api/channel/" + fmt.Sprintf("%d", data.ID)
			req, err = http.NewRequest("DELETE", url, nil)
			if err != nil {
				return fmt.Errorf("创建请求失败：%v", err)
			}
			req.Header.Set("Authorization", "Bearer "+config.SystemToken)

			resp, err = client.Do(req)
			if err != nil {
				return fmt.Errorf("删除刷新渠道失败：%v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("删除刷新渠道失败，状态码：%d", resp.StatusCode)
			}
		}
	}
	return nil
}

func main() {
	var err error
	config, err = loadConfig()
	if err != nil {
		log.Fatal("加载配置失败：", err)
	}

	// 解析时间周期
	duration, err := time.ParseDuration(config.TimePeriod)
	if err != nil {
		log.Fatal("解析时间周期失败：", err)
	}

	db, err = NewDB(*config)

	if err != nil {
		log.Fatal("数据库连接失败：", err)
	}

	ticker := time.NewTicker(duration)
	defer ticker.Stop()

	for {
		log.Println("开始检测...")
		channels, err := fetchChannels()
		if err != nil {
			log.Printf("\033[31m获取渠道失败：%v\033[0m\n", err)
			continue
		}

		for _, channel := range channels {
			if channel.Name == "refresh" {
				continue
			}
			log.Printf("开始测试渠道 %s(ID:%d) 的模型\n", channel.Name, channel.ID)
			models, err := testModels(channel)
			if err != nil {
				log.Printf("\033[31m渠道 %s(ID:%d) 测试模型失败：%v\033[0m\n", channel.Name, channel.ID, err)
				continue
			}
			err = updateModels(channel.ID, models)
			if err != nil {
				log.Printf("\033[31m更新渠道 %s(ID:%d) 的模型失败：%v\033[0m\n", channel.Name, channel.ID, err)
			} else {
				log.Printf("渠道 %s(ID:%d) 可用模型：%v\n", channel.Name, channel.ID, models)
			}
		}

		// 等待下一个周期
		<-ticker.C
	}
}
