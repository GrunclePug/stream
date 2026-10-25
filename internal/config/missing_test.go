package config

import (
	"errors"
	"strings"
	"testing"
)

func TestFindMissingDefaults(t *testing.T) {
	path := write(t, t.TempDir(), "config", "gap 3\n")
	if _, err := FindMissing(path); !errors.Is(err, ErrDefaultBindings) {
		t.Errorf("FindMissing on a file without bindings = %v, want ErrDefaultBindings", err)
	}
}

func TestFindMissing(t *testing.T) {
	// Every default but a few, with close moved and quit on toggle-tag
	// all's key, and no passthrough mode.
	var b strings.Builder
	b.WriteString("scratchpad other other foot\n")
	for _, bind := range Default().Modes[0].Bindings {
		switch bind.Command.String() {
		case "close":
			b.WriteString("bind Mod+q close\n")
		case "quit":
			b.WriteString("bind Alt+Shift+0 quit\n") // Alt is Mod
		case "toggle-tag all", "scratchpad term", "mode passthrough", "view 3":
		default:
			b.WriteString(bind.Line() + "\n")
		}
	}
	path := write(t, t.TempDir(), "config", b.String())
	ms, err := FindMissing(path)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Missing{}
	for _, m := range ms {
		got[m.Mode+": "+m.Binding.Command.String()] = m
	}
	want := map[string]Missing{
		"normal: view 3":           {},
		"normal: toggle-tag all":   {Note: "Mod+Shift+0 is taken by quit: pick another key"},
		"normal: scratchpad term":  {Note: "needs scratchpad term: define it"},
		"normal: mode passthrough": {},
		"passthrough: mode normal": {NewMode: true},
	}
	if len(got) != len(want) {
		t.Errorf("missing %d bindings, want %d: %v", len(got), len(want), got)
	}
	for name, w := range want {
		g, ok := got[name]
		if !ok {
			t.Errorf("%s not reported", name)
			continue
		}
		if g.Note != w.Note || g.NewMode != w.NewMode {
			t.Errorf("%s: note %q, new mode %v; want %q, %v", name, g.Note, g.NewMode, w.Note, w.NewMode)
		}
	}
}
