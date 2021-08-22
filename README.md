```
  ____  ____                            
 / ___|/ ___|_ __ ___  _ __   ___  __ _ 
| |  _| |  _| '_ ` _ \| '_ \ / _ \/ _` |
| |_| | |_| | | | | | | |_) |  __/ (_| |
 \____|\____|_| |_| |_| .__/ \___|\__, |
                      |_|         |___/ 
```

[GGmpeg](https://github.com/SmartBrave/GGmpeg) is a **library** that pays tribute to [FFmpeg](https://ffmpeg.org/) with [Go](https://golang.org/)!

**NOTE: GGmpeg is a wheel I made to implement various protocols of multimedia, there are still many problems to be solved. Please do not use it in production environments.**

## Feature
- [x] publish and play a stream with RTMP

## TODO
- [ ] play a stream with HTTP-flv
- [ ] play a stream with HLS
- [ ] RTMP client library
- [ ] publish and play a stream with RTP
- [ ] Image Codec
  - [ ] JPEG
  - [ ] webp
  - [ ] HEIF
  - [ ] AVIF
- [ ] Video/Audio Codec
  - [ ] H.264/AVC
  - [ ] H.265/HEVC
  - [ ] H.266/VVC
  - [ ] vp8/vp9
  - [ ] AV1
- [ ] more...

## Usage
To start a RTMP server, you only need to write one line of code, see `./demo/rtmp_server.go`:
```go
 err := rtmp.NewServer(":1935", "live").Handler()
 if err != nil {
     fmt.Println("handle server error:", err)
     return
 }
```
