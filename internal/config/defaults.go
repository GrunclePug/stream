package config

import (
	"strconv"

	"github.com/grunclepug/stream/internal/command"
	"github.com/grunclepug/stream/internal/keysym"
	"github.com/grunclepug/stream/internal/layout"
)

// Linux input event codes (linux/input-event-codes.h) for pointer buttons.
const (
	btnLeft   = 0x110
	btnRight  = 0x111
	btnMiddle = 0x112
)

// Default returns the compiled-in defaults: dwm's keys on Mod (Alt), with
// stream's extras on Super. `streamctl gen-config` writes them out as a
// config file.
func Default() Config {
	normal := []Binding{
		key(Mod, "p", "spawn wmenu-run"),
		key(Mod|Shift, "Return", "spawn foot"),
		key(Super, "Return", "scratchpad term"),

		key(Mod, "j", "focus next", repeat),
		key(Mod, "k", "focus prev", repeat),
		key(Mod|Shift, "j", "swap next", repeat),
		key(Mod|Shift, "k", "swap prev", repeat),
		key(Mod, "Return", "zoom"),
		key(Mod|Shift, "c", "close"),
		key(Mod|Shift, "space", "toggle-float"),
		key(Mod|Shift, "f", "toggle-fullscreen"),

		key(Mod, "t", "layout tile"),
		key(Mod, "f", "layout floating"),
		key(Mod, "m", "layout monocle"),
		key(Mod, "space", "layout prev"),
		key(Mod, "h", "mfact -0.05", repeat),
		key(Mod, "l", "mfact +0.05", repeat),
		key(Mod, "i", "nmaster +1"),
		key(Mod, "d", "nmaster -1"),
		key(Mod, "minus", "gap -1", repeat),
		key(Mod, "equal", "gap +1", repeat),
		key(Mod|Shift, "equal", "gap reset"),
	}
	for i := 1; i <= command.NumTags; i++ {
		n := strconv.Itoa(i)
		normal = append(normal,
			key(Mod, n, "view "+n),
			key(Mod|Ctrl, n, "toggle-view "+n),
			key(Mod|Shift, n, "tag "+n),
			key(Mod|Ctrl|Shift, n, "toggle-tag "+n),
		)
	}
	normal = append(normal,
		key(Mod, "0", "view all"),
		key(Mod|Shift, "0", "toggle-tag all"),
		key(Mod, "Tab", "view prev"),

		key(Mod, "comma", "focus-mon prev"),
		key(Mod, "period", "focus-mon next"),
		key(Mod|Shift, "comma", "tag-mon prev"),
		key(Mod|Shift, "period", "tag-mon next"),

		key(Mod|Shift, "r", "reload"),
		key(Mod|Shift, "q", "quit"),
		key(Super, "F11", "mode passthrough"),

		button(Mod, btnLeft, "move"),
		button(Mod, btnMiddle, "toggle-float"),
		button(Mod, btnRight, "resize"),
	)

	return Config{
		BorderWidth:     1,
		BorderFocused:   0x3fa9f5ff, // h2o (examples/h2o)
		BorderUnfocused: 0x1f3a52ff,
		Layout:          layout.Tile,
		NMaster:         1,
		MFact:           0.55,
		Gap:             5,
		Mod:             Alt,
		RepeatDelay:     600,
		RepeatRate:      25,
		Scratchpads: []Scratchpad{
			{Name: "term", AppID: "foot-scratch", Command: "foot --app-id foot-scratch"},
		},
		Modes: []Mode{
			{Name: NormalMode, Bindings: normal},
			{Name: "passthrough", Bindings: []Binding{
				key(Super, "F11", "mode normal"),
			}},
		},
	}
}

type flag func(*Binding)

func repeat(b *Binding) { b.Repeat = true }

// key builds a default key binding. A typo in the defaults panics, which
// every test run catches.
func key(mods Modifiers, name, cmd string, flags ...flag) Binding {
	sym, ok := keysym.FromName(name)
	if !ok {
		panic("config: bad default key " + strconv.Quote(name))
	}
	b := Binding{Mods: mods, Keysym: sym, Command: mustCommand(cmd)}
	for _, f := range flags {
		f(&b)
	}
	return b
}

func button(mods Modifiers, code uint32, cmd string) Binding {
	return Binding{Mods: mods, Button: code, Command: mustCommand(cmd)}
}

func mustCommand(text string) command.Command {
	c, err := command.Parse(text)
	if err != nil {
		panic("config: bad default command " + strconv.Quote(text) + ": " + err.Error())
	}
	return c
}
