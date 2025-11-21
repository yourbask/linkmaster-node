package handler

import (
	"net"
	"time"

	"github.com/gin-gonic/gin"
)

func handleDns(c *gin.Context, url string, params map[string]interface{}) {
	// 执行DNS查询
	start := time.Now()
	ips, err := net.LookupIP(url)
	lookupTime := time.Since(start).Milliseconds()

	if err != nil {
		c.JSON(200, gin.H{
			"type":  "ceDns",
			"url":   url,
			"error": err.Error(),
		})
		return
	}

	// 格式化IP列表
	ipList := make([]map[string]interface{}, 0)
	for _, ip := range ips {
		ipType := "A"
		if ip.To4() == nil {
			ipType = "AAAA"
		}
		ipList = append(ipList, map[string]interface{}{
			"type": ipType,
			"ip":   ip.String(),
		})
	}

	c.JSON(200, gin.H{
		"type":      "ceDns",
		"url":       url,
		"ips":       ipList,
		"lookup_time": lookupTime,
	})
}

