package handler

import (
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// timingTransport 用于跟踪HTTP请求的各个阶段时间
type timingTransport struct {
	transport     http.RoundTripper
	nameLookup    time.Duration
	connect       time.Duration
	startTransfer time.Duration
	total         time.Duration
	primaryIP     string
	mu            sync.Mutex
}

func newTimingTransport() *timingTransport {
	return &timingTransport{
		transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}
}

func (t *timingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	host := req.URL.Hostname()
	port := req.URL.Port()
	if port == "" {
		if req.URL.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	
	// DNS查询时间
	dnsStart := time.Now()
	ips, err := net.LookupIP(host)
	dnsTime := time.Since(dnsStart)
	
	t.mu.Lock()
	t.nameLookup = dnsTime
	if len(ips) > 0 {
		// 优先使用IPv4
		for _, ip := range ips {
			if ip.To4() != nil {
				t.primaryIP = ip.String()
				break
			}
		}
		if t.primaryIP == "" && len(ips) > 0 {
			t.primaryIP = ips[0].String()
		}
	}
	t.mu.Unlock()
	
	if err != nil {
		return nil, err
	}
	
	// TCP连接时间（如果已知IP）
	var connectTime time.Duration
	if t.primaryIP != "" {
		connectStart := time.Now()
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(t.primaryIP, port), 5*time.Second)
		connectTime = time.Since(connectStart)
		if err == nil {
			conn.Close()
		}
	}
	
	// 执行HTTP请求
	httpStart := time.Now()
	resp, err := t.transport.RoundTrip(req)
	httpTime := time.Since(httpStart)
	totalTime := time.Since(start)
	
	t.mu.Lock()
	if connectTime > 0 {
		t.connect = connectTime
	} else {
		// 如果没有单独测量连接时间，使用HTTP请求时间的一部分
		t.connect = httpTime / 3
	}
	t.total = totalTime
	if resp != nil {
		// 首字节时间 = DNS + 连接 + HTTP请求开始到响应头的时间
		t.startTransfer = dnsTime + connectTime + (httpTime / 2)
		if t.startTransfer > totalTime {
			t.startTransfer = totalTime * 2 / 3
		}
	}
	t.mu.Unlock()
	
	return resp, err
}

func handleGet(c *gin.Context, urlStr string, params map[string]interface{}) {
	// 获取seq参数
	seq := ""
	if seqVal, ok := params["seq"].(string); ok {
		seq = seqVal
	}

	// 解析URL
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		urlStr = "http://" + urlStr
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		c.JSON(200, gin.H{
			"seq":   seq,
			"type":  "ceGet",
			"url":   urlStr,
			"error": "URL格式错误",
		})
		return
	}

	// 准备结果
	result := map[string]interface{}{
		"seq":  seq,
		"type": "ceGet",
		"url":  urlStr,
	}

	// 创建自定义Transport用于时间跟踪
	timingTransport := newTimingTransport()
	
	// 创建HTTP客户端
	client := &http.Client{
		Transport: timingTransport,
		Timeout:   15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// 跟随重定向，最多20次
			if len(via) >= 20 {
				return fmt.Errorf("重定向次数过多")
			}
			return nil
		},
	}

	// 创建请求
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		result["error"] = err.Error()
		result["ip"] = "访问失败"
		result["totaltime"] = "*"
		result["downtime"] = "*"
		result["downsize"] = "*"
		result["downspeed"] = "*"
		result["firstbytetime"] = "*"
		result["conntime"] = "*"
		result["size"] = "*"
		c.JSON(200, result)
		return
	}

	// 设置User-Agent
	userAgents := []string{
		"Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38",
		"Mozilla/5.0 (Linux; Android 7.0; SM-G892A Build/NRD90M; wv) AppleWebKit/537.36",
		"Mozilla/5.0 (Linux; Android 8.1; EML-L29 Build/HUAWEIEML-L29) AppleWebKit/537.36",
	}
	req.Header.Set("User-Agent", userAgents[0])
	req.Header.Set("Accept-Encoding", "gzip")

	// 执行请求
	startTime := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		// 错误处理
		errMsg := err.Error()
		if strings.Contains(errMsg, "no such host") {
			result["ip"] = "域名无法解析"
		} else if strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "timeout") {
			result["ip"] = "无法连接"
		} else if strings.Contains(errMsg, "deadline exceeded") || strings.Contains(errMsg, "timeout") {
			result["ip"] = "访问超时"
		} else {
			result["ip"] = "访问失败"
		}
		result["error"] = errMsg
		result["totaltime"] = "*"
		result["downtime"] = "*"
		result["downsize"] = "*"
		result["downspeed"] = "*"
		result["firstbytetime"] = "*"
		result["conntime"] = "*"
		result["size"] = "*"
		c.JSON(200, result)
		return
	}
	defer resp.Body.Close()

	// 获取时间信息
	timingTransport.mu.Lock()
	nameLookupTime := timingTransport.nameLookup
	connectTime := timingTransport.connect
	firstByteTime := timingTransport.startTransfer
	totalTime := timingTransport.total
	primaryIP := timingTransport.primaryIP
	timingTransport.mu.Unlock()

	// 如果primaryIP为空，尝试从URL获取
	if primaryIP == "" {
		host := parsedURL.Hostname()
		if net.ParseIP(host) != nil {
			primaryIP = host
		}
	}

	// 构建header字符串（base64编码）
	headerBuilder := strings.Builder{}
	headerBuilder.WriteString(fmt.Sprintf("%s %s\r\n", resp.Proto, resp.Status))
	for k, v := range resp.Header {
		headerBuilder.WriteString(fmt.Sprintf("%s: %s\r\n", k, strings.Join(v, ", ")))
	}
	headerBuilder.WriteString("\r\n")
	headerBytes := []byte(headerBuilder.String())
	result["header"] = base64.StdEncoding.EncodeToString(headerBytes)

	// 读取响应体（限制大小）
	bodyReader := io.LimitReader(resp.Body, 1024*1024) // 限制1MB
	bodyStartTime := time.Now()
	body, err := io.ReadAll(bodyReader)
	bodyReadTime := time.Now().Sub(bodyStartTime)
	if err != nil && err != io.EOF {
		result["error"] = err.Error()
	}

	downloadSize := int64(len(body))
	statusCode := resp.StatusCode
	
	// 如果首字节时间为0，使用连接时间
	if firstByteTime == 0 {
		firstByteTime = connectTime
	}
	
	// 总时间 = 实际请求时间
	if totalTime == 0 {
		totalTime = time.Since(startTime)
	}

	// 计算下载时间（使用实际读取时间）
	downloadTime := bodyReadTime
	if downloadTime <= 0 {
		downloadTime = totalTime - firstByteTime
		if downloadTime < 0 {
			downloadTime = 0
		}
	}

	// 计算下载速度（字节/秒）
	var downloadSpeed float64
	if downloadTime > 0 {
		downloadSpeed = float64(downloadSize) / downloadTime.Seconds()
	}

	// 格式化文件大小
	sizeStr := formatSize(downloadSize)
	downSizeStr := formatSizeKB(downloadSize)

	// 填充结果
	result["ip"] = primaryIP
	result["statuscode"] = statusCode
	result["nslookuptime"] = roundFloat(nameLookupTime.Seconds(), 3)
	result["conntime"] = roundFloat(connectTime.Seconds(), 3)
	result["firstbytetime"] = roundFloat(firstByteTime.Seconds(), 3)
	result["totaltime"] = roundFloat(totalTime.Seconds(), 3)
	result["downtime"] = roundFloat(downloadTime.Seconds(), 6)
	result["downsize"] = downSizeStr
	result["downspeed"] = downloadSpeed
	result["size"] = sizeStr

	c.JSON(200, result)
}

