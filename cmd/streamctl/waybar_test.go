package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

const twoMonitors = `{"version":1,"mode":"normal","outputs":[` +
	`{"name":"DP-1","focused":false,"layout":"tile","nmaster":1,"tags":[1],"occupied":[1,2],"title":"vim","app_id":"foot"},` +
	`{"name":"eDP-1","focused":true,"layout":"monocle","nmaster":2,"tags":[3],"occupied":[3],"title":"<b>zen</b>","app_id":"zen"}],` +
	`"config_errors":[],"config_warnings":[]}`

func TestWaybarModules(t *testing.T) {
	tests := []struct {
		args   []string
		output string
		state  string
		want   string
	}{
		{[]string{"tag", "1"}, "DP-1", twoMonitors, `{"text":"1","class":["tag","focused"]}`},
		{[]string{"tag", "2"}, "DP-1", twoMonitors, `{"text":"2","class":["tag","occupied"]}`},
		{[]string{"tag", "3"}, "DP-1", twoMonitors, `{"text":"3","class":["tag","empty"]}`},
		{[]string{"tag", "3"}, "eDP-1", twoMonitors, `{"text":"3","class":["tag","focused"]}`},
		// No WAYBAR_OUTPUT_NAME, or an output that's gone: the focused one.
		{[]string{"tag", "3"}, "", twoMonitors, `{"text":"3","class":["tag","focused"]}`},
		{[]string{"tag", "3"}, "HDMI-A-1", twoMonitors, `{"text":"3","class":["tag","focused"]}`},
		{[]string{"layout"}, "DP-1", twoMonitors, `{"text":"tile"}`},
		{[]string{"layout"}, "eDP-1", twoMonitors, `{"text":"monocle 2"}`},
		{[]string{"title"}, "DP-1", twoMonitors, `{"text":"vim"}`},
		// waybar's "escape" option escapes the title; the JSON keeps it as is.
		{[]string{"title"}, "eDP-1", twoMonitors, `{"text":"\u003cb\u003ezen\u003c/b\u003e"}`},
		{[]string{"mode"}, "DP-1", twoMonitors, `{"text":""}`},
		{[]string{"mode"}, "DP-1", strings.Replace(twoMonitors, `"normal"`, `"passthrough"`, 1), `{"text":"passthrough"}`},
		{[]string{"config"}, "DP-1", twoMonitors, `{"text":""}`},
		{[]string{"config"}, "DP-1", strings.Replace(twoMonitors, `"config_errors":[]`, `"config_errors":["config:3: unknown setting \"gapp\""]`, 1),
			`{"text":"config error","class":["error"],"tooltip":"config:3: unknown setting \"gapp\""}`},
		{[]string{"config"}, "DP-1", strings.Replace(twoMonitors, `"config_warnings":[]`, `"config_warnings":["a","b"]`, 1),
			`{"text":"config warning","class":["warning"],"tooltip":"a\nb"}`},
	}
	for _, tt := range tests {
		r, err := waybarModule(tt.args)
		if err != nil {
			t.Fatalf("%v: %v", tt.args, err)
		}
		var out bytes.Buffer
		w := &waybarWriter{out: &out, render: r, output: tt.output}
		if _, err := w.Write([]byte(tt.state + "\n")); err != nil {
			t.Fatalf("%v: %v", tt.args, err)
		}
		if got := strings.TrimSuffix(out.String(), "\n"); got != tt.want {
			t.Errorf("%v on %q:\n got %s\nwant %s", tt.args, tt.output, got, tt.want)
		}
	}
}

// A state change that doesn't change the module writes nothing.
func TestWaybarSkipsUnchanged(t *testing.T) {
	r, _ := waybarModule([]string{"tag", "5"})
	var out bytes.Buffer
	w := &waybarWriter{out: &out, render: r, output: "DP-1"}
	for _, title := range []string{"a", "b", "c"} {
		if _, err := w.Write([]byte(strings.Replace(twoMonitors, `"vim"`, `"`+title+`"`, 1))); err != nil {
			t.Fatal(err)
		}
	}
	if n := strings.Count(out.String(), "\n"); n != 1 {
		t.Errorf("wrote %d lines for one unchanged tag:\n%s", n, out.String())
	}
}

func TestWaybarUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"tag"}, {"tag", "0"}, {"tag", "10"}, {"tag", "x"}, {"layout", "1"}, {"clock"}} {
		_, err := waybarModule(args)
		if ue := (usageError{}); !errors.As(err, &ue) {
			t.Errorf("waybarModule(%q) = %v, want a usage error", args, err)
		}
	}
}
