package librtmp

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SmartBrave/Athena/broadcast"
	"github.com/SmartBrave/Athena/easyerrors"
	"github.com/SmartBrave/Athena/easyio"
	"github.com/SmartBrave/GGmpeg/libdash"
	"github.com/SmartBrave/GGmpeg/libhls"
)

type server struct {
	rtmpAddress string          //default: ":1935"
	flvAddress  string          //default: ""
	hlsAddress  string          //default: ""
	apps        map[string]*App //appName, roomID, *room
}

func NewServer(address string, apps ...string) (s *server) {
	s = &server{
		rtmpAddress: address,
		apps:        make(map[string]*App, len(apps)),
	}
	for _, appName := range apps {
		s.apps[appName] = NewApp(appName)
	}
	return s
}

func (s *server) WithHTTPFlv(address string) *server {
	s.flvAddress = address
	return s
}

func (s *server) WithHls(address string) *server {
	for app := range s.apps {
		s.apps[app].hlsMode = libhls.IMMEDIATELY
	}
	s.hlsAddress = address
	return s
}

// WithDASH enables CMAF / MPEG-DASH segmenting on every app. Reuses the
// HLS HTTP listener — manifest path is /<app>/<stream>/index.mpd and
// segments live alongside HLS .ts files under <hlsDir>. Must be called
// after WithHls so an HTTP listener exists.
func (s *server) WithDASH() *server {
	for app := range s.apps {
		s.apps[app].dashEnabled = true
	}
	return s
}

func (s *server) SetHlsMode(appName string, mode libhls.HLS_MODE) *server {
	if _, ok := s.apps[appName]; !ok {
		panic("appName does not exist.")
	}
	s.apps[appName].hlsMode = mode
	return s
}

// SetHlsDir configures the directory where the given app stores HLS
// segments. If unset it defaults to "./data".
func (s *server) SetHlsDir(appName string, dir string) *server {
	if _, ok := s.apps[appName]; !ok {
		panic("appName does not exist.")
	}
	s.apps[appName].hlsDir = dir
	return s
}

func (s *server) Handler() error {
	wg := &sync.WaitGroup{}
	if s.flvAddress != "" {
		wg.Add(1)
		go func() {
			if err := s.handleHTTPFlv(wg); err != nil {
				fmt.Println("handleHTTPFlv error:", err)
				os.Exit(1)
			}
		}()
	}

	if s.hlsAddress != "" {
		wg.Add(1)
		go func() {
			if err := s.handleHls(wg); err != nil {
				fmt.Println("handleHls error:", err)
				os.Exit(1)
			}
		}()
	}
	wg.Wait()

	rtmpListener, err := newTCPListener(s.rtmpAddress)
	if err != nil {
		fmt.Println("New RTMP listener error:", err)
		return err
	}

	for {
		var err1, err2 error
		conn, err1 := rtmpListener.AcceptTCP()
		if err := easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2); err != nil {
			fmt.Println("error:", err)
			continue
		}

		peer := conn.RemoteAddr().String()
		go NewRTMP(conn, peer, s).HandlerServer()
	}
}

// parseFlvURL normalises /app/stream.flv? paths into (appName, roomID).
// Query strings are stripped and the ".flv" suffix must be present.
func parseFlvURL(rawPath string) (appName, roomID string, ok bool) {
	u, err := url.Parse(rawPath)
	if err != nil {
		return "", "", false
	}
	p := path.Clean(u.Path)
	parts := strings.Split(strings.TrimPrefix(p, "/"), "/")
	if len(parts) != 2 {
		return "", "", false
	}
	if !strings.HasSuffix(parts[1], ".flv") {
		return "", "", false
	}
	appName = parts[0]
	roomID = strings.TrimSuffix(parts[1], ".flv")
	if appName == "" || roomID == "" {
		return "", "", false
	}
	return appName, roomID, true
}

