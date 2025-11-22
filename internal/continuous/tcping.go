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
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.StopCh:
			return
		default:
			// 检查是否超过最大运行时长
			t.mu.RLock()
			if time.Since(t.StartTime) > t.MaxDuration {
				t.mu.RUnlock()
				t.Stop()
				return
			}
			t.mu.RUnlock()

			// 检查任务是否已停止
			t.mu.RLock()
			isRunning := t.IsRunning
			t.mu.RUnlock()
			if !isRunning {
				return
			}
			
			// 执行tcping测试（每次测试完成后立即返回结果）
			result := t.executeTCPing()
			
			// 再次检查任务是否已停止（执行完成后）
			t.mu.RLock()
			isRunning = t.IsRunning
			t.mu.RUnlock()
			if !isRunning {
				return
			}
			
			if resultCallback != nil {
				resultCallback(result)
			}

			// 等待间隔时间后继续下一次测试
			select {
			case <-ctx.Done():
				return
			case <-t.StopCh:
				return
			case <-time.After(t.Interval):
				// 继续下一次循环
			}
		}
	}
}

func (t *TCPingTask) Stop() {
	t.mu.Lock()
	if !t.IsRunning {
		t.mu.Unlock()
		return
	}
	t.IsRunning = false
	t.mu.Unlock()
	
	// 关闭停止通道
	select {
	case <-t.StopCh:
		// 已经关闭
	default:
		close(t.StopCh)
	}
	
	t.logger.Info("TCPing任务已停止", zap.String("task_id", t.TaskID))
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

	// 提取目标IP
	var targetIP string
	if conn != nil {
		if addr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
			targetIP = addr.IP.String()
		}
		defer conn.Close()
	}

	// 如果连接失败，从host解析
	if targetIP == "" {
		ips, err := net.LookupIP(t.Host)
		if err == nil && len(ips) > 0 {
			// 优先使用IPv4
			for _, ip := range ips {
				if ip.To4() != nil {
					targetIP = ip.String()
					break
				}
			}
			if targetIP == "" && len(ips) > 0 {
				targetIP = ips[0].String()
			}
		}
	}

	if err != nil {
		return map[string]interface{}{
			"timestamp":   time.Now().Unix(),
			"latency":     -1,
			"success":     false,
			"packet_loss": true,
			"ip":          targetIP,
			"error":       err.Error(),
		}
	}

	return map[string]interface{}{
		"timestamp":   time.Now().Unix(),
		"latency":     float64(latency),
		"success":     true,
		"packet_loss": false,
		"ip":          targetIP,
	}
}

