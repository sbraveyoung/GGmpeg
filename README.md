```
  ____  ____                            
 / ___|/ ___|_ __ ___  _ __   ___  __ _ 
| |  _| |  _| '_ ` _ \| '_ \ / _ \/ _` |
| |_| | |_| | | | | | | |_) |  __/ (_| |
 \____|\____|_| |_| |_| .__/ \___|\__, |
                      |_|         |___/ 
```

[GGmpeg](https://github.com/SmartBrave/GGmpeg) is a **LIBRARY** that pays tribute to [FFmpeg](https://ffmpeg.org/) with [Go](https://golang.org/)!

**NOTE: GGmpeg is a wheel I made to implement various protocols of multimedia, there are still many problems to be solved. Please do NOT use it in production environments.**

## Feature
- [x] publish a stream with RTMP
- [x] play a stream with RTMP
- [x] play a stream with HTTP-flv
- [ ] play a stream with HLS
- [ ] RTMP client library
- [ ] publish and play a stream with RTP
- [ ] more...

## Usage
To start a RTMP server, you only need to write one line of code, see `./demo/rtmp_server.go`:
```go
 err := rtmp.NewServer(":1935", "live").Handler()
 //...
```

Then you can publish a stream to addr: `rtmp://localhost:1935/live/${liveid}` with any RTMP publish tools such as [OBS](https://obsproject.com/), [FFmpeg](https://ffmpeg.org/) and so on, and play it from addr: `rtmp://localhost:1935/live/${liveid}` with any RTMP play tools you wanted.

To support HTTP-flv, you need to invoke `WithHTTPFlv()` methods:
```go
 err := rtmp.NewServer(":1935", "live").WithHTTPFlv(":8080").Handler()
 //...
```
