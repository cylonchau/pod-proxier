package server

import (
	"fmt"
	"net/http"

	"github.com/common-nighthawk/go-figure"
	"github.com/gin-gonic/gin"
)

func ping(c *gin.Context) {
	SuccessResponse(c, OK, "pong")
}

func versions(c *gin.Context) {
	myFigure := figure.NewFigure("Pod Proxier", "", true)
	c.String(http.StatusOK, fmt.Sprintf("%s", myFigure.String()))
}

func RegisteredRouter(e *gin.Engine) {
	e.Handle("GET", "health", ping)
	e.Handle("GET", "default", versions)
	v1API := e.Group("/api")
	apiV1Group := v1API.Group("/v1")

	v1Paths := &V1{}
	v1Paths.RegisterPortAPI(apiV1Group)
}
