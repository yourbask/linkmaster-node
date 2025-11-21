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

	// 执行TCP连接测试
	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, portStr), 5*time.Second)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		c.JSON(200, gin.H{
			"seq":    seq,
			"type":   "ceTCPing",
			"url":    url,
			"ip":     primaryIP,
			"host":   host,
			"port":   port,
			"latency": -1,
			"error":  err.Error(),
		})
		return
	}
	defer conn.Close()

	// 如果之前没有获取到IP，从连接中获取
	if primaryIP == "" {
		if addr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
			primaryIP = addr.IP.String()
		}
	}

	c.JSON(200, gin.H{
		"seq":     seq,
		"type":    "ceTCPing",
		"url":     url,
		"ip":      primaryIP,
		"host":    host,
		"port":    port,
		"latency": latency,
		"success": true,
	})
}

