// go:build amd64 && windows
package main

import (
	"flag"

	"github.com/Veids/forwardlib/server/handler"
	"vsock_connector/vsock_server/channel"
)

func main() {
	port := flag.Uint("p", 9080, "listening port")
	cidr := flag.Uint("c", uint(channel.VMAddrCIDAny), "cidr")
	flag.Parse()

	c := channel.New(uint32(*port), uint32(*cidr))
	c.Init()
	handler.Loop(&c)
}
