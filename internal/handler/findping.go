package handler

import (
	"net"
	"os/exec"
	"sync"

	"github.com/gin-gonic/gin"
)

func handleFindPing(c *gin.Context, url string, params map[string]interface{}) {
	// 获取seq参数
	seq := ""
	if seqVal, ok := params["seq"].(string); ok {
		seq = seqVal
	}

	// url应该是CIDR格式，如 8.8.8.0/24
	cidr := url
	if cidrParam, ok := params["cidr"].(string); ok && cidrParam != "" {
		cidr = cidrParam
	}

	// 解析CIDR
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		c.JSON(200, gin.H{
			"seq":    seq,
			"type":   "ceFindPing",
			"error":  "无效的CIDR格式",
		})
		return
	}

	// 生成IP列表
	var ipList []string
	for ip := ipNet.IP.Mask(ipNet.Mask); ipNet.Contains(ip); incIP(ip) {
		ipList = append(ipList, ip.String())
	}

	// 移除网络地址和广播地址
	if len(ipList) > 2 {
		ipList = ipList[1 : len(ipList)-1]
	}

	// 并发ping测试
	var wg sync.WaitGroup
	var mu sync.Mutex
	aliveIPs := make([]string, 0)

	// 限制并发数
	semaphore := make(chan struct{}, 50)

	for _, ip := range ipList {
		wg.Add(1)
		semaphore <- struct{}{}
		go func(ipAddr string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			// 执行ping（只ping一次，快速检测）
			cmd := exec.Command("ping", "-c", "1", "-W", "1", ipAddr)
			err := cmd.Run()
			if err == nil {
				mu.Lock()
				aliveIPs = append(aliveIPs, ipAddr)
				mu.Unlock()
			}
		}(ip)
	}

	wg.Wait()

	c.JSON(200, gin.H{
		"seq":         seq,
		"type":        "ceFindPing",
		"cidr":        cidr,
		"alive_ips":   aliveIPs,
		"alive_count": len(aliveIPs),
		"total_ips":   len(ipList),
	})
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

