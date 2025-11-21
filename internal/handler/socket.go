package handler

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func handleSocket(c *gin.Context, url string, params map[string]interface{}) {
	// 获取seq参数
	seq := ""
	if seqVal, ok := params["seq"].(string); ok {
		seq = seqVal
	}

	// 解析host:port格式
	var host, portStr string
	var port int

	// 尝试从URL中解析
	if strings.Contains(url, ":") {
		parts := strings.Split(url, ":")
		if len(parts) >= 2 {
			host = parts[0]
			portStr = parts[len(parts)-1]
		}
	} else {
		// 尝试从params中获取
		if hostVal, ok := params["host"].(string); ok {
			host = hostVal
		}
		if portVal, ok := params["port"].(string); ok {
			portStr = portVal
		} else if portVal, ok := params["port"].(float64); ok {
			portStr = strconv.Itoa(int(portVal))
		}
	}

	// 如果host为空，尝试从URL解析
	if host == "" {
		if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
			parts := strings.Split(url, "//")
			if len(parts) > 1 {
				hostParts := strings.Split(parts[1], "/")
				hostPort := hostParts[0]
				if idx := strings.Index(hostPort, ":"); idx != -1 {
					host = hostPort[:idx]
					portStr = hostPort[idx+1:]
				} else {
					host = hostPort
					if strings.HasPrefix(url, "https://") {
						portStr = "443"
					} else {
						portStr = "80"
					}
				}
			}
		} else {
			host = url
			portStr = "80"
		}
	}

	// 解析端口
	var err error
	port, err = strconv.Atoi(portStr)
	if err != nil {
		c.JSON(200, gin.H{
			"seq":   seq,
			"type":  "ceSocket",
			"url":   url,
			"error": "端口格式错误",
		})
		return
	}

	// 准备结果
	result := map[string]interface{}{
		"seq":   seq,
		"type":  "ceSocket",
		"url":   url,
		"port":  port,
		"ip":    "",
		"result": "false",
	}

	// 解析域名或IP
	var ip string
	parsedIP := net.ParseIP(host)
	if parsedIP != nil {
		ip = host
	} else {
		// DNS解析
		ips, err := net.LookupIP(host)
		if err != nil {
			result["ip"] = ""
			result["result"] = "域名无法解析"
			c.JSON(200, result)
			return
		}
		if len(ips) > 0 {
			ip = ips[0].String()
		}
	}

	result["ip"] = ip

	// 检查IP是否有效
	if ip == "" || ip == "0.0.0.0" || ip == "127.0.0.0" {
		result["result"] = "false"
		c.JSON(200, result)
		return
	}

	// 执行TCP连接测试
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, portStr), 5*time.Second)
	if err != nil {
		result["result"] = "false"
		if err.Error() != "" {
			result["error"] = err.Error()
		}
		c.JSON(200, result)
		return
	}
	defer conn.Close()

	result["result"] = "true"
	c.JSON(200, result)
}
