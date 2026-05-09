# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

GGmpeg is a Go library (module `github.com/sbraveyoung/GGmpeg`, Go 1.17) that implements multimedia streaming protocols from scratch — RTMP ingest, HTTP-FLV playback, and (WIP) HLS playback. It is not production-ready; treat it as a learning/reference implementation.

A running server is demonstrated in `demo/rtmp_server.go`, which is also the binary built by the Dockerfile:

```go
librtmp.NewServer(":1935", "live").WithHTTPFlv(":8080").WithHls(":8081").Handler()
```

## Commands

```bash
# Build the demo server (this is what the Dockerfile builds)
go build demo/rtmp_server.go

# Run all tests
go test ./...

# Run tests for one package
go test ./libmpeg/
go test ./libamf/

# Run a single test function
go test ./libmpeg/ -run TestPSI

# Docker build/run (see dockerfile header)
docker build -t sbraveyoung/rtmp_server:latest .
docker run --rm --name rtmp_server -p 1935:1935 -p 8080:8080 sbraveyoung/rtmp_server:latest
```

Tests live only in `libamf/` and `libmpeg/`. The RTMP/FLV/HLS glue layers are exercised end-to-end via the demo server (publish with OBS/ffmpeg to `rtmp://localhost:1935/live/<id>`, play via RTMP or `http://localhost:8080/live/<id>.flv`).

Note: `libhls/hls.go` currently writes segments to `./data/test.ts` (hardcoded), so the working directory must contain a `data/` folder when exercising HLS.

## Architecture

The packages form a layered pipeline. Data flows **publisher → librtmp → libflv tags → per-room GOP broadcast → subscribers (RTMP/FLV/HLS)**:

- `librtmp/` — RTMP server. `server.go` owns three listeners (RTMP, HTTP-FLV, HLS). `rtmp.go` wraps one TCP connection; `HandlerServer()` does handshake then loops `ParseMessage`.
- `librtmp/chunk.go` + `message.go` — RTMP chunk/message layer. A `Message` is reassembled from one or more `Chunk`s in `ParseMessage`; the `Message` interface's three verbs matter:
  - `Parse()` decode bytes received from peer
  - `Do()` apply side effects (update RTMP state, reply to peer) — called after a full message is received
  - `Send()` serialize a message we originate (e.g., GOP replay to a player)
  When adding a new message type, register it in the `switch chunk.MessageType` in `ParseMessage` and implement all three.
- `librtmp/command_message.go`, `control_message.go`, `data_message.go`, `audio.go`, `video.go` — concrete `Message` implementations (connect/createStream/publish/play, SetChunkSize, onMetaData, A/V payloads).
- `librtmp/app.go` + `room.go` — an `App` (e.g. "live") owns many `Room`s keyed by stream id. A `Room` is created by the publisher and holds a `*broadcast.Broadcast` GOP buffer (`Athena/broadcast`). Players join via `RTMPJoin` / `FLVJoin` / `HLSJoin`, each creating a `BroadcastReader` that replays the GOP and then streams new tags.
- `libflv/` — FLV tag model (`AudioTag`, `VideoTag`, `MetaTag` implement the `Tag` interface). This is the internal wire format the broadcast buffer carries — RTMP ingest parses into FLV tags, and every egress path (FLV, HLS) consumes FLV tags.
- `libamf/` — AMF0/AMF3 encode/decode used by RTMP command/data messages. `amf0.go` is the only real implementation.
- `libmpeg/` — MPEG-TS muxer. `ts.go`/`psi.go`/`pes.go` build PAT/PMT + PES packets; `crc32.go` is the MPEG-specific CRC. `NewTs(pid, cc, firstTS).Mux(...)` is the entry point. Well-tested.
- `libhls/` — RTMP→HLS transcoder glue. `HLS.Start(gopReader)` consumes FLV tags from a room's GOP broadcast and muxes them into TS via `libmpeg`. Handles AAC ADTS header insertion (`libaac`), H.264 AVCC→AnnexB conversion (`libavc`), audio/video timestamp alignment (`align.go`), and audio frame caching (`audio_cache.go`). `HLS_MODE` selects eager (`IMMEDIATELY`, default when `WithHls` is set) vs lazy (`DELAY`) transcoding.
- `libaac/`, `libavc/` — codec helpers (AAC ADTS header, AVC SPS/PPS → AnnexB) used by `libhls`.

### External dependencies worth knowing

- `github.com/SmartBrave/Athena/broadcast` — fan-out channel used as the per-room GOP buffer. `NewBroadcast(n)` keyframe window; `NewBroadcastReader(b).Read()` returns `(payload, alive)` where `alive=false` means publisher left.
- `github.com/SmartBrave/Athena/easyio` — thin `EasyReader`/`EasyWriter` wrappers used in place of raw `io.Reader`/`Writer` across the codebase. When wiring a new sink/source, wrap with `easyio.NewEasyReader/Writer`.
- `github.com/SmartBrave/Athena/easyerrors` — `HandleMultiError(easyerrors.Simple(), err1, err2, ...)` is the standard multi-error pattern; prefer it to ad-hoc `if err != nil` chains when collecting errors from several calls.

## Conventions

- Every exported "object" has a `NewXxx` constructor; callers don't struct-literal these types directly.
- Errors are generally logged with `fmt.Println` rather than wrapped/returned — matching the existing style. Reserve `errors.Wrap` (from `github.com/pkg/errors`) for parser paths in `libamf`/`libmpeg`.
- Comments and TODOs are frequently in Chinese (e.g. `libhls/hls.go` HLS_MODE notes); keep them intact when editing surrounding code.
- Many files contain commented-out code and `//XXX` / `//TODO` markers intentionally left as design notes — don't sweep them up as part of unrelated changes.
- Spec PDFs live in `doc/` (RTMP 1.0, FLV v10, ISO 13818-1, ISO 14496-3, HLS RFC 8216, AMF0). When implementing or fixing protocol logic, cross-reference the relevant PDF rather than guessing.

## Known rough edges (don't "fix" without intent)

- HLS output path `./data/test.ts` is hardcoded; `HLSJoin` in `room.go` only emits a stub m3u8; `handleHls` in `server.go` doesn't yet serve `.ts` bodies. HLS is the in-progress feature.
- `RTMP.HandlerClient()` is an empty stub — no RTMP client support yet.
- `SET_PEER_BANDWIDTH` and `ABORT_MESSAGE` cases in `ParseMessage` are intentionally empty.
