package main

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/time/rate"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"
)

// Prometheus metrics
var (
	// 渠道相关指标
	channelStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "channel_status",
			Help: "Current status of channels (1 = active, 0 = inactive)",
		},
		[]string{"channel_id", "channel_name", "channel_type"},
	)

	channelTestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "channel_test_total",
			Help: "Total number of channel tests",
		},
		[]string{"channel_id", "channel_name", "status"},
	)

	// 模型相关指标
	modelTestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "model_test_total",
			Help: "Total number of model tests",
		},
		[]string{"channel_id", "channel_name", "model", "status"},
	)

	modelAvailability = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "model_availability",
			Help: "Model availability (1 = available, 0 = unavailable)",
		},
		[]string{"channel_id", "channel_name", "model"},
	)

	modelResponseTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "model_response_time_seconds",
			Help:    "Response time for model tests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"channel_id", "channel_name", "model"},
	)

	// 系统相关指标
	testCycleTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "test_cycle_total",
			Help: "Total number of test cycles completed",
		},
	)

	testCycleDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "test_cycle_duration_seconds",
			Help:    "Duration of test cycles in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	activeChannelsGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_channels_total",
			Help: "Total number of active channels",
		},
	)

	availableModelsGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "available_models_total",
			Help: "Total number of available models per channel",
		},
		[]string{"channel_id", "channel_name"},
	)

	// 数据库操作指标
	dbOperationTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "database_operation_total",
			Help: "Total number of database operations",
		},
		[]string{"operation", "status"},
	)

	dbOperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "database_operation_duration_seconds",
			Help:    "Duration of database operations in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)

	// 通知相关指标
	notificationTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notification_total",
			Help: "Total number of notifications sent",
		},
		[]string{"type", "status"},
	)

	// UptimeKuma推送指标
	uptimeKumaPushTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "uptimekuma_push_total",
			Help: "Total number of UptimeKuma pushes",
		},
		[]string{"type", "status"},
	)
)

func init() {
	// 注册所有指标
	prometheus.MustRegister(
		channelStatus,
		channelTestTotal,
		modelTestTotal,
		modelAvailability,
		modelResponseTime,
		testCycleTotal,
		testCycleDuration,
		activeChannelsGauge,
		availableModelsGauge,
		dbOperationTotal,
		dbOperationDuration,
		notificationTotal,
		uptimeKumaPushTotal,
	)
}

type Channel struct {
	ID           int
	Type         int
	Name         string
	BaseURL      string
	Key          string
	Status       int
	ModelMapping map[string]string
}

var (
	db     *gorm.DB
	config *Config
)

