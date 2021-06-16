package main

import (
	"fmt"

	"github.com/SmartBrave/GGmpeg/rtmp"
)

func main() {
	err := rtmp.NewServer(":1935", "live").Handler()
	if err != nil {
		fmt.Println("handle server error:", err)
		return
	}
	return
}