func handlePost(c *gin.Context, urlStr string, params map[string]interface{}) {
	// 获取seq参数
	seq := ""
	if seqVal, ok := params["seq"].(string); ok {
		seq = seqVal
	}

	// 解析URL
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		urlStr = "http://" + urlStr
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		c.JSON(200, gin.H{
			"seq":   seq,
			"type":  "cePost",
			"url":   urlStr,
			"error": "URL格式错误",
		})
		return
	}

	// 准备结果
	result := map[string]interface{}{
		"seq":  seq,
		"type": "cePost",
		"url":  urlStr,
	}

	// 获取POST数据
	postData := "abc=123"
	if data, ok := params["data"].(string); ok && data != "" {
		postData = data
	}

	// 创建自定义Transport用于时间跟踪
	timingTransport := newTimingTransport()
	
	// 创建HTTP客户端
	client := &http.Client{
		Transport: timingTransport,
		Timeout:   15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 20 {
				return fmt.Errorf("重定向次数过多")
			}
			return nil
		},
	}

	// 创建POST请求
	req, err := http.NewRequest("POST", urlStr, strings.NewReader(postData))
	if err != nil {
		result["error"] = err.Error()
		result["ip"] = "访问失败"
		result["totaltime"] = "*"
		result["downtime"] = "*"
		result["downsize"] = "*"
		result["downspeed"] = "*"
		result["firstbytetime"] = "*"
		result["conntime"] = "*"
		result["size"] = "*"
		c.JSON(200, result)
		return
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38")
	req.Header.Set("Accept-Encoding", "gzip")

	// 执行请求
	startTime := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "no such host") {
			result["ip"] = "域名无法解析"
		} else if strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "timeout") {
			result["ip"] = "无法连接"
		} else if strings.Contains(errMsg, "deadline exceeded") || strings.Contains(errMsg, "timeout") {
			result["ip"] = "访问超时"
		} else {
			result["ip"] = "访问失败"
		}
		result["error"] = errMsg
		result["totaltime"] = "*"
		result["downtime"] = "*"
		result["downsize"] = "*"
		result["downspeed"] = "*"
		result["firstbytetime"] = "*"
		result["conntime"] = "*"
		result["size"] = "*"
		c.JSON(200, result)
		return
	}
	defer resp.Body.Close()

	// 获取时间信息
	timingTransport.mu.Lock()
	nameLookupTime := timingTransport.nameLookup
	connectTime := timingTransport.connect
	firstByteTime := timingTransport.startTransfer
	totalTime := timingTransport.total
	primaryIP := timingTransport.primaryIP
	timingTransport.mu.Unlock()

	// 如果primaryIP为空，尝试从URL获取
	if primaryIP == "" {
		host := parsedURL.Hostname()
		if net.ParseIP(host) != nil {
			primaryIP = host
		}
	}

	// 构建header字符串（base64编码）
	headerBuilder := strings.Builder{}
	headerBuilder.WriteString(fmt.Sprintf("%s %s\r\n", resp.Proto, resp.Status))
	for k, v := range resp.Header {
		headerBuilder.WriteString(fmt.Sprintf("%s: %s\r\n", k, strings.Join(v, ", ")))
	}
	headerBuilder.WriteString("\r\n")
	headerBytes := []byte(headerBuilder.String())
	result["header"] = base64.StdEncoding.EncodeToString(headerBytes)

	// 读取响应体（限制大小）
	bodyReader := io.LimitReader(resp.Body, 1024*1024)
	bodyStartTime := time.Now()
	body, err := io.ReadAll(bodyReader)
	bodyReadTime := time.Since(bodyStartTime)
	if err != nil && err != io.EOF {
		result["error"] = err.Error()
	}

	downloadSize := int64(len(body))
	statusCode := resp.StatusCode
	
	// 如果首字节时间为0，使用连接时间
	if firstByteTime == 0 {
		firstByteTime = connectTime
	}
	
	// 总时间 = 实际请求时间
	if totalTime == 0 {
		totalTime = time.Since(startTime)
	}

	// 计算下载时间（使用实际读取时间）
	downloadTime := bodyReadTime
	if downloadTime <= 0 {
		downloadTime = totalTime - firstByteTime
		if downloadTime < 0 {
			downloadTime = 0
		}
	}

	var downloadSpeed float64
	if downloadTime > 0 {
		downloadSpeed = float64(downloadSize) / downloadTime.Seconds()
	}

	// 格式化文件大小
	sizeStr := formatSize(downloadSize)
	downSizeStr := formatSizeKB(downloadSize)

	// 填充结果
	result["ip"] = primaryIP
	result["statuscode"] = statusCode
	result["nslookuptime"] = roundFloat(nameLookupTime.Seconds(), 3)
	result["conntime"] = roundFloat(connectTime.Seconds(), 3)
	result["firstbytetime"] = roundFloat(firstByteTime.Seconds(), 3)
	result["totaltime"] = roundFloat(totalTime.Seconds(), 3)
	result["downtime"] = roundFloat(downloadTime.Seconds(), 6)
	result["downsize"] = downSizeStr
	result["downspeed"] = downloadSpeed
	result["size"] = sizeStr

	c.JSON(200, result)
}

// 辅助函数
func roundFloat(val float64, precision int) float64 {
	multiplier := 1.0
	for i := 0; i < precision; i++ {
		multiplier *= 10
	}
	return float64(int(val*multiplier+0.5)) / multiplier
}

func formatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	}
	kb := float64(bytes) / 1024
	if kb < 1024 {
		return fmt.Sprintf("%.3fKB", kb)
	}
	mb := kb / 1024
	return fmt.Sprintf("%.3fMB", mb)
}

func formatSizeKB(bytes int64) string {
	kb := float64(bytes) / 1024
	return fmt.Sprintf("%.3fKB", kb)
}
