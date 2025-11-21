package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleTest 统一测试接口
func HandleTest(c *gin.Context) {
	var req struct {
		Type  string                 `json:"type" binding:"required"`
		URL   string                 `json:"url" binding:"required"`
		Params map[string]interface{} `json:"params"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 根据类型分发到不同的处理器
	switch req.Type {
	case "ceGet":
		handleGet(c, req.URL, req.Params)
	case "cePost":
		handlePost(c, req.URL, req.Params)
	case "cePing":
		handlePing(c, req.URL, req.Params)
	case "ceDns":
		handleDns(c, req.URL, req.Params)
	case "ceTrace":
		handleTrace(c, req.URL, req.Params)
	case "ceSocket":
		handleSocket(c, req.URL, req.Params)
	case "ceTCPing":
		handleTCPing(c, req.URL, req.Params)
	case "ceFindPing":
		handleFindPing(c, req.URL, req.Params)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的测试类型"})
	}
}

// HandleHealth 健康检查
func HandleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

