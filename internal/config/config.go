// Package config holds swm's settings: the compiled-in defaults, the config
// file parser, and the generator that writes the defaults as a config file.
package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/grunclepug/stream/internal/command"
	"github.com/grunclepug/stream/internal/layout"
)

// Color is an RGBA color, 0xRRGGBBAA, not premultiplied.
type Color uint32

// RGBA32 returns the color as the four premultiplied 32-bit channels river's
// set_borders request takes.
func (c Color) RGBA32() (r, g, b, a uint32) {
	r8, g8, b8, a8 := uint32(c>>24), uint32(c>>16&0xff), uint32(c>>8&0xff), uint32(c&0xff)
	premul := func(v uint32) uint32 { return (v * a8 / 0xff) * 0x01010101 }
	return premul(r8), premul(g8), premul(b8), a8 * 0x01010101
}

// Modifiers is a set of keyboard modifiers. The values are those of river's
// river_seat_v1.modifiers, plus Mod.
type Modifiers uint32

const (
	Shift Modifiers = 1
	Ctrl  Modifiers = 4
	Alt   Modifiers = 8
	Mod3  Modifiers = 32
	Super Modifiers = 64
	Mod5  Modifiers = 128

	// Mod stands for the modifier the mod setting names, so changing that
	// setting moves every binding written with Mod.
	Mod Modifiers = 1 << 31
)

// Resolve replaces Mod with the modifier it stands for.
func (m Modifiers) Resolve(mod Modifiers) Modifiers {
	if m&Mod != 0 {
		m = m&^Mod | mod
	}
	return m
}

// SameKey reports whether two bindings are on the same key or button, with
// Mod standing for mod.
func SameKey(a, b Binding, mod Modifiers) bool {
	return a.Keysym == b.Keysym && a.Button == b.Button && a.Mods.Resolve(mod) == b.Mods.Resolve(mod)
}

// Binding runs a command when a key or pointer button is pressed with the
// given modifiers held.
type Binding struct {
	Mods    Modifiers
	Keysym  uint32 // an xkbcommon keysym, for a key binding
	Button  uint32 // a Linux input event code such as BTN_LEFT, for a pointer binding
	Repeat  bool   // repeat the command while the key is held
	Locked  bool   // keep the binding active while the session is locked
	Also    bool   // meant to share its key with an earlier binding
	Command command.Command
}

// Mode is a named set of bindings. Exactly one mode is active at a time.
type Mode struct {
	Name     string
	Bindings []Binding
}

// NormalMode is the mode swm starts in, which holds the top-level bindings.
const NormalMode = "normal"

// Config is the complete set of swm settings.
type Config struct {
	BorderWidth     int32
	BorderFocused   Color
	BorderUnfocused Color

	// Layout, NMaster and MFact are what every tag starts with. Gap is what
	// every monitor starts with.
	Layout  layout.Kind
	NMaster int
	MFact   float64
	Gap     int32

	Mod Modifiers // what Mod in a binding stands for

	// RepeatDelay (milliseconds) and RepeatRate (per second) time the
	// repeating of -repeat bindings.
	RepeatDelay int
	RepeatRate  int

	// Modes holds every binding. Modes[0] is the normal mode.
	Modes []Mode

	// Warnings are what's odd about the config file but doesn't stop it
	// from working, such as a key bound twice without -also.
	Warnings []*Error

	// Rules set up new windows that match them, in order.
	Rules []Rule

	// Scratchpads are the windows the scratchpad command shows and hides.
	Scratchpads []Scratchpad
}

// Scratchpad is a named window kept out of sight until it is called up: the
// first window with the app-id, started with the command when there is none.
type Scratchpad struct {
	Name    string
	AppID   string
	Command string // run with sh -c
}

// Scratchpad returns the scratchpad called name.
func (c Config) Scratchpad(name string) (Scratchpad, bool) {
	for _, sp := range c.Scratchpads {
		if sp.Name == name {
			return sp, true
		}
	}
	return Scratchpad{}, false
}

// Rule matches new windows by app-id and title and changes where and how
// they open. Every matching rule applies, in order, as in dwm.
type Rule struct {
	AppID string // a glob; empty matches any window
	Title string // a glob; empty matches any window

	Float  Floating
	Tags   uint32 // a tag mask; 0 leaves the window on the shown tags
	Output string // an output name such as DP-1; empty leaves it on the focused one
}

// Floating is what a rule says about floating.
type Floating int

const (
	FloatDefault Floating = iota // float dialogs and fixed-size windows only
	FloatOn
	FloatOff
)

// Match reports whether the rule matches a window with the given app-id
// and title.
func (r Rule) Match(appID, title string) bool {
	return (r.AppID == "" || Glob(r.AppID, appID)) && (r.Title == "" || Glob(r.Title, title))
}

// Glob reports whether s matches pattern, where * matches any run of
// characters, including none, and ? matches any one character.
func Glob(pattern, s string) bool {
	p, str := []rune(pattern), []rune(s)
	// star and next are where to resume after the last *: the classic
	// backtracking matcher, linear in practice.
	star, next := -1, 0
	i, j := 0, 0
	for j < len(str) {
		switch {
		case i < len(p) && (p[i] == '?' || p[i] == str[j]):
			i++
			j++
		case i < len(p) && p[i] == '*':
			star, next = i, j
			i++
		case star >= 0:
			next++
			i, j = star+1, next
		default:
			return false
		}
	}
	for i < len(p) && p[i] == '*' {
		i++
	}
	return i == len(p)
}

// Path returns where swm looks for its config file:
// $XDG_CONFIG_HOME/stream/config, or ~/.config/stream/config.
func Path() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", errors.New("neither XDG_CONFIG_HOME nor HOME is set")
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "stream", "config"), nil
}
