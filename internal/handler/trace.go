package handler

import (
	"os/exec"
	"strings"

	"github.com/gin-gonic/gin"
)

func handleTrace(c *gin.Context, url string, params map[string]interface{}) {
	// 获取seq参数
	seq := ""
	if seqVal, ok := params["seq"].(string); ok {
		seq = seqVal
	}

	// 解析URL，提取hostname
	hostname := url
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		parts := strings.Split(url, "//")
		if len(parts) > 1 {
			hostParts := strings.Split(parts[1], "/")
			hostname = hostParts[0]
			if idx := strings.Index(hostname, ":"); idx != -1 {
				hostname = hostname[:idx]
			}
		}
	}

	// 执行traceroute命令
	cmd := exec.Command("traceroute", "-m", "30", "-n", hostname)
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(200, gin.H{
			"seq":   seq,
			"type":  "ceTrace",
			"url":   url,
			"error": err.Error(),
		})
		return
	}

	// 解析输出
	lines := strings.Split(string(output), "\n")
	traceResult := make([]string, 0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			traceResult = append(traceResult, line)
		}
	}

	c.JSON(200, gin.H{
		"seq":          seq,
		"type":         "ceTrace",
		"url":          url,
		"trace_result": traceResult,
	})
}

