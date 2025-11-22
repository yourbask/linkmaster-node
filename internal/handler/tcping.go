package handler

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func handleTCPing(c *gin.Context, url string, params map[string]interface{}) {
	// 获取seq参数
	seq := ""
	if seqVal, ok := params["seq"].(string); ok {
		seq = seqVal
	}

	// 解析host:port格式
	parts := strings.Split(url, ":")
	if len(parts) != 2 {
		c.JSON(200, gin.H{
			"seq":    seq,
			"type":   "ceTCPing",
			"url":    url,
			"error":  "格式错误，需要 host:port",
		})
		return
	}

	host := parts[0]
	portStr := parts[1]
	port, err := strconv.Atoi(portStr)
	if err != nil {
		c.JSON(200, gin.H{
			"seq":    seq,
			"type":   "ceTCPing",
			"url":    url,
			"error":  "端口格式错误",
		})
		return
	}

	// 解析hostname获取IP
	var primaryIP string
	ips, err := net.LookupIP(host)
	if err == nil && len(ips) > 0 {
		// 优先使用IPv4
		for _, ip := range ips {
			if ip.To4() != nil {
				primaryIP = ip.String()
				break
			}
		}
		if primaryIP == "" && len(ips) > 0 {
			primaryIP = ips[0].String()
		}
	}

	// 执行多次TCP连接测试（默认10次，和PING一致）
	const testCount = 10
	var latencies []float64
	successCount := 0
	failureCount := 0

	for i := 0; i < testCount; i++ {
		start := time.Now()
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, portStr), 5*time.Second)
		latency := time.Since(start).Milliseconds()

		if err == nil {
			// 成功：记录延迟
			latencies = append(latencies, float64(latency))
			successCount++
			conn.Close()

			// 如果之前没有获取到IP，从连接中获取
			if primaryIP == "" {
				if addr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
					primaryIP = addr.IP.String()
				}
			}
		} else {
			// 失败：记录为丢包
			failureCount++
		}
	}

	// 计算统计信息
	packetsTotal := testCount
	packetsRecv := successCount
	packetsLosrat := float64(failureCount) / float64(testCount) * 100.0

	var timeMin, timeMax, timeAvg float64
	if len(latencies) > 0 {
		timeMin = latencies[0]
		timeMax = latencies[0]
		sum := 0.0
		for _, lat := range latencies {
			if lat < timeMin {
				timeMin = lat
			}
			if lat > timeMax {
				timeMax = lat
			}
			sum += lat
		}
		timeAvg = sum / float64(len(latencies))
	} else {
		// 全部失败
		timeMin = -1
		timeMax = -1
		timeAvg = -1
	}

	// 如果之前没有获取到IP，尝试从host解析
	if primaryIP == "" {
		ips, err := net.LookupIP(host)
		if err == nil && len(ips) > 0 {
			for _, ip := range ips {
				if ip.To4() != nil {
					primaryIP = ip.String()
					break
				}
			}
			if primaryIP == "" && len(ips) > 0 {
				primaryIP = ips[0].String()
			}
		}
	}

	// 返回格式和PING一致
	result := gin.H{
		"seq":             seq,
		"type":            "ceTCPing",
		"url":             url,
		"ip":              primaryIP,
		"host":            host,
		"port":            port,
		"packets_total":  strconv.Itoa(packetsTotal),
		"packets_recv":   strconv.Itoa(packetsRecv),
		"packets_losrat": packetsLosrat, // float64类型，百分比值（如10.5表示10.5%）
	}
	
	// 时间字段：如果是-1（全部失败），返回字符串"-"，否则返回float64
	if timeMin < 0 {
		result["time_min"] = "-"
		result["time_max"] = "-"
		result["time_avg"] = "-"
	} else {
		result["time_min"] = timeMin
		result["time_max"] = timeMax
		result["time_avg"] = timeAvg
	}

	// 如果全部失败，添加error字段
	if successCount == 0 {
		result["error"] = "所有TCP连接测试均失败"
	}

	c.JSON(200, result)
}

