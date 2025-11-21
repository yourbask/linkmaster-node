package handler

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func handleSocket(c *gin.Context, url string, params map[string]interface{}) {
	// 解析host:port格式
	parts := strings.Split(url, ":")
	if len(parts) != 2 {
		c.JSON(200, gin.H{
			"type":  "ceSocket",
			"url":   url,
			"error": "格式错误，需要 host:port",
		})
		return
	}

	host := parts[0]
	portStr := parts[1]
	port, err := strconv.Atoi(portStr)
	if err != nil {
		c.JSON(200, gin.H{
			"type":  "ceSocket",
			"url":   url,
			"error": "端口格式错误",
		})
		return
	}

	// 执行TCP连接测试
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, portStr), 5*time.Second)
	if err != nil {
		c.JSON(200, gin.H{
			"type":   "ceSocket",
			"url":    url,
			"host":   host,
			"port":   port,
			"result": "false",
			"error":  err.Error(),
		})
		return
	}
	defer conn.Close()

	c.JSON(200, gin.H{
		"type":   "ceSocket",
		"url":    url,
		"host":   host,
		"port":   port,
		"result": "true",
	})
}

