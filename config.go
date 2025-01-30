package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

type Config struct {
	OneAPIType     string   `json:"oneapi_type"`
	ExcludeChannel []int    `json:"exclude_channel"`
	ExcludeModel   []string `json:"exclude_model"`
	Models         []string `json:"models"`
	ForceModels    bool     `json:"force_models"`
	TimePeriod     string   `json:"time_period"`
	MaxConcurrent  int      `json:"max_concurrent"`
	RPS            int      `json:"rps"`
	DbType         string   `json:"db_type"`
	DbDsn          string   `json:"db_dsn"`
	BaseURL        string   `json:"base_url"`
	SystemToken    string   `json:"system_token"`
	UptimeKuma     struct {
		Status     string            `json:"status"`
		ModelURL   map[string]string `json:"model_url"`
		ChannelURL map[string]string `json:"channel_url"`
	} `json:"uptime-kuma"`
	Notification struct {
		SMTP struct {
			Enabled  bool   `json:"enabled"`
			Host     string `json:"host"`
			Port     int    `json:"port"`
			Username string `json:"username"`
			Password string `json:"password"`
			From     string `json:"from"`
			To       string `json:"to"`
		} `json:"smtp"`
		Webhook struct {
			Enabled  bool   `json:"enabled"`
			Type     string `json:"type"`
			Telegram struct {
				ChatID string `json:"chat_id"`
				Retry  int    `json:"retry"`
			}
			Secret string `json:"secret"`
		} `json:"webhook"`
	} `json:"notification"`
}

func loadConfig() (*Config, error) {
	file, err := ioutil.ReadFile("config.json")
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	var config Config
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	if config.OneAPIType == "" {
		config.OneAPIType = "oneapi"
	}

	if config.DbType == "" {
		config.DbType = "mysql"
	}

	if config.BaseURL[len(config.BaseURL)-1] == '/' {
		config.BaseURL = config.BaseURL[:len(config.BaseURL)-1]
	}

	if config.UptimeKuma.Status == "" {
		config.UptimeKuma.Status = "disabled"
	}

	if config.MaxConcurrent == 0 {
		config.MaxConcurrent = 5
	}

	if config.RPS == 0 {
		config.RPS = 5
	}

	return &config, nil
}
