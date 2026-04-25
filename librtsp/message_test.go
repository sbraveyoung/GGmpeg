package librtsp

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestReadRequest_Basic(t *testing.T) {
	raw := "OPTIONS rtsp://example/live/x RTSP/1.0\r\n" +
		"CSeq: 1\r\n" +
		"User-Agent: test\r\n" +
		"\r\n"
	req, err := ReadRequest(bufio.NewReader(strings.NewReader(raw)))
	if err != nil {
		t.Fatalf("ReadRequest: %v", err)
	}
	if req.Method != "OPTIONS" {
		t.Errorf("Method = %q, want OPTIONS", req.Method)
	}
	if req.URL != "rtsp://example/live/x" {
		t.Errorf("URL = %q", req.URL)
	}
	if got := req.Headers.Get("CSeq"); got != "1" {
		t.Errorf("CSeq = %q, want 1", got)
	}
}

func TestReadRequest_WithBody(t *testing.T) {
	body := "v=0\r\no=- 0 0 IN IP4 127.0.0.1\r\n"
	raw := "ANNOUNCE rtsp://x/live/y RTSP/1.0\r\n" +
		"CSeq: 4\r\n" +
		"Content-Length: " + "" + "\r\n" +
		"\r\n" +
		body
	//Patch in the right Content-Length manually so the read knows
	//how many bytes to consume.
	raw = strings.Replace(raw,
		"Content-Length: \r\n",
		"Content-Length: "+itoa(len(body))+"\r\n", 1)

	req, err := ReadRequest(bufio.NewReader(strings.NewReader(raw)))
	if err != nil {
		t.Fatalf("ReadRequest: %v", err)
	}
	if string(req.Body) != body {
		t.Errorf("Body = %q, want %q", req.Body, body)
	}
}

func TestResponse_Bytes(t *testing.T) {
	resp := &Response{
		StatusCode: 200,
		Reason:     "OK",
		Headers: Headers{
			"cseq":    "3",
			"session": "abc;timeout=60",
		},
		Body: []byte("v=0\r\n"),
	}
	got := string(resp.Bytes())
	wants := []string{
		"RTSP/1.0 200 OK\r\n",
		"Cseq: 3\r\n",
		"Session: abc;timeout=60\r\n",
		"Content-Length: 5\r\n",
		"\r\nv=0\r\n",
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("response missing %q\n--- full ---\n%s", w, got)
		}
	}
}

// itoa is used by TestReadRequest_WithBody to keep the imports terse.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func TestTitleCase(t *testing.T) {
	cases := map[string]string{
		"content-length":     "Content-Length",
		"cseq":               "Cseq",
		"www-authenticate":   "Www-Authenticate",
		"a-b-c":              "A-B-C",
	}
	for in, want := range cases {
		if got := titleCase(in); got != want {
			t.Errorf("titleCase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSilenceUnused(t *testing.T) {
	//Make the linter happy if any helpers above end up unreferenced
	//after future edits.
	_ = bytes.NewReader(nil)
}
