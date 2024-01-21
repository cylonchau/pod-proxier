package server

import (
	"time"

	"github.com/gin-gonic/gin"
)

type V1 struct{}

func (this *V1) RegisterPortAPI(g *gin.RouterGroup) {
	portGroup := g.Group("/")
	portGroup.GET("/mapping", this.query)
	portGroup.POST("/mapping", this.create)
}

// create ...
// @Summary create
// @Produce  json
// @Success 200 {object} internal.Response
// @Router /fw/v3/setting/reload [POST]
func (this *V1) query(context *gin.Context) {
	// 1. 获取参数和参数校验
	var enconterError error
	proxyQuery := &ProxyQuery{}
	enconterError = context.Bind(proxyQuery)

	// 手动对请求参数进行详细的业务规则校验
	if enconterError != nil {
		APIResponse(context, enconterError, nil)
		return
	}

	b := ControllerInterface.QueryPodExists(proxyQuery.PodName)
	if b {
		SuccessResponse(context, OK, nil)
	} else {
		APIResponse(context, ErrNotFount, proxyQuery.PodName)
	}
}

func (this *V1) create(context *gin.Context) {
	var enconterError error
	proxyMappingQuery := &ProxyMapping{}
	enconterError = context.Bind(proxyMappingQuery)

	if enconterError != nil {
		APIResponse(context, enconterError, nil)
		return
	}

	if time.Duration(proxyMappingQuery.Time) > time.Hour*3 {
		proxyMappingQuery.Time = int(time.Duration(time.Hour * 3).Seconds())
	}

	if enconterError = ControllerInterface.CreateMapping(proxyMappingQuery.PodName, proxyMappingQuery.ServicePort); enconterError != nil {
		APIResponse(context, enconterError, nil)
		return
	}
	SuccessResponse(context, OK, nil)
}
