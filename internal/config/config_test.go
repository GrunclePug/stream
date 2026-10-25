package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/grunclepug/stream/internal/layout"
)

func TestColorRGBA32(t *testing.T) {
	tests := []struct {
		c          Color
		r, g, b, a uint32
	}{
		{0x000000ff, 0, 0, 0, 0xffffffff},
		{0xffffffff, 0xffffffff, 0xffffffff, 0xffffffff, 0xffffffff},
		{0x005577ff, 0, 0x55555555, 0x77777777, 0xffffffff},
		{0xff000080, 0x80808080, 0, 0, 0x80808080}, // premultiplied
		{0xffffff00, 0, 0, 0, 0},
	}
	for _, tt := range tests {
		r, g, b, a := tt.c.RGBA32()
		if r != tt.r || g != tt.g || b != tt.b || a != tt.a {
			t.Errorf("%#08x.RGBA32() = %#x %#x %#x %#x, want %#x %#x %#x %#x",
				uint32(tt.c), r, g, b, a, tt.r, tt.g, tt.b, tt.a)
		}
	}
}

func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestRoundTrip checks the promise gen-config makes: the generated file
// parses back to exactly the defaults.
func TestRoundTrip(t *testing.T) {
	path := write(t, t.TempDir(), "config", string(Generate(Default())))
	got, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse(Generate(Default())): %v", err)
	}
	if !reflect.DeepEqual(got, Default()) {
		t.Errorf("Parse(Generate(Default())) differs from Default()\ngot:\n%s\nwant:\n%s", Generate(got), Generate(Default()))
	}
}

// Flags survive a round trip, -also included.
func TestRoundTripFlags(t *testing.T) {
	cfg := Default()
	n := &cfg.Modes[0]
	n.Bindings = append(n.Bindings, Binding{Mods: Mod, Keysym: n.Bindings[0].Keysym, Also: true, Locked: true, Command: mustCommand("reload")})
	path := write(t, t.TempDir(), "config", string(Generate(cfg)))
	got, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, cfg) {
		t.Errorf("-also binding lost:\n%s", Generate(got))
	}
}

// TestDefaultModes checks the defaults pass the checks a config file gets.
func TestDefaultModes(t *testing.T) {
	cfg := Default()
	if cfg.Modes[0].Name != NormalMode {
		t.Fatalf("Modes[0] = %q, want %q", cfg.Modes[0].Name, NormalMode)
	}
	for _, m := range cfg.Modes {
		seen := map[string]bool{}
		for _, b := range m.Bindings {
			k := b.Keys()
			if seen[k] {
				t.Errorf("mode %s: %s bound twice", m.Name, b.Keys())
			}
			seen[k] = true
		}
	}
}

func TestLoadMissing(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "config"))
	if err != nil || !reflect.DeepEqual(cfg, Default()) {
		t.Errorf("Load(missing) = %v, want the defaults and no error", err)
	}
}

