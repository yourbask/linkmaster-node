package continuous

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

type PingTask struct {
	TaskID      string
	Target      string
	Interval    time.Duration
	MaxDuration time.Duration
	StartTime   time.Time
	LastRequest time.Time
	StopCh      chan struct{}
	IsRunning   bool
	mu          sync.RWMutex
	logger      *zap.Logger
}

func NewPingTask(taskID, target string, interval, maxDuration time.Duration) *PingTask {
	logger, _ := zap.NewProduction()
	return &PingTask{
		TaskID:      taskID,
		Target:      target,
		Interval:    interval,
		MaxDuration: maxDuration,
		StartTime:   time.Now(),
		LastRequest: time.Now(),
		StopCh:      make(chan struct{}),
		IsRunning:   true,
		logger:      logger,
	}
}

func (t *PingTask) Start(ctx context.Context, resultCallback func(result map[string]interface{})) {
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

			// 执行ping测试
			result := t.executePing()
			if resultCallback != nil {
				resultCallback(result)
			}
		}
	}
}

func (t *PingTask) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.IsRunning {
		t.IsRunning = false
		close(t.StopCh)
	}
}

func (t *PingTask) UpdateLastRequest() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.LastRequest = time.Now()
}

func (t *PingTask) executePing() map[string]interface{} {
	cmd := exec.Command("ping", "-c", "4", t.Target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return map[string]interface{}{
			"timestamp":   time.Now().Unix(),
			"latency":     -1,
			"success":     false,
			"packet_loss": true,
			"error":       err.Error(),
		}
	}

	// 解析ping输出
	result := parsePingOutput(string(output))
	result["timestamp"] = time.Now().Unix()
	return result
}

func parsePingOutput(output string) map[string]interface{} {
	result := map[string]interface{}{
		"latency":     float64(0),
		"success":     true,
		"packet_loss": false,
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// 解析丢包率：4 packets transmitted, 4 received, 0% packet loss
		if strings.Contains(line, "packets transmitted") {
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "packet" && i+2 < len(parts) {
					// 查找百分比
					lossStr := strings.Trim(parts[i+1], "%")
					if loss, err := strconv.ParseFloat(lossStr, 64); err == nil {
						result["packet_loss"] = loss > 0
						if loss > 0 {
							result["success"] = false
						}
					}
				}
			}
		}
		
		// 解析延迟：rtt min/avg/max/mdev = 10.123/12.456/15.789/2.345 ms
		if strings.Contains(line, "min/avg/max") || strings.Contains(line, "rtt") {
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.Contains(part, "/") && !strings.Contains(part, "=") {
					times := strings.Split(part, "/")
					if len(times) >= 2 {
						if avg, err := strconv.ParseFloat(times[1], 64); err == nil {
							result["latency"] = avg
							break
						}
					}
				}
			}
		}
	}

	return result
}