func fetchChannels() ([]Channel, error) {
	startTime := time.Now()
	defer func() {
		dbOperationDuration.WithLabelValues("fetch_channels").Observe(time.Since(startTime).Seconds())
	}()

	query := "SELECT id, type, name, base_url, `key`, status, model_mapping FROM channels"
	rows, err := db.Raw(query).Rows()
	if err != nil {
		dbOperationTotal.WithLabelValues("fetch_channels", "error").Inc()
		return nil, err
	}
	defer rows.Close()
	dbOperationTotal.WithLabelValues("fetch_channels", "success").Inc()

	var channels []Channel
	for rows.Next() {
		var c Channel
		var modelMapping string
		if err := rows.Scan(&c.ID, &c.Type, &c.Name, &c.BaseURL, &c.Key, &c.Status, &modelMapping); err != nil {
			return nil, err
		}
		c.ModelMapping = make(map[string]string)
		if modelMapping != "" {
			if err := json.Unmarshal([]byte(modelMapping), &c.ModelMapping); err != nil {
				return nil, err
			}
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
		
		// 更新渠道状态指标
		channelStatus.WithLabelValues(
			fmt.Sprintf("%d", c.ID),
			c.Name,
			fmt.Sprintf("%d", c.Type),
		).Set(float64(c.Status))
		
		channels = append(channels, c)
	}
	
	// 更新活跃渠道数量
	activeChannelsGauge.Set(float64(len(channels)))
	
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

func testModels(channel Channel, wg *sync.WaitGroup, mu *sync.Mutex) {
	defer wg.Done()

	var availableModels []string
	modelList := []string{}
	
	// 记录渠道测试
	channelTestTotal.WithLabelValues(
		fmt.Sprintf("%d", channel.ID),
		channel.Name,
		"started",
	).Inc()
	
	if config.ForceModels {
		log.Println("强制使用自定义模型列表")
		modelList = config.Models
	} else {
		if config.ForceInsideModels {
			log.Println("强制使用内置模型列表")
			// 从数据库获取模型列表
			var models string
			startTime := time.Now()
			if err := db.Raw("SELECT models FROM channels WHERE id = ?", channel.ID).Scan(&models).Error; err != nil {
				dbOperationTotal.WithLabelValues("get_models", "error").Inc()
				log.Printf("获取渠道 %s(ID:%d) 的模型列表失败：%v\n", channel.Name, channel.ID, err)
				return
			}
			dbOperationDuration.WithLabelValues("get_models").Observe(time.Since(startTime).Seconds())
			dbOperationTotal.WithLabelValues("get_models", "success").Inc()
			modelList = strings.Split(models, ",")
		} else {
			// 从/v1/models接口获取模型列表
			req, err := http.NewRequest("GET", channel.BaseURL+"/v1/models", nil)
			if err != nil {
				log.Printf("创建请求失败：%v\n", err)
				return
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
					log.Printf("获取模型列表失败，状态码：%d，响应：%s\n", resp.StatusCode, string(body))
					return
				}

				// 解析响应JSON
				var response struct {
					Data []struct {
						ID string `json:"id"`
					} `json:"data"`
				}

				if err := json.Unmarshal(body, &response); err != nil {
					log.Printf("解析模型列表失败：%v\n", err)
					return
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
	}
	// 测试模型并发处理
	modelWg := sync.WaitGroup{}
	modelMu := sync.Mutex{}
	sem := make(chan struct{}, config.MaxConcurrent)

	limiter := rate.NewLimiter(rate.Limit(config.RPS), config.RPS)

	for _, model := range modelList {
		modelWg.Add(1)
		sem <- struct{}{}

		go func(model string) {
			defer modelWg.Done()
			defer func() { <-sem }()
			// 限流
			limiter.Wait(context.Background())

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
					{"role": "user", "content": "Hi"},
				},
				"max_tokens": 1,
			}
			jsonData, _ := json.Marshal(reqBody)

			log.Printf("测试渠道 %s(ID:%d) 的模型 %s\n", channel.Name, channel.ID, model)

			req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))
			if err != nil {
				log.Printf("创建请求失败：%v\n", err)
				modelTestTotal.WithLabelValues(
					fmt.Sprintf("%d", channel.ID),
					channel.Name,
					model,
					"error",
				).Inc()
				modelAvailability.WithLabelValues(
					fmt.Sprintf("%d", channel.ID),
					channel.Name,
					model,
				).Set(0)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+channel.Key)

			// 记录响应时间
			startTime := time.Now()
			client := &http.Client{Timeout: time.Duration(config.Timeout) * time.Second}
			resp, err := client.Do(req)
			responseTime := time.Since(startTime).Seconds()
			
			if err != nil {
				log.Printf("\033[31m请求失败：%v\033[0m\n", err)
				modelTestTotal.WithLabelValues(
					fmt.Sprintf("%d", channel.ID),
					channel.Name,
					model,
					"error",
				).Inc()
				modelAvailability.WithLabelValues(
					fmt.Sprintf("%d", channel.ID),
					channel.Name,
					model,
				).Set(0)
				return
			}
			defer resp.Body.Close()

			body, _ := ioutil.ReadAll(resp.Body)
			if resp.StatusCode == http.StatusOK {
				// 根据返回内容判断是否成功
				modelMu.Lock()
				availableModels = append(availableModels, model)
				modelMu.Unlock()
				
				// 更新指标
				modelTestTotal.WithLabelValues(
					fmt.Sprintf("%d", channel.ID),
					channel.Name,
					model,
					"success",
				).Inc()
				modelAvailability.WithLabelValues(
					fmt.Sprintf("%d", channel.ID),
					channel.Name,
					model,
				).Set(1)
				modelResponseTime.WithLabelValues(
					fmt.Sprintf("%d", channel.ID),
					channel.Name,
					model,
				).Observe(responseTime)
				
				log.Printf("\033[32m渠道 %s(ID:%d) 的模型 %s 测试成功\033[0m\n", channel.Name, channel.ID, model)
				// 推送UptimeKuma
				if err := pushModelUptime(model); err != nil {
					log.Printf("\033[31m推送UptimeKuma失败：%v\033[0m\n", err)
					uptimeKumaPushTotal.WithLabelValues("model", "error").Inc()
				} else {
					uptimeKumaPushTotal.WithLabelValues("model", "success").Inc()
				}
				if err := pushChannelUptime(channel.ID); err != nil {
					log.Printf("\033[31m推送UptimeKuma失败：%v\033[0m\n", err)
					uptimeKumaPushTotal.WithLabelValues("channel", "error").Inc()
				} else {
					uptimeKumaPushTotal.WithLabelValues("channel", "success").Inc()
				}
			} else {
				log.Printf("\033[31m渠道 %s(ID:%d) 的模型 %s 测试失败，状态码：%d，响应：%s\033[0m\n", channel.Name, channel.ID, model, resp.StatusCode, string(body))
				modelTestTotal.WithLabelValues(
					fmt.Sprintf("%d", channel.ID),
					channel.Name,
					model,
					"failed",
				).Inc()
				modelAvailability.WithLabelValues(
					fmt.Sprintf("%d", channel.ID),
					channel.Name,
					model,
				).Set(0)
			}
		}(model)
	}
	modelWg.Wait()

	// 更新可用模型数量指标
	availableModelsGauge.WithLabelValues(
		fmt.Sprintf("%d", channel.ID),
		channel.Name,
	).Set(float64(len(availableModels)))
	
	// 记录渠道测试完成
	if len(availableModels) > 0 {
		channelTestTotal.WithLabelValues(
			fmt.Sprintf("%d", channel.ID),
			channel.Name,
			"success",
		).Inc()
	} else {
		channelTestTotal.WithLabelValues(
			fmt.Sprintf("%d", channel.ID),
			channel.Name,
			"failed",
		).Inc()
	}

	// 更新模型
	if config.DoNotModifyDb {
		log.Println("跳过数据库更新")
		return
	}
	mu.Lock()
	err := updateModels(channel.ID, availableModels, channel.ModelMapping)
	mu.Unlock()
	if err != nil {
		log.Printf("\033[31m更新渠道 %s(ID:%d) 的模型失败：%v\033[0m\n", channel.Name, channel.ID, err)
		dbOperationTotal.WithLabelValues("update_models", "error").Inc()
	} else {
		log.Printf("渠道 %s(ID:%d) 可用模型：%v\n", channel.Name, channel.ID, availableModels)
		dbOperationTotal.WithLabelValues("update_models", "success").Inc()
	}
}

