package librtmp

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/SmartBrave/GGmpeg/libhls"
	"github.com/SmartBrave/Athena/easyerrors"
	"github.com/SmartBrave/Athena/easyio"
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
	for app, _ := range s.apps {
		s.apps[app].hlsMode = libhls.IMMEDIATELY
	}
	s.hlsAddress = address
	return s
}

func (s *server) SetHlsMode(appName string, mode libhls.HLS_MODE) *server {
	if _, ok := s.apps[appName]; !ok {
		panic("appName does not exist.")
	}
	s.apps[appName].hlsMode = mode
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
		//err2 = conn.SetNoDelay(true)
		if err := easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2); err != nil {
			fmt.Println("error:", err)
			continue
		}

		peer := conn.RemoteAddr().String()
		go NewRTMP(conn, peer, s).HandlerServer()
	}
}

func (s *server) handleHTTPFlv(wg *sync.WaitGroup) error {
	flvListener, err := newTCPListener(s.flvAddress)
	if err != nil {
		fmt.Println("New flv listener error:", err)
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path //http://{domain}:{port}/{app}/{roomID}.flv
		slice := strings.Split(path, "/")
		if len(slice) != 3 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		appName := slice[1]
		roomID := strings.TrimRight(slice[2], ".flv")
		if app, ok := s.apps[appName]; !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		} else if room := app.Load(roomID); room == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		} else {
			w.Header().Set("Content-Type", "video/x-flv")
			//w.Header().Set("Access-Control-Allow-Methods", "GET,OPTIONS,HEAD")
			//w.Header().Set("Access-Control-Allow-Origin", "*")
			//w.Header().Set("Connection", "close")
			//w.Header().Set("Cache-Control", "no-cache")
			//w.Header().Set("Expires", "-1")
			//w.Header().Set("Pragma", "no-cache")
			//w.Header().Set("Date", time.Now().Format(""))
			room.FLVJoin(easyio.NewEasyWriter(w))
		}
	})
	wg.Done()
	return http.Serve(flvListener, mux)
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
		//http://{domain}:{port}/{app}/{roomID}/{tsName}.ts
		path := r.URL.Path
		slice := strings.Split(path, "/")
		if len(slice) != 4 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		appName := slice[1]
		roomID := slice[2]
		var room *Room
		if app, ok := s.apps[appName]; !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		} else if room = app.Load(roomID); room == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		switch slice[3] {
		case "index.m3u8":
			w.Header().Set("Content-Type", "application/x-mpegURL")
			w.Header().Set("Content-Length", "") //XXX
			room.HLSJoin(easyio.NewEasyWriter(w))
		default: //ts
			w.Header().Set("Content-Type", "video/mp2ts")
			w.Header().Set("Content-Length", "") //XXX
		}
	})
	wg.Done()
	return http.Serve(hlsListener, mux)
}

func newTCPListener(addr string) (listener *net.TCPListener, err error) {
	tcpAddr, err1 := net.ResolveTCPAddr("tcp", addr)
	listener, err2 := net.ListenTCP("tcp", tcpAddr)
	return listener, easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2)
}
