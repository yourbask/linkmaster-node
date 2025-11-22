package continuous

import (
	"bufio"
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
	targetIP    string // 存储目标IP，从ping输出中提取
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

			// 执行多个ping包测试，每个包完成后立即返回结果
			// 使用 -c 10 -i 0.5 发送10个包，间隔0.5秒，实时解析每个包的延迟
			t.executePingWithRealtimeCallback(resultCallback)

			// 等待间隔时间后继续下一次测试（缩短间隔，比如1秒）
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

func (t *PingTask) executePingWithRealtimeCallback(resultCallback func(result map[string]interface{})) {
	// 发送10个ping包，间隔0.5秒，实时解析每个包的延迟
	// 使用 -c 10 -i 0.5 发送10个包
	cmd := exec.Command("ping", "-c", "10", "-i", "0.5", t.Target)
	
	// 获取标准输出管道，实时读取
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		if resultCallback != nil {
			resultCallback(map[string]interface{}{
				"timestamp":   time.Now().Unix(),
				"latency":     -1,
				"success":     false,
				"packet_loss": true,
				"error":       err.Error(),
			})
		}
		return
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		if resultCallback != nil {
			resultCallback(map[string]interface{}{
				"timestamp":   time.Now().Unix(),
				"latency":     -1,
				"success":     false,
				"packet_loss": true,
				"error":       err.Error(),
			})
		}
		return
	}

	// 使用bufio.Scanner实时读取每一行
	scanner := bufio.NewScanner(stdout)
	processedPackets := make(map[int]bool) // 用于去重，避免重复处理同一个包（通过icmp_seq）
	
	// 在goroutine中读取输出，避免阻塞
	go func() {
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			
			// 从PING行提取目标IP：PING example.com (8.8.8.8) 56(84) bytes of data.
			if strings.HasPrefix(line, "PING") {
				t.mu.RLock()
				currentTargetIP := t.targetIP
				t.mu.RUnlock()
				
				if currentTargetIP == "" {
					// 尝试从括号中提取IP：PING example.com (8.8.8.8)
					startIdx := strings.Index(line, "(")
					endIdx := strings.Index(line, ")")
					if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
						t.mu.Lock()
						t.targetIP = line[startIdx+1 : endIdx]
						t.mu.Unlock()
					} else {
						// 如果没有括号，尝试从"from"后提取：64 bytes from 8.8.8.8:
						fromIdx := strings.Index(line, "from")
						if fromIdx != -1 {
							parts := strings.Fields(line[fromIdx+4:])
							if len(parts) > 0 {
								ipPart := strings.TrimSuffix(parts[0], ":")
								t.mu.Lock()
								t.targetIP = ipPart
								t.mu.Unlock()
							}
						}
					}
				}
			}
			
			// 解析单个ping包的响应时间：64 bytes from 8.8.8.8: icmp_seq=0 ttl=64 time=10.123 ms
			if strings.Contains(line, "time=") && strings.Contains(line, "icmp_seq") {
				// 如果还没有提取到目标IP，从这一行提取
				t.mu.RLock()
				currentTargetIP := t.targetIP
				t.mu.RUnlock()
				
				if currentTargetIP == "" {
					// 格式：64 bytes from 8.8.8.8: icmp_seq=0
					fromIdx := strings.Index(line, "from")
					if fromIdx != -1 {
						afterFrom := line[fromIdx+4:]
						colonIdx := strings.Index(afterFrom, ":")
						if colonIdx != -1 {
							t.mu.Lock()
							t.targetIP = strings.TrimSpace(afterFrom[:colonIdx])
							t.mu.Unlock()
						}
					}
				}
				
				// 提取icmp_seq用于去重
				seqIndex := strings.Index(line, "icmp_seq=")
				seq := -1
				if seqIndex != -1 {
					seqPart := line[seqIndex+9:]
					spaceIndex := strings.Index(seqPart, " ")
					if spaceIndex == -1 {
						spaceIndex = len(seqPart)
					}
					if s, err := strconv.Atoi(seqPart[:spaceIndex]); err == nil {
						seq = s
					}
				}
				
				// 检查是否已处理过这个包
				if seq >= 0 && processedPackets[seq] {
					continue
				}
				if seq >= 0 {
					processedPackets[seq] = true
				}
				
				latency := parseSinglePacketLatency(line)
				if latency >= 0 && resultCallback != nil {
					t.mu.RLock()
					currentTargetIP := t.targetIP
					t.mu.RUnlock()
					
					result := map[string]interface{}{
						"timestamp":   time.Now().Unix(),
						"latency":     latency,
						"success":     true,
						"packet_loss": false,
					}
					if currentTargetIP != "" {
						result["ip"] = currentTargetIP
					}
					resultCallback(result)
				}
			} else if strings.Contains(line, "Request timeout") || strings.Contains(line, "no answer") {
				// 处理超时的包
				if resultCallback != nil {
					t.mu.RLock()
					currentTargetIP := t.targetIP
					t.mu.RUnlock()
					
					result := map[string]interface{}{
						"timestamp":   time.Now().Unix(),
						"latency":     -1,
						"success":     false,
						"packet_loss": true,
					}
					if currentTargetIP != "" {
						result["ip"] = currentTargetIP
					}
					resultCallback(result)
				}
			}
		}
	}()

	// 等待命令完成
	cmd.Wait()
}

// parseSinglePacketLatency 解析单个ping包的延迟时间
func parseSinglePacketLatency(line string) float64 {
	// 格式：64 bytes from 8.8.8.8: icmp_seq=0 ttl=64 time=10.123 ms
	timeIndex := strings.Index(line, "time=")
	if timeIndex == -1 {
		return -1
	}
	
	timePart := line[timeIndex+5:]
	spaceIndex := strings.Index(timePart, " ")
	if spaceIndex == -1 {
		return -1
	}
	
	timeStr := timePart[:spaceIndex]
	if latency, err := strconv.ParseFloat(timeStr, 64); err == nil {
		return latency
	}
	return -1
}

func (t *PingTask) executePing() map[string]interface{} {
	// 发送单个ping包（-c 1），每个包完成后立即返回结果
	cmd := exec.Command("ping", "-c", "1", t.Target)
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

	// 解析ping输出（单个包的结果）
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
		
		// 解析单个ping包的响应时间：64 bytes from 8.8.8.8: icmp_seq=0 ttl=64 time=10.123 ms
		if strings.Contains(line, "time=") && strings.Contains(line, "icmp_seq") {
			// 提取time=后面的数值
			timeIndex := strings.Index(line, "time=")
			if timeIndex != -1 {
				timePart := line[timeIndex+5:]
				spaceIndex := strings.Index(timePart, " ")
				if spaceIndex != -1 {
					timeStr := timePart[:spaceIndex]
					if latency, err := strconv.ParseFloat(timeStr, 64); err == nil {
						result["latency"] = latency
						result["success"] = true
						result["packet_loss"] = false
						return result
					}
				}
			}
		}
		
		// 解析丢包率：1 packets transmitted, 1 received, 0% packet loss
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
							result["latency"] = -1
						}
					}
				}
			}
		}
		
		// 解析延迟：rtt min/avg/max/mdev = 10.123/10.123/10.123/0.000 ms（单个包时min=avg=max）
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

