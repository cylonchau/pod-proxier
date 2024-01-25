package server

import (
	"flag"
	"fmt"
	"io/ioutil"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"

	"pod-proxier/controller"
)

var (
	ControllerInterface *controller.Controller
)

const (
	DefaultPort                = 3343
	DefaultAddr                = "0.0.0.0"
	DefaultDataPlanAPIAddr     = "http://127.0.0.1:5555"
	DefaultDataPlanAPIUser     = "admin"
	DefaultDataPlanAPIPassword = "1fc917c7ad66487470e466c0ad40ddd45b9f7730a4b43e1b2542627f0596bbdc"
	DefaultKubeconfig          = "~/.kube/config"
	DefaultMappingPort         = 8849
	DefaultJprofilePortName    = "jprofiler_tcp"
	DefaultMaxMappingTime      = 10800
	DefaultResyncTime          = 10 * 60
)

func init() {
	gin.DefaultWriter = ioutil.Discard
	gin.DisableConsoleColor()
}

type PodProxier struct {
	Port                int
	Addr                string
	DataPlanAPIAddr     string
	DataPlanAPIUser     string
	DataPlanAPIPassword string
	Kubeconfig          string
	DefaultMappingPort  int
	JprofilerPortName   string
	MaxMappingTime      int
	ResyncTime          int
}

func NewOptions() *PodProxier {
	return &PodProxier{}
}

func NewProxyCommand() *cobra.Command {
	opts := NewOptions()

	cmd := &cobra.Command{
		Use:  "",
		Long: `The pod-proxier used for single pod proxy on kubernetes cluster.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			PrintFlags(cmd.Flags())
			if err := opts.Complete(); err != nil {
				return fmt.Errorf("failed complete: %w", err)
			}

			if err := opts.Run(); err != nil {
				return err
			}
			return nil
		},
		Args: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if len(arg) > 0 {
					return fmt.Errorf("%q does not take any arguments, got %q", cmd.CommandPath(), args)
				}
			}
			return nil
		},
	}

	fs := cmd.Flags()
	opts.AddFlags(fs)
	fs.AddGoFlagSet(flag.CommandLine) // for --boot-id-file and --machine-id-file
	_ = cmd.MarkFlagFilename("config", "yaml", "yml", "json")
	return cmd
}

func (o *PodProxier) AddFlags(fs *pflag.FlagSet) {
	fs.IntVar(&o.Port, "listen-port", DefaultPort, "serve port.")
	fs.StringVar(&o.Addr, "listen-addr", DefaultAddr, "listen address.")
	fs.StringVar(&o.DataPlanAPIAddr, "api-addr", DefaultDataPlanAPIAddr, "dataplanapi addr.")
	fs.StringVar(&o.DataPlanAPIUser, "api-user", DefaultDataPlanAPIUser, "dataplanapi user.")
	fs.StringVar(&o.DataPlanAPIPassword, "api-password", DefaultDataPlanAPIPassword, "dataplanapi password.")
	fs.StringVar(&o.Kubeconfig, "kubeconfig", DefaultKubeconfig, "kubernetes auth config.")
	fs.IntVar(&o.DefaultMappingPort, "default-map-port", DefaultMappingPort, "haproxy default mapping port.")
	fs.IntVar(&o.MaxMappingTime, "max-mapping-time", DefaultMaxMappingTime, "Max mapping time.")
	fs.IntVar(&o.ResyncTime, "resync-time", DefaultResyncTime, "Resync time from kubernetes cluster.")
	fs.StringVar(&o.JprofilerPortName, "jprofiler-port-name", DefaultJprofilePortName, "Find the name of 'jprofiler' in the pod spec.")
}

func PrintFlags(flags *pflag.FlagSet) {
	flags.VisitAll(func(flag *pflag.Flag) {
		klog.V(1).Infof("FLAG: --%s=%q", flag.Name, flag.Value)
	})
}

func (o *PodProxier) Complete() error {
	return nil
}

func (o *PodProxier) Run() (err error) {
	stopCh := make(chan struct{})
	webserver := gin.New()
	RegisteredRouter(webserver)
	klog.Info("Starting pod proixer.")
	ControllerInterface = controller.RunController(
		stopCh,
		o.Kubeconfig,
		o.Addr,
		o.Port,
		o.DataPlanAPIAddr,
		o.DataPlanAPIUser,
		o.DataPlanAPIPassword,
		o.DefaultMappingPort,
		o.JprofilerPortName,
		o.MaxMappingTime,
		o.ResyncTime,
	)
	go ControllerInterface.Run()
	klog.Infof("Listening and serving HTTP on %s:%d", o.Addr, o.Port)
	if err = webserver.Run(fmt.Sprintf("%s:%d", o.Addr, o.Port)); err != nil {
		return err
	}
	<-stopCh
	return
}
