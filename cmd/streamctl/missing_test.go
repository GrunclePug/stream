package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/grunclepug/stream/internal/config"
)

func TestGenConfigMissing(t *testing.T) {
	// The defaults without close and without the passthrough mode.
	text := string(config.Generate(config.Default()))
	text = strings.Replace(text, "bind Mod+Shift+c         close\n", "", 1)
	text = text[:strings.Index(text, "\nmode passthrough {")] + "\n"
	text = strings.Replace(text, "bind Super+F11           mode passthrough\n", "", 1)
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := genConfig([]string{"-missing", "-o", path}, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"\nbind Mod+Shift+c close\nbind Super+F11 mode passthrough\n",
		"\nmode passthrough {\n\tbind Super+F11 mode normal\n}\n",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}

	// Pasted in, the output completes the config.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.Write(out.Bytes())
	f.Close()
	out.Reset()
	if err := genConfig([]string{"-missing", "-o", path}, &out); err != nil || !strings.Contains(out.String(), "every default binding") {
		t.Errorf("after pasting: %v\n%s", err, out.String())
	}
}

func TestGenConfigCheck(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "foot"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin) // for the default scratchpad
	dir := t.TempDir()
	run := func(text string) (string, error) {
		t.Helper()
		path := filepath.Join(dir, "config")
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		err := genConfig([]string{"-check", "-o", path}, &out)
		return out.String(), err
	}

	if out, err := run("bind Mod+x close\nbind -also Mod+x zoom\n"); err != nil || !strings.Contains(out, "no problems found") {
		t.Errorf("clean config: %v\n%s", err, out)
	}
	if out, err := run("bind Mod+x close\nbind Mod+x zoom\nbind Mod+p spawn gone\n"); err != nil || !strings.Contains(out, "2 warnings; swm loads it") {
		t.Errorf("warnings: %v\n%s", err, out)
	}
	if out, err := run("gap x\nbind Mod+x nope\n"); err == nil || !strings.Contains(err.Error(), "2 errors") || !strings.Contains(out, "error: ") {
		t.Errorf("errors: %v\n%s", err, out)
	}
}
