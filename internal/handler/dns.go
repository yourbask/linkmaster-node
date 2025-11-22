package handler

import (
	"encoding/base64"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func handleDns(c *gin.Context, url string, params map[string]interface{}) {
	// 获取seq参数
	seq := ""
	if seqVal, ok := params["seq"].(string); ok {
		seq = seqVal
	}

	// 获取dig类型参数
	digType := ""
	if dt, ok := params["dt"].(string); ok {
		digType = dt
	}

	// 获取DNS服务器参数
	dnsServer := ""
	if ds, ok := params["ds"].(string); ok {
		dnsServer = ds
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

	// 准备结果
	result := map[string]interface{}{
		"seq":    seq,
		"type":   "ceDns",
		"requrl": hostname,
		"ips":    []interface{}{},
		"cnames": []interface{}{},
	}

	// 构建dig命令
	args := []string{hostname}
	if digType != "" {
		args = append([]string{"-t", digType}, args...)
	}
	if dnsServer != "" {
		args = append([]string{"@" + dnsServer}, args...)
	}

	cmd := exec.Command("dig", args...)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// 编码完整输出为base64（header字段）
	result["header"] = base64.StdEncoding.EncodeToString([]byte(outputStr))

	if err != nil {
		result["error"] = err.Error()
		c.JSON(200, result)
		return
	}

	// 解析dig输出
	lines := strings.Split(outputStr, "\n")
	inAnswerSection := false
	ipList := make([]map[string]interface{}, 0)
	cnameList := make([]map[string]interface{}, 0)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "ANSWER SECTION") {
			inAnswerSection = true
			continue
		}
		if inAnswerSection {
			if line == "" {
				break
			}
			// 解析dig输出行，格式如：example.com.  300  IN  A  192.168.1.1
			// 或者：www.baidu.com.  430  IN  CNAME  www.a.shifen.com.
			parts := strings.Fields(line)
			if len(parts) >= 5 {
				// parts[0] = 域名
				// parts[1] = TTL
				// parts[2] = "IN" (class)
				// parts[3] = 记录类型 (A, AAAA, CNAME等)
				// parts[4] = 记录值
				recordClass := parts[3] // A, AAAA, CNAME等
				recordValue := ""
				if len(parts) >= 5 {
					recordValue = strings.Join(parts[4:], " ")
					// 移除值末尾的点（如果有）
					recordValue = strings.TrimSuffix(recordValue, ".")
				}

				domain := strings.TrimSuffix(parts[0], ".") // 移除域名末尾的点

				// 分别处理A/AAAA记录和CNAME记录
				if recordClass == "A" || recordClass == "AAAA" {
					ipItem := map[string]interface{}{
						"url":  domain,
						"type": recordClass,
						"ip":   recordValue,
					}
					ipList = append(ipList, ipItem)
				} else if recordClass == "CNAME" {
					cnameItem := map[string]interface{}{
						"url":  domain,
						"type": recordClass,
						"cname": recordValue, // CNAME值
					}
					cnameList = append(cnameList, cnameItem)
				}
			}
		}
	}

	// 如果没有从dig输出解析到IP，尝试使用net.LookupIP
	if len(ipList) == 0 {
		start := time.Now()
		ips, err := net.LookupIP(hostname)
		lookupTime := time.Since(start)

		if err == nil {
			for _, ip := range ips {
				ipType := "A"
				if ip.To4() == nil {
					ipType = "AAAA"
				}
				ipItem := map[string]interface{}{
					"url":  hostname,
					"type": ipType,
					"ip":   ip.String(),
				}
				ipList = append(ipList, ipItem)
			}
			// 更新header，包含lookup时间信息
			lookupInfo := fmt.Sprintf("Lookup time: %v\n", lookupTime)
			result["header"] = base64.StdEncoding.EncodeToString([]byte(outputStr + lookupInfo))
		} else {
			result["error"] = err.Error()
		}
	}

	result["ips"] = ipList
	result["cnames"] = cnameList
	c.JSON(200, result)
}
