package server

import (
	"github.com/gin-gonic/gin"
)

func ping(c *gin.Context) {
	SuccessResponse(c, OK, "pong")
}

func RegisteredRouter(e *gin.Engine) {
	e.Handle("GET", "health", ping)
	v1API := e.Group("/api")
	apiV1Group := v1API.Group("/v1")

	v1Paths := &V1{}
	v1Paths.RegisterPortAPI(apiV1Group)
}
