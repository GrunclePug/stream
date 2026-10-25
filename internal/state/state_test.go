package state

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s := New()
	s.Focused, s.FocusedOutput = "abc", "DP-1"
	s.Outputs["DP-1"] = &Output{
		Tagset: [2]uint32{1, 4}, Seltags: 1, Gap: 8,
		Tags:       []Tag{{Layouts: [2]string{"tile", "monocle"}, Sel: 1, NMaster: 2, MFact: 0.6}},
		Clients:    []string{"abc", "def"},
		FocusOrder: []string{"def", "abc"},
	}
	s.Windows["abc"] = &Window{Output: "DP-1", Tags: 4, Floating: true, Float: Rect{1, 2, 3, 4}, Scratch: "term"}
	data, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(path, data); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, s) {
		t.Errorf("Load(Write(s)) = %+v, want %+v", got, s)
	}
	// Nothing but the file is left in the directory.
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("%d files left, want 1", len(entries))
	}
}

func TestLoadMissingOrOld(t *testing.T) {
	dir := t.TempDir()
	if s, err := Load(filepath.Join(dir, "none.json")); err != nil || len(s.Windows) != 0 {
		t.Errorf("missing file: %v", err)
	}
	old := filepath.Join(dir, "old.json")
	os.WriteFile(old, []byte(`{"version":0,"windows":{"x":{"tags":1}}}`), 0o644)
	if s, err := Load(old); err != nil || len(s.Windows) != 0 {
		t.Errorf("old version: %v, %d windows", err, len(s.Windows))
	}
	bad := filepath.Join(dir, "bad.json")
	os.WriteFile(bad, []byte(`{`), 0o644)
	if _, err := Load(bad); err == nil {
		t.Error("no error for a broken file")
	}
}
