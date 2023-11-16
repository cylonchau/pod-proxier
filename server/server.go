package server

import (
	"flag"
	"fmt"
	"io/ioutil"

	"k8s.io/klog/v2"

	"github.com/gin-gonic/gin"

	"pod-proxier/controller"
)

var (
	webserver           *gin.Engine
	ControllerInterface *controller.Controller
	stopCh              = make(chan struct{})
	PodProxierConf      = &PodProxier{}
)

const (
	DefaultPort                = 3343
	DefaultDataPlanAPIAddr     = "http://127.0.0.1:5555"
	DefaultDataPlanAPIUser     = "admin"
	DefaultDataPlanAPIPassword = "1fc917c7ad66487470e466c0ad40ddd45b9f7730a4b43e1b2542627f0596bbdc"
	DefaultKubeconfig          = "~/.kube/config"
)

func init() {
	gin.DefaultWriter = ioutil.Discard
	gin.DisableConsoleColor()
}

type PodProxier struct {
	Port                int
	DataPlanAPIAddr     string
	DataPlanAPIUser     string
	DataPlanAPIPassword string
	Kubeconfig          string
}

func BuildInitFlags() {
	flagset := flag.CommandLine
	flagset.IntVar(&PodProxierConf.Port, "listen port", DefaultPort, "serve port")
	flagset.StringVar(&PodProxierConf.DataPlanAPIAddr, "apiAddr", DefaultDataPlanAPIAddr, "dataplanapi addr")
	flagset.StringVar(&PodProxierConf.DataPlanAPIUser, "apiUser", DefaultDataPlanAPIUser, "dataplanapi user")
	flagset.StringVar(&PodProxierConf.DataPlanAPIPassword, "apiPass", DefaultDataPlanAPIPassword, "dataplanapi password")
	flag.StringVar(&PodProxierConf.Kubeconfig, "kubeconfig", DefaultKubeconfig, "kubernetes auth config")

	klog.InitFlags(flagset)
	flag.Parse()
}

func NewHTTPSever() (err error) {
	webserver = gin.New()
	RegisteredRouter(webserver)
	klog.Info("Starting pod proixer.")
	ControllerInterface = controller.RunController(PodProxierConf.DataPlanAPIUser, PodProxierConf.DataPlanAPIPassword, PodProxierConf.DataPlanAPIAddr, PodProxierConf.Kubeconfig, stopCh)
	go ControllerInterface.Run()
	klog.Infof("Listening and serving HTTP on %s:%d", "0.0.0.0", PodProxierConf.Port)
	if err = webserver.Run(fmt.Sprintf("%s:%d", "0.0.0.0", PodProxierConf.Port)); err != nil {
		return err
	}
	<-stopCh
	return
}
