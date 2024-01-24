package main

import (
	"flag"
	"os"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"

	"pod-proxier/server"
)

func main() {

	command := server.NewProxyCommand()
	flagset := flag.CommandLine
	//klog.InitFlags(flagset)
	klog.InitFlags(nil)
	pflag.CommandLine.AddGoFlagSet(flagset)
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