func TestParseSettings(t *testing.T) {
	path := write(t, t.TempDir(), "config", `
# a comment
border-width 2
border-focused #5fd4c4
border-unfocused #2a364080
layout monocle
nmaster 2
mfact 0.6
gap 8
mod super
repeat-delay 660
repeat-rate 30
`)
	got, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	want := Default()
	want.BorderWidth = 2
	want.BorderFocused = 0x5fd4c4ff
	want.BorderUnfocused = 0x2a364080
	want.Layout = layout.Monocle
	want.NMaster = 2
	want.MFact = 0.6
	want.Gap = 8
	want.Mod = Super
	want.RepeatDelay = 660
	want.RepeatRate = 30
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestParseBindings(t *testing.T) {
	path := write(t, t.TempDir(), "config", `
bind Mod+Shift+Return spawn foot -e sh -c 'echo # not a comment'
bind -repeat Mod+J focus next
bind -locked -repeat XF86AudioRaiseVolume spawn wpctl set-volume @DEFAULT_AUDIO_SINK@ 5%+
bind super+BTN_LEFT move
bind Super+r mode resize

mode resize {
	bind Escape mode normal
	bind -repeat h mfact -0.05
}
`)
	got, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Modes) != 2 || got.Modes[0].Name != NormalMode || got.Modes[1].Name != "resize" {
		t.Fatalf("modes = %+v", got.Modes)
	}
	var lines []string
	for _, m := range got.Modes {
		for _, b := range m.Bindings {
			lines = append(lines, m.Name+": "+b.Keys()+" "+b.Command.String()+flags(b))
		}
	}
	want := []string{
		"normal: Mod+Shift+Return spawn foot -e sh -c 'echo # not a comment'",
		"normal: Mod+j focus next -repeat",
		"normal: XF86AudioRaiseVolume spawn wpctl set-volume @DEFAULT_AUDIO_SINK@ 5%+ -repeat -locked",
		"normal: Super+BTN_LEFT move",
		"normal: Super+r mode resize",
		"resize: Escape mode normal",
		"resize: h mfact -0.05 -repeat",
	}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("bindings:\n%s\nwant:\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
}

func flags(b Binding) string {
	s := ""
	if b.Repeat {
		s += " -repeat"
	}
	if b.Locked {
		s += " -locked"
	}
	return s
}

func TestModResolve(t *testing.T) {
	if got := (Mod | Shift).Resolve(Super); got != Super|Shift {
		t.Errorf("(Mod|Shift).Resolve(Super) = %#x", got)
	}
	if got := (Alt | Shift).Resolve(Super); got != Alt|Shift {
		t.Errorf("(Alt|Shift).Resolve(Super) = %#x", got)
	}
}

func TestInclude(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "theme"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "theme/stream", "border-focused #5fd4c4\ninclude ../more\n")
	write(t, dir, "more", "border-width 3\n")
	path := write(t, dir, "config", "border-width 2\ninclude theme/stream\ngap 1\n")
	got, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.BorderFocused != 0x5fd4c4ff || got.BorderWidth != 3 || got.Gap != 1 {
		t.Errorf("got border-focused %#x, border-width %d, gap %d", got.BorderFocused, got.BorderWidth, got.Gap)
	}

	// -optional skips a missing file but reads one that is there.
	path = write(t, dir, "config", "include -optional nosuch\ninclude -optional theme/stream\n")
	if got, err := Parse(path); err != nil || got.BorderFocused != 0x5fd4c4ff {
		t.Errorf("-optional: %v, border-focused %#x", err, got.BorderFocused)
	}
	// Its errors still count.
	write(t, dir, "bad", "gap x\n")
	path = write(t, dir, "config", "include -optional bad\n")
	if _, err := Parse(path); err == nil || !strings.Contains(err.Error(), "bad:1: gap") {
		t.Errorf("-optional with an error: %v", err)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		config string
		want   string // in the error
	}{
		{"nosuch 1", "config:1: unknown setting"},
		{"\n\ngap", "config:3: gap: missing value"},
		{"gap -1", "gap:"},
		{"mfact 1.5", "mfact:"},
		{"layout grid", "layout:"},
		{"border-focused 5fd4c4", "colour"},
		{"mod mod", "mod:"},
		{"bind Mod+nosuchkey close", "unknown key"},
		{"bind Hyper+x close", "unknown modifier"},
		{"bind Mod+Mod+x close", "twice"},
		{"bind Mod+ close", "missing key"},
		{"bind -sticky Mod+x close", "unknown flag"},
		{"bind Mod+x", "missing command"},
		{"bind Mod+x frobnicate", "unknown command"},
		{"bind Mod+x move", "needs a pointer button"},
		{"bind -repeat Mod+BTN_LEFT move", "only works on keys"},
		{"bind Mod+x mode nosuch", "no such mode"},
		{"mode stuck {\n\tbind x close\n}", "config:1: mode stuck: no binding leads back"},
		{"mode a {\n\tbind x close\n", "missing }"},
		{"}", "without a mode block"},
		{"mode a {\n\tgap 1\n}", "only bind lines"},
		{"mode normal {\n}", "top-level bind lines"},
		{"mode a\n", "mode <name> {"},
		{"mode a {\nbind x mode normal\n}\nmode a {\n}", "already declared"},
		{"include nosuch", "include:"},
		{"include -optional", "missing path"},
		{"include -maybe x", "unknown flag"},
		{"include config", "include loop"},
	}
	for _, tt := range tests {
		path := write(t, t.TempDir(), "config", tt.config)
		_, err := Parse(path)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("Parse(%q) = %v, want an error containing %q", tt.config, err, tt.want)
		}
	}
}

