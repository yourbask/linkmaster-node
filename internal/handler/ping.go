package handler

import (
	"net"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func handlePing(c *gin.Context, url string, params map[string]interface{}) {
	// 执行ping命令
	cmd := exec.Command("ping", "-c", "4", url)
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(200, gin.H{
			"type":  "cePing",
			"url":   url,
			"error": err.Error(),
		})
		return
	}

	// 解析ping输出
	result := parsePingOutput(string(output), url)
	c.JSON(200, result)
}

func parsePingOutput(output, url string) map[string]interface{} {
	result := map[string]interface{}{
		"type": "cePing",
		"url":  url,
		"ip":   "",
	}

	// 解析IP地址
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "PING") {
			// 提取IP地址
			parts := strings.Fields(line)
			for _, part := range parts {
				if net.ParseIP(part) != nil {
					result["ip"] = part
					break
				}
			}
		}
		if strings.Contains(line, "packets transmitted") {
			// 解析丢包率
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "packet" && i+2 < len(parts) {
					if loss, err := strconv.ParseFloat(strings.Trim(parts[i+1], "%"), 64); err == nil {
						result["packets_losrat"] = loss
					}
				}
			}
		}
		if strings.Contains(line, "min/avg/max") {
			// 解析延迟统计
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

	return result
}

