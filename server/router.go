package server

import (
	"github.com/gin-gonic/gin"
)

func ping(c *gin.Context) {
	SuccessResponse(c, OK, "pong")
}

func RegisteredRouter(e *gin.Engine) {
	e.Handle("GET", "ping", ping)
	e.Handle("GET", "proxier", create)
}

type ProxyQuery struct {
	PodName string `form:"pod_name" json:"pod_name,omitempty" binding:"required"`
}

// reload ...
// @Summary reload
// @Produce  json
// @Success 200 {object} internal.Response
// @Router /fw/v3/setting/reload [POST]
func create(context *gin.Context) {
	// 1. 获取参数和参数校验
	var enconterError error
	proxyQuery := &ProxyQuery{}
	enconterError = context.Bind(proxyQuery)

	// 手动对请求参数进行详细的业务规则校验
	if enconterError != nil {
		APIResponse(context, enconterError, nil)
		return
	}

	if enconterError = c.CreateTask(proxyQuery.PodName); enconterError != nil {
		APIResponse(context, enconterError, nil)
		return
	}
	SuccessResponse(context, OK, nil)
}