// A key bound twice runs both commands, with a warning unless the later
// binding says -also.
func TestDuplicateKeys(t *testing.T) {
	tests := []struct {
		config string
		want   []string // warnings
	}{
		{"bind Mod+x close\nbind Mod+x zoom\n", []string{"config:2: bind Mod+x: the key also runs close (line 1)"}},
		{"bind Mod+x close\nbind -also Mod+x zoom\n", nil},
		{"mod super\nbind Mod+x close\nbind Super+x zoom\n", []string{"config:3: bind Super+x: the key also runs close (line 2)"}},
		{"bind Mod+x close\nbind Super+x zoom\n", nil}, // Mod is Alt
		{"bind Mod+x close\nbind Mod+x zoom\nbind -also Mod+x reload\n", []string{"config:2:"}},
		{"bind Mod+x close\nmode a {\n\tbind Mod+x mode normal\n}\nbind Mod+y mode a\n", nil}, // other modes
	}
	for _, tt := range tests {
		path := write(t, t.TempDir(), "config", tt.config)
		cfg, err := Parse(path)
		if err != nil {
			t.Errorf("Parse(%q): %v", tt.config, err)
			continue
		}
		if len(cfg.Warnings) != len(tt.want) {
			t.Errorf("Parse(%q) warnings %v, want %d", tt.config, cfg.Warnings, len(tt.want))
			continue
		}
		for i, w := range tt.want {
			if got := cfg.Warnings[i].Error(); !strings.Contains(got, w) {
				t.Errorf("Parse(%q) warning %q, want it to contain %q", tt.config, got, w)
			}
		}
		if n := len(cfg.Modes[0].Bindings); n < 2 {
			t.Errorf("Parse(%q) kept %d bindings, want both", tt.config, n)
		}
	}
}

func TestParseReportsEveryError(t *testing.T) {
	path := write(t, t.TempDir(), "config", "gap x\nnmaster y\nbind Mod+x close\n")
	_, err := Parse(path)
	if err == nil {
		t.Fatal("no error")
	}
	if got := strings.Count(err.Error(), "\n") + 1; got != 2 {
		t.Errorf("got %d errors, want 2:\n%v", got, err)
	}
}

func TestGlob(t *testing.T) {
	tests := []struct {
		pattern, s string
		want       bool
	}{
		{"gimp*", "gimp-2.10", true},
		{"gimp*", "Gimp", false}, // case matters
		{"*pavucontrol", "org.pulseaudio.pavucontrol", true},
		{"*", "", true},
		{"", "", true},
		{"", "x", false},
		{"a?c", "abc", true},
		{"a?c", "ac", false},
		{"*/*", "~/code/go", true}, // * crosses slashes, unlike path.Match
		{"a*b*c", "aXbYbZc", true},
		{"a*b*c", "aXbYbZ", false},
		{"Picture-in-Picture", "Picture-in-Picture", true},
		{"é*", "état", true},
	}
	for _, tt := range tests {
		if got := Glob(tt.pattern, tt.s); got != tt.want {
			t.Errorf("Glob(%q, %q) = %t, want %t", tt.pattern, tt.s, got, tt.want)
		}
	}
}

