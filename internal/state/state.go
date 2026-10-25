// Package state saves what swm knows about windows and monitors, so a
// restarted swm puts everything back where it was. river keeps the windows
// across a restart and hands them to the new swm with the same identifiers.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// version changes when the format does; a file with another version is
// ignored rather than misread.
const version = 1

// State is the saved state of one river session.
type State struct {
	Version int                `json:"version"`
	Outputs map[string]*Output `json:"outputs"` // by output name
	Windows map[string]*Window `json:"windows"` // by river window identifier

	Focused       string `json:"focused,omitempty"`        // the focused window
	FocusedOutput string `json:"focused_output,omitempty"` // the focused output
}

// Output is a monitor's tags and layouts.
type Output struct {
	Tagset  [2]uint32 `json:"tagset"`
	Seltags int       `json:"seltags"`
	Gap     int32     `json:"gap"`
	Tags    []Tag     `json:"tags"`

	Clients    []string `json:"clients"`     // window identifiers in layout order
	FocusOrder []string `json:"focus_order"` // most recently focused first
}

// Tag is one tag's layout state.
type Tag struct {
	Layouts [2]string `json:"layouts"`
	Sel     int       `json:"sel"`
	NMaster int       `json:"nmaster"`
	MFact   float64   `json:"mfact"`
}

// Window is where a window was and how it was shown.
type Window struct {
	Output     string  `json:"output"`
	Tags       uint32  `json:"tags"`
	Floating   bool    `json:"floating,omitempty"`
	Float      Rect    `json:"float"`
	Weight     float64 `json:"weight,omitempty"`
	Fullscreen bool    `json:"fullscreen,omitempty"`
	Scratch    string  `json:"scratchpad,omitempty"`
}

// Rect is a floating window's geometry.
type Rect struct {
	X, Y, W, H int32
}

// New returns an empty state.
func New() *State {
	return &State{Version: version, Outputs: map[string]*Output{}, Windows: map[string]*Window{}}
}

// Path returns the state file of the river session this process runs in.
func Path() (string, error) {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		return "", errors.New("XDG_RUNTIME_DIR is not set")
	}
	display := os.Getenv("WAYLAND_DISPLAY")
	if display == "" {
		return "", errors.New("WAYLAND_DISPLAY is not set")
	}
	return filepath.Join(dir, "stream-"+filepath.Base(display)+".state.json"), nil
}

// Load reads the state file. A missing file, or one from another version of
// the format, gives an empty state.
func Load(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return New(), nil
	}
	if err != nil {
		return New(), err
	}
	s := New()
	if err := json.Unmarshal(data, s); err != nil {
		return New(), fmt.Errorf("%s: %w", path, err)
	}
	if s.Version != version {
		return New(), nil
	}
	if s.Outputs == nil {
		s.Outputs = map[string]*Output{}
	}
	if s.Windows == nil {
		s.Windows = map[string]*Window{}
	}
	return s, nil
}

// Encode returns the state as the file holds it.
func (s *State) Encode() ([]byte, error) {
	return json.Marshal(s)
}

// Write replaces the file at path with data atomically, so a crash never
// leaves half a file.
func Write(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // fails harmlessly after the rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
