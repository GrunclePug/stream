package config

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

const header = `# stream configuration
#
# swm reads this file when it starts and on streamctl reload. streamctl
# gen-config wrote it from swm's built-in defaults.
#
# One directive per line. Lines starting with # are comments; a # anywhere
# else is part of the line, as in a colour. A setting left out keeps its
# default. Bindings are all or nothing: if any bind line is left, the bind
# lines here replace every default binding. streamctl gen-config -missing
# prints the default bindings this file lacks, such as new ones.
#
#   include [-optional] <path>
#                    read another file here (~/ and paths relative to this
#                    file); with -optional, a missing file is skipped
#   bind [-repeat] [-locked] [-also] <keys> <command>
#   mode <name> {    bind lines for a mode, up to a line holding }
#
# Keys are modifiers and a key joined by +. Modifiers: Mod (the mod setting),
# Super, Alt, Ctrl and Shift. Keys are xkb keysym names (xkbcommon-keysyms.h
# without XKB_KEY_), such as Return, comma or XF86AudioMute; letters are
# written in lower case, with Shift for capitals. BTN_LEFT, BTN_MIDDLE and
# BTN_RIGHT make pointer bindings.
#
# -repeat runs the command again while the key is held. -locked keeps the
# binding working while the screen is locked. A key bound more than once runs
# every command, in order, with a warning unless the later bindings say
# -also. Top-level bindings belong to the normal mode, which swm starts in.
#
#   rule [-app-id <glob>] [-title <glob>] <action>...
#
# Rules set up new windows whose app-id and title match; * and ? are
# wildcards, and "double quotes" keep spaces in a value. Actions: float, tile,
# tag <n> and monitor <name> (the output's name, such as DP-1 or eDP-1). Every
# matching rule applies, in order. For example:
#
#   rule -app-id gimp* float
#   rule -title "Picture-in-Picture" float
#
#   scratchpad <name> <app-id> <command>
#
# A scratchpad is a floating window that "scratchpad <name>" shows over the
# current tags and hides again: the first window with the app-id, started with
# the command when there is none. The command has to give its window that
# app-id. Defining any scratchpad here replaces the default ones.
#
#   input <type|name-glob> <option> <value>...
#
# Input lines set libinput options on the devices they match: by type
# (keyboard, pointer, touchpad, touch or tablet; pointer leaves touchpads out)
# or by a glob on the device name. Options: tap, drag, natural-scroll, dwt
# (disable while typing), left-handed and middle-emulation take on or off;
# accel-profile takes flat or adaptive; accel-speed a number from -1 to 1.
# Every matching line applies, in order; options left out keep the device's
# default. For example:
#
#   input touchpad tap on natural-scroll on dwt on
#   input "Logitech*" accel-profile flat
`

// Generate writes cfg as a config file, with comments. Parsing the result
// gives back cfg.
func Generate(cfg Config) []byte {
	var b bytes.Buffer
	b.WriteString(header)

	section(&b, "Look")
	setting(&b, "border-width", strconv.Itoa(int(cfg.BorderWidth)))
	setting(&b, "border-focused", formatColor(cfg.BorderFocused))
	setting(&b, "border-unfocused", formatColor(cfg.BorderUnfocused))

	section(&b, "Layout every tag starts with, and the gap every monitor starts with")
	setting(&b, "layout", cfg.Layout.String())
	setting(&b, "nmaster", strconv.Itoa(cfg.NMaster))
	setting(&b, "mfact", strconv.FormatFloat(cfg.MFact, 'f', -1, 64))
	setting(&b, "gap", strconv.Itoa(int(cfg.Gap)))

	section(&b, "Keyboard: what Mod stands for, and how held keys repeat")
	setting(&b, "mod", strings.ToLower(modifierName(cfg.Mod)))
	setting(&b, "repeat-delay", strconv.Itoa(cfg.RepeatDelay))
	setting(&b, "repeat-rate", strconv.Itoa(cfg.RepeatRate))

	if len(cfg.Scratchpads) > 0 {
		section(&b, "Scratchpads")
		for _, sp := range cfg.Scratchpads {
			fmt.Fprintf(&b, "scratchpad %s %s %s\n", sp.Name, sp.AppID, sp.Command)
		}
	}

	if len(cfg.Rules) > 0 {
		section(&b, "Rules")
		for _, r := range cfg.Rules {
			b.WriteString(formatRule(r) + "\n")
		}
	}

	if len(cfg.Inputs) > 0 {
		section(&b, "Input devices")
		for _, in := range cfg.Inputs {
			b.WriteString(formatInput(in) + "\n")
		}
	}

	for _, m := range cfg.Modes {
		if m.Name == NormalMode {
			bindings(&b, m.Bindings, "", true)
			continue
		}
		fmt.Fprintf(&b, "\nmode %s {\n", m.Name)
		bindings(&b, m.Bindings, "\t", false)
		b.WriteString("}\n")
	}
	return b.Bytes()
}

