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
	DbType         string   `json:"db_type"`
	DbDsn          string   `json:"db_dsn"`
	BaseURL        string   `json:"base_url"`
	SystemToken    string   `json:"system_token"`
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

	return &config, nil
}
