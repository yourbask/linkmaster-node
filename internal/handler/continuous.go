package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"linkmaster-node/internal/config"
	"linkmaster-node/internal/continuous"
	"linkmaster-node/internal/heartbeat"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var continuousTasks = make(map[string]*ContinuousTask)
var taskMutex sync.RWMutex
var backendURL string
var logger *zap.Logger

func InitContinuousHandler(cfg *config.Config) {
	backendURL = cfg.Backend.URL
	logger, _ = zap.NewProduction()
}

type ContinuousTask struct {
	TaskID      string
	Type        string
	Target      string
	Interval    time.Duration
	MaxDuration time.Duration
	StartTime   time.Time
	LastRequest time.Time
	StopCh      chan struct{}
	IsRunning   bool
	pingTask    *continuous.PingTask
	tcpingTask  *continuous.TCPingTask
}

func HandleContinuousStart(c *gin.Context) {
	var req struct {
		Type        string `json:"type" binding:"required"`
		Target      string `json:"target" binding:"required"`
		Interval    int    `json:"interval"`     // 秒
		MaxDuration int    `json:"max_duration"` // 分钟
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 生成任务ID
	taskID := generateTaskID()

	// 设置默认值
	interval := 10 * time.Second
	if req.Interval > 0 {
		interval = time.Duration(req.Interval) * time.Second
	}

	maxDuration := 60 * time.Minute
	if req.MaxDuration > 0 {
		maxDuration = time.Duration(req.MaxDuration) * time.Minute
	}

	// 创建任务
	task := &ContinuousTask{
		TaskID:      taskID,
		Type:        req.Type,
		Target:      req.Target,
		Interval:    interval,
		MaxDuration: maxDuration,
		StartTime:   time.Now(),
		LastRequest: time.Now(),
		StopCh:      make(chan struct{}),
		IsRunning:   true,
	}

	// 根据类型创建对应的任务
	if req.Type == "ping" {
		pingTask := continuous.NewPingTask(taskID, req.Target, interval, maxDuration)
		task.pingTask = pingTask
	} else if req.Type == "tcping" {
		tcpingTask, err := continuous.NewTCPingTask(taskID, req.Target, interval, maxDuration)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		task.tcpingTask = tcpingTask
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的持续测试类型"})
		return
	}

	taskMutex.Lock()
	continuousTasks[taskID] = task
	taskMutex.Unlock()

	// 启动持续测试goroutine
	ctx := context.Background()
	if task.pingTask != nil {
		go task.pingTask.Start(ctx, func(result map[string]interface{}) {
			pushResultToBackend(taskID, result)
		})
	} else if task.tcpingTask != nil {
		go task.tcpingTask.Start(ctx, func(result map[string]interface{}) {
			pushResultToBackend(taskID, result)
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"task_id": taskID,
	})
}

func HandleContinuousStop(c *gin.Context) {
	var req struct {
		TaskID string `json:"task_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	taskMutex.Lock()
	task, exists := continuousTasks[req.TaskID]
	if exists {
		task.IsRunning = false
		if task.pingTask != nil {
			task.pingTask.Stop()
		}
		if task.tcpingTask != nil {
			task.tcpingTask.Stop()
		}
		close(task.StopCh)
		delete(continuousTasks, req.TaskID)
	}
	taskMutex.Unlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "任务已停止"})
}

func HandleContinuousStatus(c *gin.Context) {
	taskID := c.Query("task_id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task_id参数缺失"})
		return
	}

	taskMutex.RLock()
	task, exists := continuousTasks[taskID]
	if exists {
		// 更新LastRequest时间
		task.LastRequest = time.Now()
		if task.pingTask != nil {
			task.pingTask.UpdateLastRequest()
		}
		if task.tcpingTask != nil {
			task.tcpingTask.UpdateLastRequest()
		}
	}
	taskMutex.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"task_id":      task.TaskID,
		"is_running":   task.IsRunning,
		"start_time":   task.StartTime,
		"last_request": task.LastRequest,
	})
}

func pushResultToBackend(taskID string, result map[string]interface{}) {
	// 确保result包含必要的字段
	if result["timestamp"] == nil {
		result["timestamp"] = time.Now().Unix()
	}
	if result["latency"] == nil {
		result["latency"] = 0.0
	}
	if result["success"] == nil {
		result["success"] = true
	}
	if result["packet_loss"] == nil {
		result["packet_loss"] = false
	}

	// 推送结果到后端
	url := fmt.Sprintf("%s/api/public/node/continuous/result", backendURL)
	
	// 优先使用心跳返回的节点信息
	nodeID := heartbeat.GetNodeID()
	nodeIP := heartbeat.GetNodeIP()
	
	// 如果心跳还没有返回节点信息，使用本地IP作为后备
	if nodeIP == "" {
		nodeIP = getLocalIP()
		logger.Debug("使用本地IP作为后备", zap.String("node_ip", nodeIP))
	}
	
	// 发送 node_id 和 node_ip，后端可以通过这些信息精准匹配
	data := map[string]interface{}{
		"task_id": taskID,
		"result":  result,
	}
	
	// 如果 node_id 存在，优先发送 node_id
	if nodeID > 0 {
		data["node_id"] = nodeID
		logger.Debug("推送结果时使用存储的node_id", zap.Uint("node_id", nodeID))
	}
	
	// 如果 node_ip 存在，发送 node_ip
	if nodeIP != "" {
		data["node_ip"] = nodeIP
		logger.Debug("推送结果时使用存储的node_ip", zap.String("node_ip", nodeIP))
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		logger.Error("序列化结果失败", zap.Error(err), zap.String("task_id", taskID))
		return
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		logger.Error("创建请求失败", zap.Error(err), zap.String("task_id", taskID))
		return
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Warn("推送结果失败，继续运行", 
			zap.Error(err), 
			zap.String("task_id", taskID),
			zap.String("url", url))
		// 推送失败不停止任务，继续运行
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Warn("推送结果失败，继续运行", 
			zap.Int("status", resp.StatusCode),
			zap.String("task_id", taskID),
			zap.String("url", url),
			zap.String("response", string(body)))
		// 推送失败不停止任务，继续运行
		return
	}

	logger.Debug("推送结果成功", zap.String("task_id", taskID))
}

func getLocalIP() string {
	// 简化实现：返回第一个非回环IP
	// 实际应该获取外网IP
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String()
			}
		}
	}
	
	return "127.0.0.1"
}

func generateTaskID() string {
	return fmt.Sprintf("task_%d", time.Now().UnixNano())
}

// 定期清理超时任务
func StartTaskCleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for range ticker.C {
			now := time.Now()
			taskMutex.Lock()
			for taskID, task := range continuousTasks {
				// 检查最大运行时长
				if now.Sub(task.StartTime) > task.MaxDuration {
					logger.Info("任务达到最大运行时长，自动停止", zap.String("task_id", taskID))
					task.IsRunning = false
					if task.pingTask != nil {
						task.pingTask.Stop()
					}
					if task.tcpingTask != nil {
						task.tcpingTask.Stop()
					}
					delete(continuousTasks, taskID)
					continue
				}
				// 检查无客户端连接（30分钟无请求）
				if now.Sub(task.LastRequest) > 30*time.Minute {
					logger.Info("任务无客户端连接，自动停止", zap.String("task_id", taskID))
					task.IsRunning = false
					if task.pingTask != nil {
						task.pingTask.Stop()
					}
					if task.tcpingTask != nil {
						task.tcpingTask.Stop()
					}
					delete(continuousTasks, taskID)
				}
			}
			taskMutex.Unlock()
		}
	}()
}

