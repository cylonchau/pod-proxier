package server

import (
	"fmt"
	"io/ioutil"

	"k8s.io/klog/v2"

	"github.com/gin-gonic/gin"

	"pod-proxier/controller"
)

var webserver *gin.Engine
var stopCh = make(chan struct{})
var c *controller.Controller

func init() {
	gin.DefaultWriter = ioutil.Discard
	gin.DisableConsoleColor()
}

func NewHTTPSever() (err error) {
	webserver = gin.New()
	RegisteredRouter(webserver)
	klog.V(2).Infof("Starting pod proixer.")
	c = controller.RunController()
	go c.Run(1, stopCh)
	klog.V(2).Infof("Listening and serving HTTP on %s:%d", "0.0.0.0", 3343)

	if err = webserver.Run(fmt.Sprintf("%s:%s", "0.0.0.0", "3343")); err != nil {
		return err
	}

	<-stopCh
	return
}
