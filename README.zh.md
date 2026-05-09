```
  ____  ____
 / ___|/ ___|_ __ ___  _ __   ___  __ _
| |  _| |  _| '_ ` _ \| '_ \ / _ \/ _` |
| |_| | |_| | | | | | | |_) |  __/ (_| |
 \____|\____|_| |_| |_| .__/ \___|\__, |
                      |_|         |___/
```

[English](./README.md) | **简体中文**

[GGmpeg](https://github.com/sbraveyoung/GGmpeg) 是一个用纯 Go 实现的多协议流媒体**库**，向 [FFmpeg](https://ffmpeg.org/) 致敬。所有线协议都是从零实现（不依赖 `pion/`、`gortsplib` 等），代码本身可作为协议参考实现阅读。

> **注意：GGmpeg 是一个学习/参考性项目。许多边界场景（加密、拥塞控制、完整 codec 支持）刻意做得很简洁。请不要在生产环境使用。**

---

## 功能

### 入站（推流到 GGmpeg）

| 协议 | 状态 | 备注 |
|---|---|---|
| RTMP | ✅ | 服务端；兼容 OBS / FFmpeg / FMLE。SIMPLE + COMPLEX（digest）握手 |
| RTMP 拉流 | ✅ | 出站客户端：连接上游 RTMP 并把流注入到本地 publish |
| RTSP `ANNOUNCE` + `RECORD` | ✅ | TCP 交错传输。同时支持 UDP 传输 |
| SRT | ✅ | Live 模式监听器，含基于 NAK 的 ARQ。AES-CTR 原语已就位（KMREQ 密钥派生 TODO） |

### 出站（客户端从 GGmpeg 拉流）

| 协议 | 状态 | 备注 |
|---|---|---|
| RTMP play | ✅ | 标准 NetStream 命令全套 |
| HTTP-FLV | ✅ | 中途加入有序列头回填、CORS、chunked flush |
| WebSocket-FLV | ✅ | 与 HTTP-FLV 同 URL；带 `Upgrade: websocket` 即走 WS 帧——直接喂 `flv.js` |
| HLS | ✅ | TS 切片 + 滚动窗口播放列表 |
| LL-HLS | ✅ | Partial segment（BYTERANGE）、`_HLS_msn` / `_HLS_part` 阻塞 reload、EXT-X-PRELOAD-HINT |
| MPEG-DASH | ✅ | CMAF fMP4 切片 + 动态 isoff-live `.mpd` |
| RTSP play | ✅ | TCP 交错 + UDP 双传输 |

### 编解码器

| 编解码器 | RTMP | HTTP-FLV | HLS | DASH | RTSP | SRT |
|---|---|---|---|---|---|---|
| **H.264**（AVC） | ✅ | ✅ | ✅ | ✅ | ✅（RFC 6184 single-NAL + FU-A） | ✅（TS demux） |
| **H.265**（HEVC） | ✅ | ✅ | ✅（stream_type 0x24） | ✅（hev1 + hvcC） | ✅（RFC 7798 FU type 49） | 部分 |
| **AAC**（ADTS / hbr） | ✅ | ✅ | ✅ | ⚠️（init segment 仅视频） | ✅（RFC 3640 mode AAC-hbr） | ✅ |
| **Opus** | ✅ | ✅ | — | — | ✅（RFC 7587） | — |

✅ = 支持 | ⚠️ = 部分 | — = 明确不在范围内

---

## 快速开始

### 用 Docker 运行

```bash
docker build -t sbraveyoung/rtmp_server:latest .
docker run --rm --name rtmp_server \
  -p 1935:1935 -p 8080:8080 -p 8081:8081 \
  sbraveyoung/rtmp_server:latest
```

### 从源码运行

```bash
go build demo/rtmp_server.go
mkdir -p data            # HLS / DASH 切片输出目录
./rtmp_server
```

### 推流

```bash
ffmpeg -re -i input.mp4 -c copy -f flv rtmp://localhost:1935/live/x
# 或者用 OBS：服务器填 "rtmp://localhost:1935/live"，串流密钥填 "x"
```

### 播放

| 协议 | URL |
|---|---|
| RTMP | `ffplay rtmp://localhost:1935/live/x` |
| HTTP-FLV | `ffplay http://localhost:8080/live/x.flv` |
| WebSocket-FLV | 用 `flv.js` 拉 `ws://localhost:8080/live/x.flv` |
| HLS | `ffplay http://localhost:8081/live/x/index.m3u8` |
| DASH | `ffplay http://localhost:8081/live/x/index.mpd` |
| RTSP | `ffplay -rtsp_transport tcp rtsp://localhost:554/live/x` |

---

## Builder API

demo 通过链式调用 `librtmp.NewServer` 把每个协议串起来：

```go
package main

import "github.com/sbraveyoung/GGmpeg/librtmp"

func main() {
    librtmp.NewServer(":1935", "live").
        WithHTTPFlv(":8080").     // HTTP-FLV + WebSocket-FLV，端口 :8080
        WithHls(":8081").         // HLS playlist + .ts 切片，端口 :8081
        WithDASH().               // CMAF fMP4 + .mpd，复用 :8081
        WithRTSP(":554").         // RTSP play (DESCRIBE/SETUP/PLAY) + 推流 (ANNOUNCE/RECORD)
        WithSRT(":9710", "live", "ingest").     // SRT 推流入口
        WithRTMPPull(                            // 从上游 RTMP 拉流注入到本地
            "rtmp://upstream.example/live/foo",
            "live", "mirror").
        Handler()
}
```

| 方法 | 用途 |
|---|---|
| `NewServer(addr, apps...)` | RTMP 监听地址 + app 名列表 |
| `WithHTTPFlv(addr)` | 开启 HTTP-FLV / WS-FLV 监听 |
| `WithHls(addr)` | 开启 HLS HTTP 监听 |
| `WithDASH()` | 复用 HLS 端口出 DASH manifest + 切片 |
| `WithRTSP(addr)` | 开启 RTSP TCP 监听 |
| `WithSRT(addr, app, stream)` | 开启 SRT UDP 监听；推上来的 TS 注入到 `apps[app]/streams[stream]` |
| `WithRTMPPull(url, app, stream)` | 从上游 RTMP 拉流并注入到本地 publish |
| `SetHlsMode(app, mode)` | `IMMEDIATELY`（饿汉式）或 `DELAY`（懒汉式：第一个观众到才启动 segmenter） |
| `SetHlsDir(app, dir)` | HLS / DASH 切片写入目录 |

---

## 架构

```
┌────────────────────────┐
│   推流端（ingest）       │
│  RTMP / RTSP / SRT /   │
│  RTMP-pull             │
└──────────┬─────────────┘
           │  libflv tags（音频 / 视频 / meta）
           ▼
┌────────────────────────┐
│      每房间 GOP          │   Athena/broadcast — 扇出
│       Broadcast        │   ring，缓存序列头
└──────────┬─────────────┘
           │  libflv tags
           ▼
┌────────────────────────┐
│    拉流端（egress）       │
├────────────────────────┤
│ RTMP play │ HTTP-FLV   │
│ WS-FLV    │ HLS / LL   │
│ DASH      │ RTSP play  │
└────────────────────────┘
```

每个协议消费同一份 `libflv.Tag` 流——加新出口只需写一个 `Room.XxxJoin()` 方法订阅 broadcast。

### 包说明

| 包 | 职责 |
|---|---|
| `librtmp/` | RTMP 服务端 + RTMP 拉流客户端 + RTSP 服务端 + SRT 桥 + WebSocket-FLV（所有线协议的集成中枢） |
| `librtsp/` | RTSP 请求/响应、RTP 打包（H.264 / HEVC / AAC / Opus）、SDP、解包 |
| `libsrt/` | SRT 16字节包头、握手（INDUCTION + CONCLUSION）、ARQ（NAK + ACK）、AES-CTR 原语、MPEG-TS demux |
| `libhls/` | HLS / LL-HLS 切片器 + playlist 生成（TS 经 `libmpeg`） |
| `libdash/` | CMAF / DASH 切片器 + 动态 `.mpd` manifest |
| `libmp4/` | ISO BMFF（ftyp / moov / moof / mdat / avc1 / hev1 / avcC / hvcC）—— `libdash` 用 |
| `libmpeg/` | MPEG-TS muxer（PAT / PMT / PES） |
| `libflv/` | FLV tag 模型——ingest 与 egress 之间的通用语言 |
| `libamf/` | RTMP command / data 消息的 AMF0 编解码 |
| `libavc/` | H.264 SPS/PPS 抽取 + AVCC ↔ AnnexB 转换 |
| `libaac/` | AAC AudioSpecificConfig + ADTS 头 |

外部依赖（均来自 [SmartBrave/Athena](https://github.com/SmartBrave/Athena)）：
- `Athena/broadcast` —— 每房间 GOP 扇出 channel
- `Athena/easyio` —— `EasyReader` / `EasyWriter` 包装
- `Athena/easyerrors` —— 多 error 收集器

---

## 开发

### 构建

```bash
go build ./...                  # 整个库
go build demo/rtmp_server.go    # Dockerfile 构建的那个 demo 二进制
```

### 测试

```bash
go test ./...                                # 116 项测试，约 0.5 秒
go test -v ./...                             # 详细列表
go test -coverprofile=cov.out ./...
go tool cover -func=cov.out | tail -1        # 总覆盖率
go tool cover -html=cov.out                  # 浏览器查看分行高亮
```

测试明细（116 通过，0 失败，43.7% 语句覆盖率）：

| 包 | 测试数 | 覆盖率 |
|---|---:|---:|
| libaac | 4 | 100.0 % |
| libmp4 | 3 | 82.4 % |
| librtsp | 29 | 81.5 % |
| libavc | 4 | 75.9 % |
| libsrt | 23 | 75.1 % |
| libamf | 8 | 65.4 % |
| libhls | 8 | 50.5 % |
| libdash | 5 | 28.0 % |
| librtmp | 25 | 28.0 % |
| libmpeg | 5 | 25.9 % |
| libflv | 2 | 24.2 % |

涵盖单元测试、集成测试（segmenter 端到端、RTMP 握手走 `net.Pipe`）、端到端测试（RTSP DESCRIBE/SETUP/PLAY/TEARDOWN、SRT INDUCTION→CONCLUSION→DATA 走真实 UDP）、以及冒烟测试（HTTP listener 路由、builder 链）。

---

## 协议规范

`doc/` 下放着每个包对应实现的规范 PDF：

- RTMP 1.0 → `rtmp_specification_1.0.pdf`
- AMF0 → `amf0-file-format-specification.pdf`
- FLV v10 → `video_file_format_spec_v10.pdf` / `video_file_format_spec_v10_1.pdf`
- ISO/IEC 13818-1（MPEG-TS）→ `iso13818-1.pdf`
- ISO/IEC 14496-3（AAC）→ `ISO14496-3-2009.pdf`
- HLS（RFC 8216 + bis-09 草案）→ `rfc8216.txt.pdf` / `draft-pantos-hls-rfc8216bis-09.pdf`
- ISO/IEC 14496-12 / 14496-15（ISO BMFF + AVC-in-MP4）—— 在 `libmp4/` 内联引用
- RTSP 1.0（RFC 2326）、RTP（RFC 3550）、H.264 RTP（RFC 6184）、HEVC RTP（RFC 7798）、AAC RTP（RFC 3640）、Opus RTP（RFC 7587）、SRT（draft-sharabayko-srt-01）—— 在 `librtsp/` 和 `libsrt/` 内联引用

---

## 已知粗糙之处

- **HLS 默认切片目录硬编码** —— `WithHls(addr)` 默认用 `./data`；通过 `SetHlsDir(app, dir)` 覆盖。
- **DASH init segment 仅含视频** —— audio AdaptationSet 暂未接入。
- **SRT 密钥管理**（KMREQ + PBKDF2 + AES Key Wrap）是 TODO；无密码的发布者可正常工作。
- **WebRTC / WHIP / WHEP** —— 未实现；需要 ICE + DTLS + SRTP 整套，建议直接用 `pion/webrtc`。
- **RTMP `releaseStream` / `FCPublish`** 会回 `_result`，但目前重连场景下没去重。
- **没有拥塞控制** —— 发送端带宽估计不在目标范围内。

设计注释和贡献者指引见 `CLAUDE.md`。

---

## 许可证

[查看 `LICENSE`。](./LICENSE)