func TestParseRules(t *testing.T) {
	path := write(t, t.TempDir(), "config", `
rule -app-id gimp* float
rule	-title "Picture in picture"	float
rule -app-id firefox tag 9 tag 8 monitor DP-1
rule -app-id foot -title "*scratch*" tile
`)
	got, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []Rule{
		{AppID: "gimp*", Float: FloatOn},
		{Title: "Picture in picture", Float: FloatOn},
		{AppID: "firefox", Tags: 1<<8 | 1<<7, Output: "DP-1"},
		{AppID: "foot", Title: "*scratch*", Float: FloatOff},
	}
	if !reflect.DeepEqual(got.Rules, want) {
		t.Errorf("rules = %+v\nwant %+v", got.Rules, want)
	}
	// Rules don't touch the default bindings.
	if !reflect.DeepEqual(got.Modes, Default().Modes) {
		t.Error("rules changed the bindings")
	}

	// And they survive a round trip through the generator.
	again, err := Parse(write(t, t.TempDir(), "config", string(Generate(got))))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(again, got) {
		t.Errorf("round trip: got %+v\nwant %+v", again.Rules, got.Rules)
	}
}

func TestParseRuleErrors(t *testing.T) {
	for _, line := range []string{
		"rule float",
		"rule -app-id foot",
		"rule -app-id",
		"rule -app-id foot tag",
		"rule -app-id foot tag 10",
		"rule -app-id foot float tile",
		"rule -app-id foot fly",
		`rule -title "open float`,
	} {
		if _, err := Parse(write(t, t.TempDir(), "config", line)); err == nil {
			t.Errorf("Parse(%q): no error", line)
		}
	}
}

func TestTabs(t *testing.T) {
	path := write(t, t.TempDir(), "config", "gap\t7\nbind\t-repeat\tMod+x\tspawn\tfoot\t-e\ttop\n")
	got, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	b := got.Modes[0].Bindings[0]
	if got.Gap != 7 || !b.Repeat || b.Command.String() != "spawn foot\t-e\ttop" {
		t.Errorf("gap %d, binding %+v", got.Gap, b)
	}
}

func TestParseScratchpads(t *testing.T) {
	path := write(t, t.TempDir(), "config", `
scratchpad term foot-scratch foot --app-id foot-scratch -e  tmux
scratchpad music  music-scratch	foot --app-id music-scratch -e ncmpcpp
bind Super+Return scratchpad term
bind Super+m scratchpad music
`)
	got, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []Scratchpad{
		{"term", "foot-scratch", "foot --app-id foot-scratch -e  tmux"},
		{"music", "music-scratch", "foot --app-id music-scratch -e ncmpcpp"},
	}
	if !reflect.DeepEqual(got.Scratchpads, want) {
		t.Errorf("scratchpads = %+v", got.Scratchpads)
	}
	again, err := Parse(write(t, t.TempDir(), "config", string(Generate(got))))
	if err != nil || !reflect.DeepEqual(again, got) {
		t.Errorf("round trip: %v\n%+v", err, again.Scratchpads)
	}
}

func TestParseScratchpadErrors(t *testing.T) {
	tests := []struct{ config, want string }{
		{"scratchpad term", "scratchpad <name> <app-id> <command>"},
		{"scratchpad term foot-scratch", "scratchpad <name> <app-id> <command>"},
		{"scratchpad a/b x foot", "not a valid name"},
		{"scratchpad a x foot\nscratchpad a y foot", "defined twice"},
		{"scratchpad a x foot\nscratchpad b x foot", "same app-id"},
		{"bind Super+m scratchpad music", "config:1: scratchpad music: no such scratchpad"},
		{"scratchpad music m foot", "default binding Super+Return runs scratchpad term"},
	}
	for _, tt := range tests {
		_, err := Parse(write(t, t.TempDir(), "config", tt.config))
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("Parse(%q) = %v, want an error containing %q", tt.config, err, tt.want)
		}
	}
}
