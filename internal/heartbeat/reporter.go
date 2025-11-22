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

// 节点信息存储（通过心跳更新，优先从配置文件读取）
var nodeInfo struct {
	sync.RWMutex
	nodeID    uint
	nodeIP    string
	country   string
	province  string
	city      string
	isp       string
	cfg       *config.Config
	initialized bool
}

// InitNodeInfo 初始化节点信息（从配置文件读取）
func InitNodeInfo(cfg *config.Config) {
	nodeInfo.Lock()
	defer nodeInfo.Unlock()
	
	nodeInfo.cfg = cfg
	nodeInfo.nodeID = cfg.Node.ID
	nodeInfo.nodeIP = cfg.Node.IP
	nodeInfo.country = cfg.Node.Country
	nodeInfo.province = cfg.Node.Province
	nodeInfo.city = cfg.Node.City
	nodeInfo.isp = cfg.Node.ISP
	nodeInfo.initialized = true
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

// GetNodeLocation 获取节点位置信息
func GetNodeLocation() (country, province, city, isp string) {
	nodeInfo.RLock()
	defer nodeInfo.RUnlock()
	return nodeInfo.country, nodeInfo.province, nodeInfo.city, nodeInfo.isp
}

type Reporter struct {
	cfg    *config.Config
	client *http.Client
	logger *zap.Logger
	stopCh chan struct{}
}

func NewReporter(cfg *config.Config) *Reporter {
	logger, _ := zap.NewProduction()
	
	// 初始化节点信息（从配置文件读取）
	InitNodeInfo(cfg)
	
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

// RegisterNode 注册节点（安装时或首次启动时调用）
func RegisterNode(cfg *config.Config) error {
	url := fmt.Sprintf("%s/api/node/heartbeat", cfg.Backend.URL)
	req, err := http.NewRequest("POST", url, bytes.NewBufferString("type=pingServer"))
	if err != nil {
		return fmt.Errorf("创建心跳请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("发送心跳失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("读取响应失败: %w", err)
		}

		// 尝试解析 JSON 响应
		var result struct {
			Status   string `json:"status"`
			NodeID   uint   `json:"node_id"`
			NodeIP   string `json:"node_ip"`
			Country  string `json:"country"`
			Province string `json:"province"`
			City     string `json:"city"`
			ISP      string `json:"isp"`
		}
		if err := json.Unmarshal(body, &result); err == nil {
			// 成功解析 JSON，更新配置文件和内存
			if result.NodeID > 0 && result.NodeIP != "" {
				cfg.Node.ID = result.NodeID
				cfg.Node.IP = result.NodeIP
				cfg.Node.Country = result.Country
				cfg.Node.Province = result.Province
				cfg.Node.City = result.City
				cfg.Node.ISP = result.ISP

				// 保存到配置文件
				if err := cfg.Save(); err != nil {
					return fmt.Errorf("保存配置文件失败: %w", err)
				}

				// 更新内存中的节点信息
				nodeInfo.Lock()
				nodeInfo.nodeID = result.NodeID
				nodeInfo.nodeIP = result.NodeIP
				nodeInfo.country = result.Country
				nodeInfo.province = result.Province
				nodeInfo.city = result.City
				nodeInfo.isp = result.ISP
				nodeInfo.cfg = cfg
				nodeInfo.initialized = true
				nodeInfo.Unlock()

				return nil
			}
		}
		return fmt.Errorf("心跳响应格式无效或节点信息不完整")
	}

	return fmt.Errorf("心跳请求失败，状态码: %d", resp.StatusCode)
}

func (r *Reporter) sendHeartbeat() {
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
				Status   string `json:"status"`
				NodeID   uint   `json:"node_id"`
				NodeIP   string `json:"node_ip"`
				Country  string `json:"country"`
				Province string `json:"province"`
				City     string `json:"city"`
				ISP      string `json:"isp"`
			}
			if err := json.Unmarshal(body, &result); err == nil {
				// 成功解析 JSON，检查是否有更新
				if result.NodeID > 0 && result.NodeIP != "" {
					nodeInfo.Lock()
					needUpdate := false
					if nodeInfo.nodeID != result.NodeID || nodeInfo.nodeIP != result.NodeIP ||
						nodeInfo.country != result.Country || nodeInfo.province != result.Province ||
						nodeInfo.city != result.City || nodeInfo.isp != result.ISP {
						needUpdate = true
					}

					if needUpdate {
						// 更新内存
						nodeInfo.nodeID = result.NodeID
						nodeInfo.nodeIP = result.NodeIP
						nodeInfo.country = result.Country
						nodeInfo.province = result.Province
						nodeInfo.city = result.City
						nodeInfo.isp = result.ISP

						// 更新配置文件
						if nodeInfo.cfg != nil {
							nodeInfo.cfg.Node.ID = result.NodeID
							nodeInfo.cfg.Node.IP = result.NodeIP
							nodeInfo.cfg.Node.Country = result.Country
							nodeInfo.cfg.Node.Province = result.Province
							nodeInfo.cfg.Node.City = result.City
							nodeInfo.cfg.Node.ISP = result.ISP
							if err := nodeInfo.cfg.Save(); err != nil {
								r.logger.Warn("保存节点信息到配置文件失败", zap.Error(err))
							}
						}
						nodeInfo.Unlock()

						r.logger.Info("节点信息已更新",
							zap.Uint("node_id", result.NodeID),
							zap.String("node_ip", result.NodeIP),
							zap.String("location", fmt.Sprintf("%s/%s/%s", result.Country, result.Province, result.City)))
					} else {
						nodeInfo.Unlock()
					}
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

