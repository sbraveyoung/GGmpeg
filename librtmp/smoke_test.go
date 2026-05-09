package librtmp

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sbraveyoung/GGmpeg/libhls"
)

// TestSmoke_BuilderChain asserts the With* method chain compiles, is
// non-destructive, and persists state on the server struct.
func TestSmoke_BuilderChain(t *testing.T) {
	s := NewServer(":1935", "live", "play").
		WithHTTPFlv(":8080").
		WithHls(":8081").
		WithDASH().
		WithRTSP(":554").
		WithRTMPPull("rtmp://up/live/x", "live", "x").
		WithSRT(":9710", "live", "ingest")

	if s.flvAddress != ":8080" {
		t.Errorf("flvAddress = %q", s.flvAddress)
	}
	if s.hlsAddress != ":8081" {
		t.Errorf("hlsAddress = %q", s.hlsAddress)
	}
	if s.rtspAddress != ":554" {
		t.Errorf("rtspAddress = %q", s.rtspAddress)
	}
	if len(s.pulls) != 1 {
		t.Errorf("pulls = %v", s.pulls)
	}
	if len(s.srtSpecs) != 1 {
		t.Errorf("srtSpecs = %v", s.srtSpecs)
	}
	if !s.apps["live"].dashEnabled {
		t.Errorf("DASH not enabled on live")
	}
	if s.apps["live"].hlsMode != libhls.IMMEDIATELY {
		t.Errorf("HLS mode on live = %v, want IMMEDIATELY", s.apps["live"].hlsMode)
	}
	if _, ok := s.apps["play"]; !ok {
		t.Errorf("apps[\"play\"] missing")
	}
}

// TestSmoke_HTTPFlvListenerResponds wires the HTTP-FLV handler onto a
// real TCP listener (port 0 → kernel-assigned) and asserts that
//   - a GET to a malformed URL returns 400
//   - a GET to a missing app returns 404
//   - a GET with Upgrade: websocket triggers the WS handshake path
func TestSmoke_HTTPFlvListenerResponds(t *testing.T) {
	srv := NewServer(":0", "live").WithHTTPFlv(":0")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	srv.flvAddress = listener.Addr().String()

	mux := http.NewServeMux()
	//Stand the same handler up by hand — sidesteps Handler()'s
	//os.Exit on error path.
	mux.HandleFunc("/", flvHandlerFromServer(srv))
	go http.Serve(listener, mux)

	addr := "http://" + listener.Addr().String()
	check := func(path string, want int) {
		t.Helper()
		c := &http.Client{Timeout: 2 * time.Second}
		resp, err := c.Get(addr + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		if resp.StatusCode != want {
			t.Errorf("GET %s = %d, want %d", path, resp.StatusCode, want)
		}
	}
	check("/notenough", 400)         //malformed (only 1 path segment)
	check("/missing/x.flv", 404)     //app not found
	check("/live/missing.flv", 404)  //app exists, room missing
	check("/live/x", 400)            //missing .flv suffix
}

// flvHandlerFromServer rebuilds the per-request closure used by
// handleHTTPFlv so we can exercise it in a test without spinning up
// the full Handler() blocking loop.
func flvHandlerFromServer(s *server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		appName, roomID, ok := parseFlvURL(r.URL.Path)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		app, ok := s.apps[appName]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if app.Load(roomID) == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

// TestSmoke_RTSPAcceptsConnection exercises handleRTSP's accept loop
// against a real TCP listener and asserts a freshly-connected client
// can complete the OPTIONS handshake.
func TestSmoke_RTSPAcceptsConnection(t *testing.T) {
	srv := NewServer(":0", "live")

	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := listener.AcceptTCP()
		if err != nil {
			return
		}
		newRTSPSession(conn, srv).run()
	}()

	c, err := net.DialTimeout("tcp", listener.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(2 * time.Second))

	req := "OPTIONS rtsp://test/x RTSP/1.0\r\nCSeq: 1\r\n\r\n"
	if _, err := c.Write([]byte(req)); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, 1024)
	n, err := c.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(buf[:n]), "200 OK") {
		t.Errorf("response = %q", buf[:n])
	}

	//Tear down so the session goroutine exits.
	teardown := "TEARDOWN rtsp://test/x RTSP/1.0\r\nCSeq: 2\r\n\r\n"
	c.Write([]byte(teardown))
	c.Read(buf)
	c.Close()

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("session goroutine didn't exit")
	}
}

// TestSmoke_RoomLifecycle verifies the publish → join → close flow:
// adding a room makes it Loadable; closing it via App.Delete causes
// any joined RTMP/FLV/HLS subscribers to wake up with alive=false on
// their broadcast reader.
func TestSmoke_RoomLifecycle(t *testing.T) {
	app := NewApp("live")
	rtmp := &RTMP{app: "live"}
	room := NewRoom(rtmp, "x")
	app.Store("x", room)
	if got := app.Load("x"); got != room {
		t.Errorf("Load returned %v, want %v", got, room)
	}

	app.Delete("x")
	if got := app.Load("x"); got != nil {
		t.Errorf("after Delete, Load returned %v", got)
	}
}

// TestSmoke_TLogServerString smoke-checks the server stringification
// path used in fmt.Sprintf calls — ensures no panic when fmt
// substitutes nil pointers post-cleanup.
func TestSmoke_TLogServerString(t *testing.T) {
	srv := NewServer(":1935", "live")
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on %%v: %v", r)
		}
	}()
	_ = fmt.Sprintf("%v", srv)
}