func section(b *bytes.Buffer, title string) {
	fmt.Fprintf(b, "\n# %s\n", title)
}

func setting(b *bytes.Buffer, name, value string) {
	fmt.Fprintf(b, "%s %s\n", name, value)
}

// bindings writes one mode's bindings with their commands lined up. With
// headings, a comment names each group of related bindings.
func bindings(b *bytes.Buffer, bs []Binding, indent string, headings bool) {
	lefts := make([]string, len(bs))
	width := 0
	for i, bind := range bs {
		lefts[i] = bindLeft(bind)
		width = max(width, len(lefts[i]))
	}
	group := ""
	for i, bind := range bs {
		if g := groupOf(bind); headings && g != group {
			section(b, g)
			group = g
		}
		fmt.Fprintf(b, "%s%-*s %s\n", indent, width, lefts[i], bind.Command)
	}
}

// bindLeft is a bind line up to its command: the flags and keys.
func bindLeft(b Binding) string {
	left := "bind"
	if b.Repeat {
		left += " -repeat"
	}
	if b.Locked {
		left += " -locked"
	}
	if b.Also {
		left += " -also"
	}
	return left + " " + b.Keys()
}

// groupOf names the group a binding is listed under.
func groupOf(b Binding) string {
	if b.Button != 0 {
		return "Mouse"
	}
	switch b.Command.Name {
	case "spawn", "scratchpad":
		return "Programs"
	case "focus", "swap", "zoom", "close", "toggle-float", "toggle-fullscreen":
		return "Windows"
	case "layout", "mfact", "nmaster", "gap", "reset":
		return "Layout"
	case "view", "toggle-view", "tag", "toggle-tag":
		return "Tags"
	case "focus-mon", "tag-mon":
		return "Monitors"
	}
	return "Session"
}

func formatRule(r Rule) string {
	words := []string{"rule"}
	if r.AppID != "" {
		words = append(words, "-app-id", quote(r.AppID))
	}
	if r.Title != "" {
		words = append(words, "-title", quote(r.Title))
	}
	switch r.Float {
	case FloatOn:
		words = append(words, "float")
	case FloatOff:
		words = append(words, "tile")
	}
	for i := range 32 {
		if r.Tags&(1<<i) != 0 {
			words = append(words, "tag", strconv.Itoa(i+1))
		}
	}
	if r.Output != "" {
		words = append(words, "monitor", quote(r.Output))
	}
	return strings.Join(words, " ")
}

// quote wraps a rule value in double quotes if it has a space or is empty.
func quote(s string) string {
	if s == "" || strings.ContainsAny(s, " \t") {
		return `"` + s + `"`
	}
	return s
}

func formatColor(c Color) string {
	if c&0xff == 0xff {
		return fmt.Sprintf("#%06x", uint32(c>>8))
	}
	return fmt.Sprintf("#%08x", uint32(c))
}
