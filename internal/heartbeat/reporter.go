package heartbeat

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"linkmaster-node/internal/config"

	"go.uber.org/zap"
)

type Reporter struct {
	cfg    *config.Config
	client *http.Client
	logger *zap.Logger
	stopCh chan struct{}
}

func NewReporter(cfg *config.Config) *Reporter {
	logger, _ := zap.NewProduction()
	return &Reporter{
		cfg: cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
		stopCh: make(chan struct{}),
	}
}

func (r *Reporter) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(r.cfg.Heartbeat.Interval) * time.Second)
	defer ticker.Stop()

	// 立即发送一次心跳
	r.sendHeartbeat()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.sendHeartbeat()
		}
	}
}

func (r *Reporter) Stop() {
	close(r.stopCh)
}

func (r *Reporter) sendHeartbeat() {
	// 新节点不发送IP，让后端服务器从请求中获取
	// 发送心跳（使用Form格式，兼容旧接口）
	url := fmt.Sprintf("%s/api/node/heartbeat", r.cfg.Backend.URL)
	req, err := http.NewRequest("POST", url, bytes.NewBufferString("type=pingServer"))
	if err != nil {
		r.logger.Error("创建心跳请求失败", zap.Error(err))
		return
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := r.client.Do(req)
	if err != nil {
		r.logger.Warn("发送心跳失败", zap.Error(err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		r.logger.Debug("心跳发送成功，后端将从请求中获取节点IP")
	} else {
		r.logger.Warn("心跳发送失败", zap.Int("status", resp.StatusCode))
	}
}

