// Package librtsp implements a minimal RTSP 1.0 (RFC 2326) server that
// re-streams the per-room broadcast as RTP packets carrying H.264 video
// and AAC audio. Scope:
//
//   - server-side PLAY only (no RECORD / ANNOUNCE)
//   - RTP-over-RTSP interleaved TCP transport ($ framing in RFC 2326
//     §10.12); no UDP, no NAT traversal
//   - H.264 over RTP per RFC 6184 (single-NAL + FU-A fragmentation)
//   - AAC over RTP per RFC 3640 (MPEG4-GENERIC, mode AAC-hbr)
//
// Played by VLC, ffplay, and gstreamer's rtspsrc with `protocols=tcp`.
package librtsp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// rtspVersion is the only protocol level we accept on inbound requests
// and emit on outbound responses.
const rtspVersion = "RTSP/1.0"

// Request is one parsed RTSP request — methods reuse Go's
// http.Header so callers can use Get / Set / Add idiomatically.
type Request struct {
	Method  string
	URL     string
	Version string
	Headers Headers
	Body    []byte
}

// Response is one outbound RTSP response.
type Response struct {
	Version    string
	StatusCode int
	Reason     string
	Headers    Headers
	Body       []byte
}

// Headers is a case-insensitive multi-value header map. RTSP shares
// HTTP/1.1 header semantics so we provide the same minimal API.
type Headers map[string]string

func (h Headers) Set(key, value string) { h[strings.ToLower(key)] = value }
func (h Headers) Get(key string) string { return h[strings.ToLower(key)] }
func (h Headers) Del(key string)        { delete(h, strings.ToLower(key)) }

// ReadRequest consumes one request from r. Body length is taken from
// the Content-Length header; absent length means no body.
func ReadRequest(r *bufio.Reader) (*Request, error) {
	startLine, err := readLine(r)
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(startLine, " ", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed request line: %q", startLine)
	}
	req := &Request{
		Method:  parts[0],
		URL:     parts[1],
		Version: parts[2],
		Headers: Headers{},
	}
	if err := readHeaders(r, req.Headers); err != nil {
		return nil, err
	}
	if cl := req.Headers.Get("content-length"); cl != "" {
		n, err := strconv.Atoi(cl)
		if err != nil {
			return nil, fmt.Errorf("bad Content-Length: %v", err)
		}
		body := make([]byte, n)
		if _, err := io.ReadFull(r, body); err != nil {
			return nil, err
		}
		req.Body = body
	}
	return req, nil
}

// Bytes serialises the response in wire format.
func (resp *Response) Bytes() []byte {
	if resp.Version == "" {
		resp.Version = rtspVersion
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s %d %s\r\n", resp.Version, resp.StatusCode, resp.Reason)
	if resp.Headers == nil {
		resp.Headers = Headers{}
	}
	if len(resp.Body) > 0 && resp.Headers.Get("content-length") == "" {
		resp.Headers.Set("Content-Length", strconv.Itoa(len(resp.Body)))
	}
	for k, v := range resp.Headers {
		//Title-case the header name on output for the wire (§12).
		fmt.Fprintf(&sb, "%s: %s\r\n", titleCase(k), v)
	}
	sb.WriteString("\r\n")
	out := []byte(sb.String())
	out = append(out, resp.Body...)
	return out
}

// readLine returns the next CRLF-terminated line without the CRLF.
// Tolerates bare LF (which some clients emit).
func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimRight(line, "\r\n")
	return line, nil
}

func readHeaders(r *bufio.Reader, h Headers) error {
	for {
		line, err := readLine(r)
		if err != nil {
			return err
		}
		if line == "" {
			return nil
		}
		idx := strings.Index(line, ":")
		if idx < 0 {
			return fmt.Errorf("malformed header: %q", line)
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		h.Set(key, val)
	}
}

// titleCase rewrites "content-length" → "Content-Length", matching
// what FFmpeg/VLC expect on the wire. Plain strings.Title is locale-
// sensitive (Go 1.18+ also deprecated it) so we do the dash-aware
// version inline.
func titleCase(s string) string {
	if s == "" {
		return s
	}
	parts := strings.Split(s, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		first := strings.ToUpper(p[:1])
		rest := strings.ToLower(p[1:])
		parts[i] = first + rest
	}
	return strings.Join(parts, "-")
}

// reasonPhrase maps RTSP status codes we use to their canonical
// reason-phrase strings (RFC 2326 §11). Unknown codes get "Unknown".
func reasonPhrase(code int) string {
	switch code {
	case 200:
		return "OK"
	case 400:
		return "Bad Request"
	case 404:
		return "Not Found"
	case 405:
		return "Method Not Allowed"
	case 454:
		return "Session Not Found"
	case 455:
		return "Method Not Valid In This State"
	case 461:
		return "Unsupported Transport"
	case 500:
		return "Internal Server Error"
	case 501:
		return "Not Implemented"
	default:
		return "Unknown"
	}
}

// errMalformedInterleave is returned when an inbound interleaved data
// frame ($-prefixed) is malformed. We don't actually parse interleaved
// data on the inbound path (clients don't send it for PLAY) but the
// session loop must skip past it cleanly if present.
var errMalformedInterleave = errors.New("malformed interleaved frame")
