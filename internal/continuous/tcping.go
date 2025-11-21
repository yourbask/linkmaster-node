package continuous

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

type TCPingTask struct {
	TaskID      string
	Target      string
	Host        string
	Port        int
	Interval    time.Duration
	MaxDuration time.Duration
	StartTime   time.Time
	LastRequest time.Time
	StopCh      chan struct{}
	IsRunning   bool
	mu          sync.RWMutex
	logger      *zap.Logger
}

func NewTCPingTask(taskID, target string, interval, maxDuration time.Duration) (*TCPingTask, error) {
	// 解析host:port
	parts := strings.Split(target, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("无效的target格式，需要 host:port")
	}

	host := parts[0]
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("无效的端口: %v", err)
	}

	logger, _ := zap.NewProduction()
	return &TCPingTask{
		TaskID:      taskID,
		Target:      target,
		Host:        host,
		Port:        port,
		Interval:    interval,
		MaxDuration: maxDuration,
		StartTime:   time.Now(),
		LastRequest: time.Now(),
		StopCh:      make(chan struct{}),
		IsRunning:   true,
		logger:      logger,
	}, nil
}

func (t *TCPingTask) Start(ctx context.Context, resultCallback func(result map[string]interface{})) {
	ticker := time.NewTicker(t.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.StopCh:
			return
		case <-ticker.C:
			// 检查是否超过最大运行时长
			t.mu.RLock()
			if time.Since(t.StartTime) > t.MaxDuration {
				t.mu.RUnlock()
				t.Stop()
				return
			}
			t.mu.RUnlock()

			// 执行tcping测试
			result := t.executeTCPing()
			if resultCallback != nil {
				resultCallback(result)
			}
		}
	}
}

func (t *TCPingTask) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.IsRunning {
		t.IsRunning = false
		close(t.StopCh)
	}
}

func (t *TCPingTask) UpdateLastRequest() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.LastRequest = time.Now()
}

func (t *TCPingTask) executeTCPing() map[string]interface{} {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(t.Host, strconv.Itoa(t.Port)), 5*time.Second)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return map[string]interface{}{
			"timestamp":   time.Now().Unix(),
			"latency":     -1,
			"success":     false,
			"packet_loss": true,
			"error":       err.Error(),
		}
	}
	defer conn.Close()

	return map[string]interface{}{
		"timestamp":   time.Now().Unix(),
		"latency":     float64(latency),
		"success":     true,
		"packet_loss": false,
	}
}

