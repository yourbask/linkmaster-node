package heartbeat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"linkmaster-node/internal/config"

	"go.uber.org/zap"
)

// 节点信息存储（通过心跳更新）
var nodeInfo struct {
	sync.RWMutex
	nodeID uint
	nodeIP string
}

// GetNodeID 获取节点ID
func GetNodeID() uint {
	nodeInfo.RLock()
	defer nodeInfo.RUnlock()
	return nodeInfo.nodeID
}

// GetNodeIP 获取节点IP
func GetNodeIP() string {
	nodeInfo.RLock()
	defer nodeInfo.RUnlock()
	return nodeInfo.nodeIP
}

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
		// 尝试解析响应，获取 node_id 和 node_ip
		body, err := io.ReadAll(resp.Body)
		if err == nil && len(body) > 0 {
			// 尝试解析 JSON 响应
			var result struct {
				Status string `json:"status"`
				NodeID uint   `json:"node_id"`
				NodeIP string `json:"node_ip"`
			}
			if err := json.Unmarshal(body, &result); err == nil {
				// 成功解析 JSON，更新节点信息
				if result.NodeID > 0 && result.NodeIP != "" {
					nodeInfo.Lock()
					nodeInfo.nodeID = result.NodeID
					nodeInfo.nodeIP = result.NodeIP
					nodeInfo.Unlock()
					r.logger.Debug("心跳响应解析成功，已更新节点信息",
						zap.Uint("node_id", result.NodeID),
						zap.String("node_ip", result.NodeIP))
				}
			} else {
				// 不是 JSON 格式，可能是旧格式的 "done"，忽略
				r.logger.Debug("心跳响应为旧格式，跳过解析")
			}
		}
		r.logger.Debug("心跳发送成功")
	} else {
		r.logger.Warn("心跳发送失败", zap.Int("status", resp.StatusCode))
	}
}

