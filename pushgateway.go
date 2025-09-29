// pushgateway.go
package main

import (
	"log"
	"time"
	
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

// PushMetrics 推送指标到PushGateway
func pushMetricsToPushGateway(config PushGatewayConfig) error {
	if !config.Enabled {
		return nil
	}
	
	// 创建pusher
	pusher := push.New(config.URL, config.Job).
		Grouping("instance", config.Instance)
	
	// 收集所有已注册的指标
	pusher.Gatherer(prometheus.DefaultGatherer)
	
	// 推送指标
	if err := pusher.Push(); err != nil {
		return err
	}
	
	log.Printf("Successfully pushed metrics to PushGateway: %s", config.URL)
	return nil
}

// StartPushGatewayPusher 启动定期推送指标的goroutine
func startPushGatewayPusher(config PushGatewayConfig) {
	if !config.Enabled {
		log.Println("PushGateway is disabled")
		return
	}
	
	interval, err := time.ParseDuration(config.Interval)
	if err != nil {
		log.Printf("Failed to parse push interval: %v, using default 30s", err)
		interval = 30 * time.Second
	}
	
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := pushMetricsToPushGateway(config); err != nil {
				log.Printf("Failed to push metrics to PushGateway: %v", err)
			}
		}
	}()
	
	log.Printf("Started PushGateway pusher with interval: %v", interval)
}

// 在main.go中调用此函数的示例：
// 在main函数中添加：
// if config.PushGateway.Enabled {
//     startPushGatewayPusher(config.PushGateway)
// }