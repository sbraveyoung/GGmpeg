```
  ____  ____
 / ___|/ ___|_ __ ___  _ __   ___  __ _
| |  _| |  _| '_ ` _ \| '_ \ / _ \/ _` |
| |_| | |_| | | | | | | |_) |  __/ (_| |
 \____|\____|_| |_| |_| .__/ \___|\__, |
                      |_|         |___/
```

**English** | [简体中文](./README.zh.md)

[GGmpeg](https://github.com/sbraveyoung/GGmpeg) is a multi-protocol media streaming **library** in pure Go that pays tribute to [FFmpeg](https://ffmpeg.org/). It implements the wire protocols from scratch (no `pion/`, `gortsplib`, etc.) so the codebase doubles as a reference implementation.

> **NOTE: GGmpeg is a learning / reference project. Many edge cases (encryption, congestion control, full codec support) are intentionally minimal. Please do NOT use it in production.**

---

## Features

### Ingest (publishers push to GGmpeg)

| Protocol | Status | Notes |
|---|---|---|
| RTMP | ✅ | Server-side; OBS / FFmpeg / FMLE compatible. SIMPLE + COMPLEX (digest) handshake |
| RTMP pull | ✅ | Outbound client: connect to upstream RTMP and inject as a local publish |
| RTSP `ANNOUNCE` + `RECORD` | ✅ | TCP-interleaved transport. UDP transport supported |
| SRT | ✅ | Live-mode listener with NAK-based ARQ. AES-CTR primitives present (KMREQ key derivation TODO) |

### Egress (clients pull from GGmpeg)

| Protocol | Status | Notes |
|---|---|---|
| RTMP play | ✅ | All standard NetStream commands |
| HTTP-FLV | ✅ | Sequence-header backfill for mid-GOP joiners, CORS, chunked flush |
| WebSocket-FLV | ✅ | Same URL as HTTP-FLV; `Upgrade: websocket` triggers WS framing — feeds `flv.js` |
| HLS | ✅ | TS segments + rolling-window playlist |
| LL-HLS | ✅ | Partial segments (BYTERANGE), `_HLS_msn` / `_HLS_part` blocking reload, EXT-X-PRELOAD-HINT |
| MPEG-DASH | ✅ | CMAF fMP4 segments + dynamic isoff-live `.mpd` |
| RTSP play | ✅ | TCP-interleaved + UDP transport |

### Codecs

| Codec | RTMP | HTTP-FLV | HLS | DASH | RTSP | SRT |
|---|---|---|---|---|---|---|
| **H.264** (AVC) | ✅ | ✅ | ✅ | ✅ | ✅ (RFC 6184 single-NAL + FU-A) | ✅ (TS demux) |
| **H.265** (HEVC) | ✅ | ✅ | ✅ (stream_type 0x24) | ✅ (hev1 + hvcC) | ✅ (RFC 7798 FU type 49) | partial |
| **AAC** (ADTS / hbr) | ✅ | ✅ | ✅ | ⚠️ (init segment is video-only) | ✅ (RFC 3640 mode AAC-hbr) | ✅ |
| **Opus** | ✅ | ✅ | — | — | ✅ (RFC 7587) | — |

✅ = supported | ⚠️ = partial | — = explicitly out of scope

---

## Quick start

### Run via Docker

```bash
docker build -t sbraveyoung/rtmp_server:latest .
docker run --rm --name rtmp_server \
  -p 1935:1935 -p 8080:8080 -p 8081:8081 \
  sbraveyoung/rtmp_server:latest
```

### Run from source

```bash
go build demo/rtmp_server.go
mkdir -p data            # HLS / DASH segment output
./rtmp_server
```

### Push a stream

```bash
ffmpeg -re -i input.mp4 -c copy -f flv rtmp://localhost:1935/live/x
# or use OBS → server "rtmp://localhost:1935/live", stream key "x"
```

### Play it back

| Protocol | URL |
|---|---|
| RTMP | `ffplay rtmp://localhost:1935/live/x` |
| HTTP-FLV | `ffplay http://localhost:8080/live/x.flv` |
| WebSocket-FLV | `flv.js` pointed at `ws://localhost:8080/live/x.flv` |
| HLS | `ffplay http://localhost:8081/live/x/index.m3u8` |
| DASH | `ffplay http://localhost:8081/live/x/index.mpd` |
| RTSP | `ffplay -rtsp_transport tcp rtsp://localhost:554/live/x` |

---

## Builder API

The demo wires every protocol via a method-chain on `librtmp.NewServer`:

```go
package main

import "github.com/sbraveyoung/GGmpeg/librtmp"

func main() {
    librtmp.NewServer(":1935", "live").
        WithHTTPFlv(":8080").     // HTTP-FLV + WebSocket-FLV on :8080
        WithHls(":8081").         // HLS playlist + .ts segments on :8081
        WithDASH().               // CMAF fMP4 + .mpd on the same :8081
        WithRTSP(":554").         // RTSP play (DESCRIBE/SETUP/PLAY) + publish (ANNOUNCE/RECORD)
        WithSRT(":9710", "live", "ingest").     // SRT publish endpoint
        WithRTMPPull(                            // pull from upstream RTMP into local stream
            "rtmp://upstream.example/live/foo",
            "live", "mirror").
        Handler()
}
```

| Method | Purpose |
|---|---|
| `NewServer(addr, apps...)` | RTMP listen address + app names |
| `WithHTTPFlv(addr)` | Open HTTP-FLV / WS-FLV listener |
| `WithHls(addr)` | Open HLS HTTP listener |
| `WithDASH()` | Reuse HLS port for DASH manifest + segments |
| `WithRTSP(addr)` | Open RTSP TCP listener |
| `WithSRT(addr, app, stream)` | Open SRT UDP listener; published TS goes to `apps[app]/streams[stream]` |
| `WithRTMPPull(url, app, stream)` | Pull from upstream RTMP and inject as a local publish |
| `SetHlsMode(app, mode)` | `IMMEDIATELY` (eager) or `DELAY` (start segmenter on first viewer) |
| `SetHlsDir(app, dir)` | Where HLS / DASH segments are written |

---

## Architecture

```
┌────────────────────────┐
│   Publishers (ingest)  │
│  RTMP / RTSP / SRT /   │
│  RTMP-pull             │
└──────────┬─────────────┘
           │  libflv tags (audio / video / meta)
           ▼
┌────────────────────────┐
│      Per-room GOP      │   Athena/broadcast — fan-out
│       Broadcast        │   ring with sequence-header cache
└──────────┬─────────────┘
           │  libflv tags
           ▼
┌────────────────────────┐
│  Subscribers (egress)  │
├────────────────────────┤
│ RTMP play │ HTTP-FLV   │
│ WS-FLV    │ HLS / LL   │
│ DASH      │ RTSP play  │
└────────────────────────┘
```

Every protocol consumes the same internal `libflv.Tag` stream — adding a new egress means writing one `Room.XxxJoin()` method that subscribes to the broadcast.

### Package map

| Package | Responsibility |
|---|---|
| `librtmp/` | RTMP server + RTMP pull client + RTSP server + SRT bridge + WebSocket-FLV (the integration hub for every wire protocol) |
| `librtsp/` | RTSP request/response, RTP packetisation (H.264 / HEVC / AAC / Opus), SDP, depacketisation |
| `libsrt/` | SRT 16-byte packet header, handshake (INDUCTION + CONCLUSION), ARQ (NAK + ACK), AES-CTR primitives, MPEG-TS demux |
| `libhls/` | HLS / LL-HLS segmenter + playlist generator (TS via `libmpeg`) |
| `libdash/` | CMAF / DASH segmenter + dynamic `.mpd` manifest |
| `libmp4/` | ISO BMFF (ftyp / moov / moof / mdat / avc1 / hev1 / avcC / hvcC) — used by `libdash` |
| `libmpeg/` | MPEG-TS muxer (PAT / PMT / PES) |
| `libflv/` | FLV tag model — the lingua franca between ingest and egress |
| `libamf/` | AMF0 codec for RTMP command / data messages |
| `libavc/` | H.264 SPS/PPS extraction + AVCC ↔ AnnexB conversion |
| `libaac/` | AAC AudioSpecificConfig + ADTS header |

External deps (all from [SmartBrave/Athena](https://github.com/SmartBrave/Athena)):
- `Athena/broadcast` — per-room GOP fan-out channel
- `Athena/easyio` — `EasyReader` / `EasyWriter` shims
- `Athena/easyerrors` — multi-error collector

---

## Development

### Build

```bash
go build ./...                  # whole library
go build demo/rtmp_server.go    # the demo binary the Dockerfile uses
```

### Tests

```bash
go test ./...                                # 116 tests, ~0.5 s
go test -v ./...                             # verbose listing
go test -coverprofile=cov.out ./...
go tool cover -func=cov.out | tail -1        # total coverage
go tool cover -html=cov.out                  # browser-friendly view
```

Test breakdown (116 PASS, 0 FAIL, 43.7 % statement coverage):

| Package | Tests | Coverage |
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

Includes unit, integration (segmenter end-to-end, RTMP handshake over `net.Pipe`), end-to-end (RTSP DESCRIBE/SETUP/PLAY/TEARDOWN, SRT INDUCTION→CONCLUSION→DATA over real UDP), and smoke tests (HTTP listener routing, builder chain).

---

## Spec references

PDFs of the specs each package implements live in `doc/`:

- RTMP 1.0 → `rtmp_specification_1.0.pdf`
- AMF0 → `amf0-file-format-specification.pdf`
- FLV v10 → `video_file_format_spec_v10.pdf` / `video_file_format_spec_v10_1.pdf`
- ISO/IEC 13818-1 (MPEG-TS) → `iso13818-1.pdf`
- ISO/IEC 14496-3 (AAC) → `ISO14496-3-2009.pdf`
- HLS (RFC 8216 + bis-09 draft) → `rfc8216.txt.pdf` / `draft-pantos-hls-rfc8216bis-09.pdf`
- ISO/IEC 14496-12 / 14496-15 (ISO BMFF + AVC-in-MP4) — referenced inline in `libmp4/`
- RTSP 1.0 (RFC 2326), RTP (RFC 3550), H.264 RTP (RFC 6184), HEVC RTP (RFC 7798), AAC RTP (RFC 3640), Opus RTP (RFC 7587), SRT (draft-sharabayko-srt-01) — referenced inline in `librtsp/` and `libsrt/`

---

## Known rough edges

- **HLS hardcoded segment dir** — `WithHls(addr)` uses `./data` by default; override via `SetHlsDir(app, dir)`.
- **DASH init segment is video-only** — audio AdaptationSet not yet wired.
- **SRT key management** (KMREQ + PBKDF2 + AES Key Wrap) is a TODO; passphrase-less publishers work.
- **WebRTC / WHIP / WHEP** — not implemented; would require ICE + DTLS + SRTP. Closest fit is `pion/webrtc` if you really need it.
- **RTMP `releaseStream` / `FCPublish`** echo `_result` but don't currently dedupe across reconnects.
- **No congestion control** anywhere — sender-side bandwidth estimation isn't a goal.

See `CLAUDE.md` for the in-repo design notes and contributor walkthrough.

---

## License

[See `LICENSE`.](./LICENSE)
