package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func handleGet(c *gin.Context, url string, params map[string]interface{}) {
	// TODO: 实现HTTP GET测试
	// 这里先返回一个简单的响应
	c.JSON(http.StatusOK, gin.H{
		"type":        "ceGet",
		"url":         url,
		"statuscode":  200,
		"totaltime":   time.Since(time.Now()).Milliseconds(),
		"response":    "OK",
	})
}

func handlePost(c *gin.Context, url string, params map[string]interface{}) {
	// TODO: 实现HTTP POST测试
	c.JSON(http.StatusOK, gin.H{
		"type":        "cePost",
		"url":         url,
		"statuscode":  200,
		"totaltime":   time.Since(time.Now()).Milliseconds(),
		"response":    "OK",
	})
}

