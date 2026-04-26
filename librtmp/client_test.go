package librtmp

import "testing"

func TestParseAppName(t *testing.T) {
	cases := map[string]string{
		"/live/x":     "live",
		"live/x":      "live",
		"/foo/bar/baz": "foo",
		"":            "",
		"/onlyone":    "onlyone",
	}
	for in, want := range cases {
		if got := parseAppName(in); got != want {
			t.Errorf("parseAppName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParsePlayName(t *testing.T) {
	cases := map[string]string{
		"/live/x":      "x",
		"live/x":       "x",
		"/foo/bar/baz": "bar/baz",
		"/onlyone":     "",
		"":             "",
	}
	for in, want := range cases {
		if got := parsePlayName(in); got != want {
			t.Errorf("parsePlayName(%q) = %q, want %q", in, got, want)
		}
	}
}