func (s *server) handleHTTPFlv(wg *sync.WaitGroup) error {
	flvListener, err := newTCPListener(s.flvAddress)
	if err != nil {
		fmt.Println("New flv listener error:", err)
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		//http://{domain}:{port}/{app}/{roomID}.flv[?query]
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
		room := app.Load(roomID)
		if room == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		//WebSocket-FLV: same URL, but client sends Upgrade: websocket.
		//Used by flv.js when the page is loaded over https or when the
		//user wants to multiplex the stream over an existing WS proxy.
		if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			ws, err := upgradeWebSocket(w, r)
			if err != nil {
				fmt.Println("websocket upgrade error:", err)
				return
			}
			defer ws.Close()
			stop := make(chan struct{})
			//Drain client-originated frames (ping/close) in the
			//background so the OS-level read buffer never stalls the
			//writer side.
			go ws.servePings(stop)
			done := make(chan struct{})
			go func() {
				room.FLVJoin(easyio.NewEasyWriter(&wsWriter{ws: ws}))
				close(done)
			}()
			select {
			case <-stop:
			case <-done:
			}
			return
		}

		w.Header().Set("Content-Type", "video/x-flv")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,OPTIONS,HEAD")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "-1")
		//http.ResponseWriter defaults to chunked when no Content-Length
		//is set and Flush() has been called. FLVJoin flushes after each
		//tag via the wrapped writer below.

		flusher, _ := w.(http.Flusher)
		room.FLVJoin(newFlushingWriter(w, flusher))
	})
	wg.Done()
	return http.Serve(flvListener, mux)
}

// flushingWriter wraps a http.ResponseWriter so every write triggers a
// Flusher.Flush(), giving FLV players byte-level incremental delivery.
type flushingWriter struct {
	w http.ResponseWriter
	f http.Flusher
}

func newFlushingWriter(w http.ResponseWriter, f http.Flusher) easyio.EasyWriter {
	return easyio.NewEasyWriter(&flushingWriter{w: w, f: f})
}

func (fw *flushingWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	if fw.f != nil {
		fw.f.Flush()
	}
	return n, err
}

// parseHlsURL splits /app/stream/file.(m3u8|ts) into its parts.
func parseHlsURL(rawPath string) (appName, roomID, file string, ok bool) {
	u, err := url.Parse(rawPath)
	if err != nil {
		return "", "", "", false
	}
	p := path.Clean(u.Path)
	parts := strings.Split(strings.TrimPrefix(p, "/"), "/")
	if len(parts) != 3 {
		return "", "", "", false
	}
	appName, roomID, file = parts[0], parts[1], parts[2]
	if appName == "" || roomID == "" || file == "" {
		return "", "", "", false
	}
	return appName, roomID, file, true
}

