package handler

import (
	"os/exec"
	"strings"

	"github.com/gin-gonic/gin"
)

func handleTrace(c *gin.Context, url string, params map[string]interface{}) {
	// 执行traceroute命令
	cmd := exec.Command("traceroute", url)
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(200, gin.H{
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
		"type":         "ceTrace",
		"url":          url,
		"trace_result": traceResult,
	})
}

