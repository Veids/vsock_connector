package main

import (
	"flag"

	"github.com/Veids/forwardlib/client/handler"
	"github.com/mdlayher/vsock"
)

func main() {
	port := flag.Uint("p", 9080, "listening port")
	controlAddr := flag.String("c", "127.0.0.1:8337", "host:port")
	flag.Parse()

	l, err := vsock.Listen(uint32(*port), nil)
	if err != nil {
		panic(err)
	}

	c, err := l.Accept()
	if err != nil {
		panic(err)
	}

	handler.Loop(c, *controlAddr)
}