func (s *server) handleHls(wg *sync.WaitGroup) error {
	hlsListener, err := newTCPListener(s.hlsAddress)
	if err != nil {
		fmt.Println("New hls listener error:", err)
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		//http://{domain}:{port}/{app}/{roomID}/index.m3u8
		//http://{domain}:{port}/{app}/{roomID}/{seq}.ts
		appName, roomID, file, ok := parseHlsURL(r.URL.Path)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		app, ok := s.apps[appName]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		room := app.Load(roomID)
		if room == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		hls := app.LoadHLS(roomID)
		if hls == nil {
			//DELAY mode: lazy-start the transcoder on first playlist
			//hit so no-one is paying for HLS segmentation when no viewer
			//is attached.
			if app.hlsMode == libhls.DELAY && strings.HasSuffix(file, ".m3u8") {
				hls = libhls.NewHls().WithStreamID(roomID).WithDir(app.hlsDir)
				app.StoreHLS(roomID, hls)
				go hls.Start(broadcast.NewBroadcastReader(room.GOP))
				hls.WaitFirstSegment()
			} else {
				w.WriteHeader(http.StatusNotFound)
				return
			}
		}

		//DASH dispatch (manifest + init + media segments). Lazy-start
		//if WithDASH is on but no DASH instance exists yet.
		switch {
		case file == "index.mpd",
			strings.HasSuffix(file, "-init.mp4"),
			strings.HasSuffix(file, ".m4s"):
			dash := app.LoadDASH(roomID)
			if dash == nil && app.dashEnabled {
				dash = libdash.NewDASH().WithStreamID(roomID).WithDir(app.dashDir)
				app.StoreDASH(roomID, dash)
				go dash.Start(broadcast.NewBroadcastReader(room.GOP))
				dash.WaitFirstSegment()
			}
			if dash == nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			s.serveDASH(w, r, dash, file)
			return
		}

		switch {
		case strings.HasSuffix(file, ".m3u8"):
			//LL-HLS blocking playlist reload: clients append
			//_HLS_msn=<seq>&_HLS_part=<idx> to ask the server to delay
			//the response until that media-sequence/part has been
			//produced. Falls back to a normal (non-blocking) reply
			//when those params are absent or the segmenter doesn't
			//have LL enabled.
			q := r.URL.Query()
			var playlist []byte
			if msnStr := q.Get("_HLS_msn"); msnStr != "" {
				msn, _ := strconv.Atoi(msnStr)
				part, _ := strconv.Atoi(q.Get("_HLS_part"))
				//Apple recommends timeout ~= 3 * PART-TARGET; use 3 s
				//as a safe floor so even non-LL mode answers promptly.
				timeout := 3 * hls.PartTargetDur()
				if timeout < time.Second {
					timeout = time.Second
				}
				playlist = hls.WaitForPlaylist(msn, part, timeout)
			} else {
				playlist = hls.Playlist()
			}
			if len(playlist) == 0 {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Cache-Control", "no-cache")
			_, _ = w.Write(playlist)
		case strings.HasSuffix(file, ".ts"):
			//Resolve and re-clean the path under the segment dir to
			//defeat path traversal attempts.
			requested := filepath.Join(hls.Dir(), file)
			rel, err := filepath.Rel(hls.Dir(), requested)
			if err != nil || strings.HasPrefix(rel, "..") {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "video/mp2t")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Cache-Control", "max-age=3600")
			http.ServeFile(w, r, requested)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	wg.Done()
	return http.Serve(hlsListener, mux)
}

// serveDASH handles the three URL shapes a DASH player asks for:
//   index.mpd            → dynamic manifest
//   <stream>-init.mp4    → init segment (ftyp + moov)
//   <stream>-<seq>.m4s   → media segment (moof + mdat)
// Both file types are static under dash.Dir() so http.ServeFile is the
// right tool — and it handles Range requests for free, which is useful
// if/when we add CMAF byte-range chunked-transfer mode.
func (s *server) serveDASH(w http.ResponseWriter, r *http.Request, dash *libdash.DASH, file string) {
	switch {
	case file == "index.mpd":
		mpd := dash.Manifest()
		if len(mpd) == 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/dash+xml")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(mpd)
	case strings.HasSuffix(file, "-init.mp4"):
		full := filepath.Join(dash.Dir(), file)
		rel, err := filepath.Rel(dash.Dir(), full)
		if err != nil || strings.HasPrefix(rel, "..") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		http.ServeFile(w, r, full)
	case strings.HasSuffix(file, ".m4s"):
		full := filepath.Join(dash.Dir(), file)
		rel, err := filepath.Rel(dash.Dir(), full)
		if err != nil || strings.HasPrefix(rel, "..") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "video/iso.segment")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		http.ServeFile(w, r, full)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func newTCPListener(addr string) (listener *net.TCPListener, err error) {
	tcpAddr, err1 := net.ResolveTCPAddr("tcp", addr)
	listener, err2 := net.ListenTCP("tcp", tcpAddr)
	return listener, easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2)
}
