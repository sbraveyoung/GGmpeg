package librtmp

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/SmartBrave/Athena/easyerrors"
	"github.com/SmartBrave/Athena/easyio"
	"github.com/SmartBrave/GGmpeg/libamf"
)

// pullSpec is one pre-configured upstream stream to pull. The server
// dials it on Handler() startup and forwards every received tag into
// apps[App]/rooms[StreamID] as if it were a local publish.
type pullSpec struct {
	remoteURL string //rtmp://host[:port]/app/stream
	app       string //local app to inject under
	streamID  string //local stream id
}

// PullClient connects to an upstream RTMP server, performs the
// connect/createStream/play handshake, and writes incoming
// audio/video/data tags into a Room owned by the local server.
type PullClient struct {
	spec   pullSpec
	server *server
}

func newPullClient(srv *server, ps pullSpec) *PullClient {
	return &PullClient{spec: ps, server: srv}
}

// Run dials the upstream and drives the pull session synchronously.
// Returns when the upstream disconnects or a parse error fires; the
// caller is expected to retry with backoff on transient failures.
func (pc *PullClient) Run() error {
	u, err := url.Parse(pc.spec.remoteURL)
	if err != nil {
		return fmt.Errorf("parse remote URL: %w", err)
	}
	host := u.Host
	if !strings.Contains(host, ":") {
		host += ":1935"
	}
	conn, err := net.DialTimeout("tcp", host, 10*time.Second)
	if err != nil {
		return fmt.Errorf("dial %s: %w", host, err)
	}

	rtmp := NewRTMP(conn, host, pc.server)
	app, ok := pc.server.apps[pc.spec.app]
	if !ok {
		_ = conn.Close()
		return fmt.Errorf("local app %q not configured", pc.spec.app)
	}
	if existing := app.Load(pc.spec.streamID); existing != nil {
		_ = conn.Close()
		return fmt.Errorf("local stream %q already publishing", pc.spec.streamID)
	}
	rtmp.app = pc.spec.app
	rtmp.room = NewRoom(rtmp, pc.spec.streamID)
	app.Store(pc.spec.streamID, rtmp.room)
	//We act as publisher into the local broadcast — when the upstream
	//disconnects, cleanup() should tear the local room down so HLS /
	//FLV viewers exit cleanly.
	rtmp.role = rolePublisher

	rtmp.connectApp = parseAppName(u.Path)
	rtmp.tcURL = strings.TrimSuffix(pc.spec.remoteURL,
		"/"+rtmp.connectApp+"/"+parsePlayName(u.Path)) + "/" + rtmp.connectApp
	rtmp.playType = parsePlayName(u.Path)
	if rtmp.playType == "" {
		rtmp.playType = pc.spec.streamID
	}

	rtmp.HandlerClient()
	return nil
}

// runClientCommands drives connect → createStream → play on the
// outbound side. ParseMessage runs synchronously between sends so by
// the time we return, the upstream has accepted the play request.
func (rtmp *RTMP) runClientCommands() error {
	connectObj := map[string]interface{}{
		"app":            rtmp.connectApp,
		"flashVer":       "FMLE/3.0 (compatible; GGmpeg)",
		"tcUrl":          rtmp.tcURL,
		"fpad":           false,
		"capabilities":   15.0,
		"audioCodecs":    4071.0,
		"videoCodecs":    252.0,
		"videoFunction":  1.0,
		"objectEncoding": 0.0,
	}
	if err := rtmp.sendCommandRaw("connect", 1, connectObj, nil); err != nil {
		return fmt.Errorf("send connect: %w", err)
	}
	if err := rtmp.expectResult(15 * time.Second); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	if err := rtmp.sendCommandRaw("createStream", 2, nil, nil); err != nil {
		return fmt.Errorf("send createStream: %w", err)
	}
	if err := rtmp.expectResult(15 * time.Second); err != nil {
		return fmt.Errorf("createStream: %w", err)
	}

	//play emits no _result on most servers — only an onStatus update.
	//Don't block waiting for it; the main ParseMessage loop will
	//absorb whatever comes next and start receiving media.
	if err := rtmp.sendCommandRaw("play", 3, nil, []interface{}{rtmp.playType}); err != nil {
		return fmt.Errorf("send play: %w", err)
	}
	return nil
}

// sendCommandRaw assembles an AMF0 command message body and pushes it
// onto the wire on csidCommand. cmdObject (optional) becomes the third
// AMF element; extras append after that.
func (rtmp *RTMP) sendCommandRaw(name string, txnID int, cmdObject map[string]interface{}, extras []interface{}) error {
	buf := bytes.NewBuffer(nil)
	w := easyio.NewEasyWriter(buf)
	amf := libamf.AMF0
	var err1, err2, err3 error
	err1 = amf.Encode(w, name)
	err2 = amf.Encode(w, txnID)
	if cmdObject != nil {
		err3 = amf.Encode(w, cmdObject)
	} else {
		err3 = amf.Encode(w, nil)
	}
	if err := easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2, err3); err != nil {
		return err
	}
	for _, e := range extras {
		if e == nil {
			if err := amf.Encode(w, nil); err != nil {
				return err
			}
		} else {
			if err := amf.Encode(w, e); err != nil {
				return err
			}
		}
	}
	body, err := io.ReadAll(buf)
	if err != nil {
		return err
	}
	chunkSize := rtmp.ownMaxChunkSize
	for i := 0; ; i++ {
		l := i * chunkSize
		if l >= len(body) {
			break
		}
		r := l + chunkSize
		if r > len(body) {
			r = len(body)
		}
		fmtType := FMT0
		if i != 0 {
			fmtType = FMT3
		}
		if err := NewChunk(COMMAND_MESSAGE_AMF0, uint32(len(body)), 0,
			fmtType, csidCommand, body[l:r]).Send(rtmp); err != nil {
			return err
		}
	}
	return nil
}

// expectResult pumps ParseMessage until rtmp.resultCount advances —
// CommandMessage.Do bumps it on every _result/_error. Returns ErrTimeout
// if the deadline is reached without progress.
func (rtmp *RTMP) expectResult(timeout time.Duration) error {
	start := atomic.LoadUint32(&rtmp.resultCount)
	deadline := time.Now().Add(timeout)
	for {
		if err := ParseMessage(rtmp); err != nil {
			return err
		}
		if atomic.LoadUint32(&rtmp.resultCount) > start {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for _result")
		}
	}
}

// parseAppName plucks the app component out of an RTMP URL path:
//   /live/x → "live"
//   /foo/bar/baz → "foo"
func parseAppName(p string) string {
	p = strings.TrimPrefix(p, "/")
	if i := strings.Index(p, "/"); i >= 0 {
		return p[:i]
	}
	return p
}

// parsePlayName plucks the stream component (after the app):
//   /live/x → "x"
//   /live/x/y → "x/y"
func parsePlayName(p string) string {
	p = strings.TrimPrefix(p, "/")
	if i := strings.Index(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return ""
}
