package main

import (
	"fmt"
	"net"

	rtmp_pkg "github.com/SmartBrave/GGmpeg/rtmp"
)

func main() {
	// listener, err := net.Listen("tcp", "127.0.0.1:1935")
	listener, err := net.Listen("tcp", ":1935")
	if err != nil {
		fmt.Println("net.Listen error:", err)
		return
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("listener.Accept error:", err)
			continue
		}
		go rtmp_pkg.NewRTMP(conn).Handler()
	}
}
