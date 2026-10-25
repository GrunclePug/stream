package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMissingProgram(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "here"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	tests := []struct{ cmdline, want string }{
		{"here -x", ""},
		{"exec here", ""},
		{"gone --flag", "gone"},
		{"exec gone", "gone"},
		{filepath.Join(bin, "here"), ""},
		{"/no/such/prog", "/no/such/prog"},
		{"$TERMINAL -e top", ""}, // not a plain word: not checked
		{"FOO=1 gone", ""},       // an assignment first: not checked
		{"", ""},
	}
	for _, tt := range tests {
		if got := missingProgram(tt.cmdline); got != tt.want {
			t.Errorf("missingProgram(%q) = %q, want %q", tt.cmdline, got, tt.want)
		}
	}
}

func TestCheck(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "foot"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	path := write(t, t.TempDir(), "config", "scratchpad term term gone-term\nbind Mod+p spawn gone-menu\nbind Mod+t spawn foot\nbind Mod+x close\nbind Mod+x zoom\nbind Super+Return scratchpad term\n")
	warnings, err := Check(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"config:5: bind Mod+x: the key also runs close",
		"config:2: bind Mod+p: gone-menu is not installed",
		"config:1: scratchpad term: gone-term is not installed",
	}
	if len(warnings) != len(want) {
		t.Fatalf("warnings %v, want %d", warnings, len(want))
	}
	for i, w := range want {
		if !strings.Contains(warnings[i].Error(), w) {
			t.Errorf("warning %q, want %q", warnings[i], w)
		}
	}
}

// With no bindings or scratchpads of its own, the defaults are checked.
func TestCheckDefaults(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	path := write(t, t.TempDir(), "config", "gap 2\n")
	warnings, err := Check(path)
	if err != nil {
		t.Fatal(err)
	}
	var text []string
	for _, w := range warnings {
		text = append(text, w.Error())
	}
	all := strings.Join(text, "\n")
	for _, want := range []string{"default binding Mod+p runs wmenu-run", "default scratchpad term runs foot"} {
		if !strings.Contains(all, want) {
			t.Errorf("warnings lack %q:\n%s", want, all)
		}
	}
}
