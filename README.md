# GGmpeg
[GGmpeg](https://github.com/SmartBrave/GGmpeg) is a **library** that pays tribute to [FFmpeg](https://ffmpeg.org/) with [Go](https://golang.org/)!

**NOTE: GGmpeg is a wheel I made to implement various protocols of multimedia, there are still many problems to be solved. Please do not use it in production environments.**

## Feature
- [x] publish a stream with RTMP
- [x] play a stream with RTMP

## Usage
To start a RTMP server, you only need to write one line of code, see `./demo/rtmp_server.go`:
```go
 err := rtmp.NewServer(":1935", "live").Handler()
 if err != nil {
     fmt.Println("handle server error:", err)
     return
 }
```

## TODO
- [ ] play a stream with HTTP-flv
- [ ] play a stream with HLS
- [ ] RTMP client
- [ ] more...
