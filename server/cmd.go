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
	ControllerInterface        *controller.Controller
	ServiceControllerInterface *controller.ServiceController
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
	DefaultPortRangeStart      = 9000
	DefaultPortRangeEnd        = 9100
	DefaultAllowedNamespaces   = "default"
	DefaultPortName            = "debug"
	DefaultCheckTimeout        = int64(15)
	DefaultCheckInterval       = int64(2)
	DefaultMaxBatchSize        = 50
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

	// V2 功能参数
	EnableV1          bool
	EnableV2          bool
	HideBackend       bool
	PortRangeStart    int
	PortRangeEnd      int
	PortName          string
	AllowedNamespaces []string
	CheckTimeout      int64
	CheckInterval     int64
	MaxBatchSize      int
}

func NewOptions() *PodProxier {
	return &PodProxier{}
}

func NewProxyCommand() *cobra.Command {
	opts := NewOptions()

	cmd := &cobra.Command{
		Use:  "",
		Long: `The pod-proxier used for single pod proxy and service proxy on kubernetes cluster.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			PrintFlags(cmd.Flags())
			if err := opts.Complete(); err != nil {
				return fmt.Errorf("failed complete: %w", err)
			}

			if err := opts.Validate(); err != nil {
				return fmt.Errorf("validation failed: %w", err)
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
	fs.AddGoFlagSet(flag.CommandLine)
	_ = cmd.MarkFlagFilename("config", "yaml", "yml", "json")
	return cmd
}

func (o *PodProxier) AddFlags(fs *pflag.FlagSet) {
	// 原有 v1 参数
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

	// V2 功能参数
	fs.BoolVar(&o.EnableV1, "enable-v1", false, "Enable v1 pod proxy functionality.")
	fs.BoolVar(&o.EnableV2, "enable-v2", false, "Enable v2 service proxy functionality.")
	fs.StringVar(&o.PortName, "port-name", DefaultPortName, "Specifiy port name, only this port can be mapping.")
	fs.IntVar(&o.PortRangeStart, "port-range-start", DefaultPortRangeStart, "Start of port range for v2 service mapping.")
	fs.IntVar(&o.PortRangeEnd, "port-range-end", DefaultPortRangeEnd, "End of port range for v2 service mapping.")
	fs.BoolVar(&o.HideBackend, "hide-backend", true, "If specified, hide backend information in stats page.")

	// 处理命名空间列表
	fs.StringSliceVar(&o.AllowedNamespaces, "allowed-namespaces",
		[]string{DefaultAllowedNamespaces},
		"Comma-separated list of allowed namespaces for service proxy.")
	fs.Int64Var(&o.CheckTimeout, "check-timeout", DefaultCheckTimeout, "Backend health check timeout in seconds.")
	fs.Int64Var(&o.CheckInterval, "check-interval", DefaultCheckInterval, "Backend health check interval in seconds.")
	fs.IntVar(&o.MaxBatchSize, "max-batch-size", DefaultMaxBatchSize, "Max batch size for service updates.")
}

func PrintFlags(flags *pflag.FlagSet) {
	flags.VisitAll(func(flag *pflag.Flag) {
		klog.V(1).Infof("FLAG: --%s=%q", flag.Name, flag.Value)
	})
}

func (o *PodProxier) Complete() error {
	return nil
}

func (o *PodProxier) Validate() error {
	if !o.EnableV1 && !o.EnableV2 {
		return fmt.Errorf("at least one of --enable-v1 or --enable-v2 must be specify")
	}

	if o.EnableV2 {
		if o.PortRangeStart >= o.PortRangeEnd {
			return fmt.Errorf("port-range-start must be less than port-range-end")
		}

		if o.PortRangeStart < 1024 || o.PortRangeEnd > 65535 {
			return fmt.Errorf("port range must be between 1024 and 65535")
		}

		if len(o.AllowedNamespaces) == 0 {
			return fmt.Errorf("allowed-namespaces cannot be empty when v2 is enabled")
		}
	}

	return nil
}

func (o *PodProxier) Run() (err error) {
	stopCh := make(chan struct{})
	webserver := gin.New()
	RegisteredRouter(webserver)

	klog.Infof("Starting pod proxier with V1=%t, V2=%t", o.EnableV1, o.EnableV2)

	// 启动 V1 控制器 (Pod 代理)
	if o.EnableV1 {
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
		klog.Info("V1 Pod proxy controller started")
	}

	// 启动 V2 控制器 (Service 代理)
	if o.EnableV2 {
		ServiceControllerInterface = controller.RunServiceController(
			stopCh,
			o.Kubeconfig,
			o.Addr,
			o.Port,
			o.DataPlanAPIAddr,
			o.DataPlanAPIUser,
			o.DataPlanAPIPassword,
			o.HideBackend,
			o.PortRangeStart,
			o.PortRangeEnd,
			o.AllowedNamespaces,
			[]string{o.PortName},
			o.ResyncTime,
			o.CheckTimeout,
			o.CheckInterval,
			o.MaxBatchSize,
		)
		go ServiceControllerInterface.Run()
		klog.Info("V2 Service proxy controller started")
	}

	klog.Infof("Listening and serving HTTP on %s:%d", o.Addr, o.Port)
	if err = webserver.Run(fmt.Sprintf("%s:%d", o.Addr, o.Port)); err != nil {
		return err
	}
	<-stopCh
	return
}
