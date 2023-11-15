package main

import "pod-proxier/server"

func main() {
	server.BuildInitFlags()
	server.NewHTTPSever()
}
