package librtmp

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/SmartBrave/utils_sb/easyerrors"
	"github.com/SmartBrave/utils_sb/easyio"
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
	s.hlsAddress = address
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

	var err1, err2 error
	tcpAddr, err1 := net.ResolveTCPAddr("tcp", s.rtmpAddress)
	rtmpListener, err2 := net.ListenTCP("tcp", tcpAddr)
	if err := easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2); err != nil {
		return err
	}

	for {
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
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
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
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "video/x-flv")
			room.FLVJoin(easyio.NewEasyWriter(w))
		}
	})
	wg.Done()
	return http.ListenAndServe(s.flvAddress, nil)
}

func (s *server) handleHls(wg *sync.WaitGroup) error {
	defer wg.Done()
	//TODO
	return nil
}
