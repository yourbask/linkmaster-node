package handler

import (
	"encoding/base64"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func handlePing(c *gin.Context, url string, params map[string]interface{}) {
	// 获取seq参数
	seq := ""
	if seqVal, ok := params["seq"].(string); ok {
		seq = seqVal
	}

	// 解析URL，提取hostname
	hostname := url
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		// 从URL中提取hostname
		parts := strings.Split(url, "//")
		if len(parts) > 1 {
			hostParts := strings.Split(parts[1], "/")
			hostname = hostParts[0]
			// 移除端口号
			if idx := strings.Index(hostname, ":"); idx != -1 {
				hostname = hostname[:idx]
			}
		}
	}

	// 执行ping命令
	cmd := exec.Command("ping", "-c", "10", "-i", "0.5", hostname)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// 准备结果
	result := map[string]interface{}{
		"seq":  seq,
		"type": "cePing",
		"url":  url,
		"ip":   "",
	}

	// 编码完整输出为base64（header字段）
	result["header"] = base64.StdEncoding.EncodeToString([]byte(outputStr))

	if err != nil {
		result["error"] = err.Error()
		c.JSON(200, result)
		return
	}

	// 解析ping输出
	lines := strings.Split(outputStr, "\n")

	// 解析IP地址（从PING行）
	for _, line := range lines {
		if strings.Contains(line, "PING") {
			// 提取IP地址，格式如：PING example.com (192.168.1.1) 56(84) bytes of data.
			re := regexp.MustCompile(`\(([0-9.]+)\)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				result["ip"] = matches[1]
			} else {
				// 尝试直接解析IP
				parts := strings.Fields(line)
				for _, part := range parts {
					if ip := net.ParseIP(part); ip != nil {
						result["ip"] = part
						break
					}
				}
			}
			break
		}
	}

	// 解析包大小（bytes字段）
	for _, line := range lines {
		if strings.Contains(line, "bytes of data") {
			// 提取bytes，格式如：64 bytes from ...
			re := regexp.MustCompile(`(\d+)\s+bytes`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				result["bytes"] = matches[1]
			}
			break
		}
	}

	// 解析统计信息
	for _, line := range lines {
		// 解析丢包率和包统计
		if strings.Contains(line, "packets transmitted") {
			// 格式如：10 packets transmitted, 10 received, 0% packet loss
			re := regexp.MustCompile(`(\d+)\s+packets\s+transmitted[,\s]+(\d+)\s+received[,\s]+(\d+(?:\.\d+)?)%`)
			matches := re.FindStringSubmatch(line)
			if len(matches) >= 4 {
				result["packets_total"] = matches[1]
				result["packets_recv"] = matches[2]
				if loss, err := strconv.ParseFloat(matches[3], 64); err == nil {
					result["packets_losrat"] = loss
				}
			} else {
				// 备用解析方式
				parts := strings.Fields(line)
				for i, part := range parts {
					if part == "packets" && i+1 < len(parts) {
						if total, err := strconv.Atoi(parts[i-1]); err == nil {
							result["packets_total"] = strconv.Itoa(total)
						}
					}
					if part == "received" && i-1 >= 0 {
						if recv, err := strconv.Atoi(parts[i-1]); err == nil {
							result["packets_recv"] = strconv.Itoa(recv)
						}
					}
					if part == "packet" && i+2 < len(parts) {
						if loss, err := strconv.ParseFloat(strings.Trim(parts[i+1], "%"), 64); err == nil {
							result["packets_losrat"] = loss
						}
					}
				}
			}
		}

		// 解析时间统计（min/avg/max）
		if strings.Contains(line, "min/avg/max") || strings.Contains(line, "rtt min/avg/max") {
			// 格式如：rtt min/avg/max/mdev = 10.123/12.456/15.789/2.345 ms
			re := regexp.MustCompile(`=\s*([0-9.]+)/([0-9.]+)/([0-9.]+)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) >= 4 {
				if min, err := strconv.ParseFloat(matches[1], 64); err == nil {
					result["time_min"] = min
				}
				if avg, err := strconv.ParseFloat(matches[2], 64); err == nil {
					result["time_avg"] = avg
				}
				if max, err := strconv.ParseFloat(matches[3], 64); err == nil {
					result["time_max"] = max
				}
			} else {
				// 备用解析方式
				parts := strings.Fields(line)
				for _, part := range parts {
					if strings.Contains(part, "/") {
						times := strings.Split(part, "/")
						if len(times) >= 3 {
							if min, err := strconv.ParseFloat(times[0], 64); err == nil {
								result["time_min"] = min
							}
							if avg, err := strconv.ParseFloat(times[1], 64); err == nil {
								result["time_avg"] = avg
							}
							if max, err := strconv.ParseFloat(times[2], 64); err == nil {
								result["time_max"] = max
							}
						}
					}
				}
			}
		}
	}

	c.JSON(200, result)
}
