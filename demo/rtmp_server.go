package main

import (
	"fmt"

	"github.com/SmartBrave/GGmpeg/librtmp"
)

func main() {
	err := librtmp.NewServer(":1935", "live").WithHTTPFlv(":8080").WithHls(":8081").Handler()
	if err != nil {
		fmt.Println("handle server error:", err)
		return
	}
	return
}
