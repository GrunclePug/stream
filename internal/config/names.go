package config

import (
	"fmt"
	"strings"

	"github.com/grunclepug/stream/internal/keysym"
)

// modifierNames are the modifiers in the order the generator writes them.
var modifierNames = []struct {
	name string
	mod  Modifiers
}{
	{"Mod", Mod},
	{"Super", Super},
	{"Alt", Alt},
	{"Ctrl", Ctrl},
	{"Shift", Shift},
	{"Mod3", Mod3},
	{"Mod5", Mod5},
}

// modifierAliases are other names the parser accepts.
var modifierAliases = map[string]Modifiers{
	"control": Ctrl,
	"logo":    Super,
	"mod1":    Alt,
	"mod4":    Super,
}

func modifierFromName(name string) (Modifiers, bool) {
	lower := strings.ToLower(name)
	for _, m := range modifierNames {
		if strings.ToLower(m.name) == lower {
			return m.mod, true
		}
	}
	m, ok := modifierAliases[lower]
	return m, ok
}

func modifierName(mod Modifiers) string {
	for _, m := range modifierNames {
		if m.mod == mod {
			return m.name
		}
	}
	return fmt.Sprintf("%#x", uint32(mod))
}

// buttons are the pointer buttons a binding can use, with their Linux input
// event codes.
var buttons = []struct {
	name string
	code uint32
}{
	{"BTN_LEFT", btnLeft},
	{"BTN_RIGHT", btnRight},
	{"BTN_MIDDLE", btnMiddle},
	{"BTN_SIDE", 0x113},
	{"BTN_EXTRA", 0x114},
	{"BTN_FORWARD", 0x115},
	{"BTN_BACK", 0x116},
	{"BTN_TASK", 0x117},
}

// parseKeys parses a binding's keys, such as Mod+Shift+Return or
// Mod+BTN_LEFT.
func parseKeys(s string) (Binding, error) {
	parts := strings.Split(s, "+")
	var b Binding
	for _, p := range parts[:len(parts)-1] {
		m, ok := modifierFromName(p)
		if !ok {
			return b, fmt.Errorf("unknown modifier %q in %q", p, s)
		}
		if b.Mods&m != 0 {
			return b, fmt.Errorf("modifier %s twice in %q", p, s)
		}
		b.Mods |= m
	}
	last := parts[len(parts)-1]
	if last == "" {
		return b, fmt.Errorf("missing key in %q", s)
	}
	for _, btn := range buttons {
		if strings.EqualFold(btn.name, last) {
			b.Button = btn.code
			return b, nil
		}
	}
	sym, ok := keysym.FromName(last)
	if !ok {
		return b, fmt.Errorf("unknown key %q (keys are xkb keysym names, such as Return or comma)", last)
	}
	b.Keysym = sym
	return b, nil
}

// Line returns the binding as a bind line of the config file.
func (b Binding) Line() string {
	return bindLeft(b) + " " + b.Command.String()
}

// Keys returns the binding's keys as the config writes them.
func (b Binding) Keys() string {
	var parts []string
	for _, m := range modifierNames {
		if b.Mods&m.mod != 0 {
			parts = append(parts, m.name)
		}
	}
	if b.Button != 0 {
		name := fmt.Sprintf("%#x", b.Button)
		for _, btn := range buttons {
			if btn.code == b.Button {
				name = btn.name
			}
		}
		return strings.Join(append(parts, name), "+")
	}
	name, ok := keysym.Name(b.Keysym)
	if !ok {
		name = fmt.Sprintf("%#x", b.Keysym)
	}
	return strings.Join(append(parts, name), "+")
}
