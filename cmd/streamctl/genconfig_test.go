package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/grunclepug/stream/internal/config"
)

func TestGenConfig(t *testing.T) {
	want := string(config.Generate(config.Default()))
	read := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}

	t.Run("default path", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", dir)
		var out bytes.Buffer
		if err := genConfig(nil, &out); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "stream", "config")
		if read(path) != want {
			t.Error("wrong content")
		}
		if err := genConfig(nil, &out); err == nil || !strings.Contains(err.Error(), "-force") {
			t.Errorf("second run: %v, want a refusal naming -force", err)
		}
		if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := genConfig([]string{"-force"}, &out); err != nil || read(path) != want {
			t.Errorf("-force: %v", err)
		}
	})

	t.Run("directory", func(t *testing.T) {
		dir := t.TempDir()
		var out bytes.Buffer
		if err := genConfig([]string{"-o", dir}, &out); err != nil {
			t.Fatal(err)
		}
		if read(filepath.Join(dir, "config")) != want {
			t.Error("wrong content")
		}
	})

	t.Run("file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "new", "stream.conf")
		var out bytes.Buffer
		if err := genConfig([]string{"-o", path}, &out); err != nil {
			t.Fatal(err)
		}
		if read(path) != want {
			t.Error("wrong content")
		}
	})

	t.Run("stdout", func(t *testing.T) {
		for _, args := range [][]string{{"-o"}, {"-o", "-force"}} {
			var out bytes.Buffer
			if err := genConfig(args, &out); err != nil {
				t.Fatal(err)
			}
			if out.String() != want {
				t.Errorf("%q: wrong output", args)
			}
		}
	})

	t.Run("bad argument", func(t *testing.T) {
		var out bytes.Buffer
		err := genConfig([]string{"-x"}, &out)
		if _, ok := err.(usageError); !ok {
			t.Errorf("got %v, want a usage error", err)
		}
	})
}