func updateModels(channelID int, models []string, modelMapping map[string]string) error {
	startTime := time.Now()
	defer func() {
		dbOperationDuration.WithLabelValues("update_models").Observe(time.Since(startTime).Seconds())
	}()

	// 获取旧的模型列表
	var oldModels string
	if err := db.Raw("SELECT models FROM channels WHERE id = ?", channelID).Scan(&oldModels).Error; err != nil {
		return err
	}
	oldModelsList := strings.Split(oldModels, ",")

	// 如果不是onehub，直接更新数据库
	if config.OneAPIType != "onehub" {
		// 开始事务
		tx := db.Begin()
		if tx.Error != nil {
			return tx.Error
		}

		// 处理模型映射，用modelMapping反向替换models中的模型
		invertedMapping := make(map[string]string)
		for k, v := range modelMapping {
			invertedMapping[v] = k
		}
		for i, model := range models {
			if v, ok := invertedMapping[model]; ok {
				models[i] = v
			}
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
	} else {
		// 如果是onehub，使用PUT更新
		// 先获取渠道详情
		url := config.BaseURL + "/api/channel/" + fmt.Sprintf("%d", channelID)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return fmt.Errorf("创建请求失败：%v", err)
		}
		req.Header.Set("Authorization", "Bearer "+config.SystemToken)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("获取渠道详情失败：%v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("获取渠道详情失败，状态码：%d", resp.StatusCode)
		}

		body, _ := ioutil.ReadAll(resp.Body)
		var response struct {
			Data struct {
				ID                 int                    `json:"id"`
				Type               int                    `json:"type"`
				Key                string                 `json:"key"`
				Status             int                    `json:"status"`
				Name               string                 `json:"name"`
				Weight             int                    `json:"weight"`
				CreatedTime        int                    `json:"created_time"`
				TestTime           int                    `json:"test_time"`
				ResponseTime       int                    `json:"response_time"`
				BaseURL            string                 `json:"base_url"`
				Other              string                 `json:"other"`
				Balance            int                    `json:"balance"`
				BalanceUpdatedTime int                    `json:"balance_updated_time"`
				Models             string                 `json:"models"`
				Group              string                 `json:"group"`
				Tag                string                 `json:"tag"`
				UsedQuota          int                    `json:"used_quota"`
				ModelMapping       string                 `json:"model_mapping"`
				ModelHeaders       string                 `json:"model_headers"`
				Priority           int                    `json:"priority"`
				Proxy              string                 `json:"proxy"`
				TestModel          string                 `json:"test_model"`
				OnlyChat           bool                   `json:"only_chat"`
				PreCost            int                    `json:"pre_cost"`
				Plugin             map[string]interface{} `json:"plugin"`
			} `json:"data"`
			Message string `json:"message"`
			Success bool   `json:"success"`
		}

		if err := json.Unmarshal(body, &response); err != nil {
			return fmt.Errorf("解析渠道详情失败：%v", err)
		}

		// 更新模型
		response.Data.Models = strings.Join(models, ",")

		// 更新渠道
		url = config.BaseURL + "/api/channel/"
		payloadBytes, _ := json.Marshal(response.Data)
		req, err = http.NewRequest("PUT", url, strings.NewReader(string(payloadBytes)))
		if err != nil {
			return fmt.Errorf("创建请求失败：%v", err)
		}
		req.Header.Set("Authorization", "Bearer "+config.SystemToken)
		req.Header.Set("Content-Type", "application/json")

		resp, err = client.Do(req)
		if err != nil {
			return fmt.Errorf("更新渠道失败：%v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("更新渠道失败，状态码：%d", resp.StatusCode)
		}
		log.Println("更新成功")
	}

	// 对比模型变化并发送通知
	added, removed := compareModels(oldModelsList, models)
	if len(added) > 0 || len(removed) > 0 {
		var channelName string
		if err := db.Raw("SELECT name FROM channels WHERE id = ?", channelID).Scan(&channelName).Error; err != nil {
			log.Printf("获取渠道名称失败: %v", err)
		}

		change := ChannelChange{
			ChannelID:     channelID,
			ChannelName:   channelName,
			OldModels:     oldModelsList,
			NewModels:     models,
			AddedModels:   added,
			RemovedModels: removed,
		}

		if err := sendNotification(change); err != nil {
			log.Printf("发送通知失败: %v", err)
			notificationTotal.WithLabelValues("model_change", "error").Inc()
		} else {
			notificationTotal.WithLabelValues("model_change", "success").Inc()
		}
	}
	return nil
}

func pushModelUptime(modelName string) error {
	if config.UptimeKuma.Status != "enabled" {
		return nil
	}

	if config.UptimeKuma.ModelURL == nil {
		return nil
	}

	pushURL, ok := config.UptimeKuma.ModelURL[modelName]
	if !ok {
		return fmt.Errorf("找不到模型 %s 的推送地址", modelName)
	}

	req, err := http.NewRequest("GET", pushURL, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败：%v", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("推送失败：%v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("推送失败，状态码：%d", resp.StatusCode)
	}

	return nil
}

func pushChannelUptime(channelID int) error {
	if config.UptimeKuma.Status != "enabled" {
		return nil
	}

	if config.UptimeKuma.ChannelURL == nil {
		return nil
	}

	pushURL, ok := config.UptimeKuma.ChannelURL[fmt.Sprintf("%d", channelID)]
	if !ok {
		return fmt.Errorf("找不到渠道 %d 的推送地址", channelID)
	}

	req, err := http.NewRequest("GET", pushURL, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败：%v", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("推送失败：%v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("推送失败，状态码：%d", resp.StatusCode)
	}

	return nil
}

// 启动Metrics服务器
func startMetricsServer() {
	metricsPort := ":2112" // 默认端口
	if config.MetricsPort != "" {
		metricsPort = config.MetricsPort
	}
	
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	log.Printf("Starting metrics server on %s", metricsPort)
	if err := http.ListenAndServe(metricsPort, mux); err != nil {
		log.Printf("Failed to start metrics server: %v", err)
	}
}

func main() {
	var err error
	config, err = loadConfig()
	if err != nil {
		log.Fatal("加载配置失败：", err)
	}

	// 启动Metrics服务器
	go startMetricsServer()

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
		cycleStart := time.Now()
		log.Println("开始检测...")
		
		channels, err := fetchChannels()
		if err != nil {
			log.Printf("\033[31m获取渠道失败：%v\033[0m\n", err)
			continue
		}

		var wg sync.WaitGroup
		var mu sync.Mutex
		for _, channel := range channels {
			if channel.Name == "refresh" {
				continue
			}
			wg.Add(1)
			go testModels(channel, &wg, &mu)
		}
		wg.Wait()

		// 记录测试周期指标
		testCycleDuration.Observe(time.Since(cycleStart).Seconds())
		testCycleTotal.Inc()
		
		log.Printf("测试周期完成，耗时：%v\n", time.Since(cycleStart))

		// 等待下一个周期
		<-ticker.C
	}
}