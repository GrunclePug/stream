package config

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Input sets libinput options on the input devices it matches. Options it
// leaves at their zero value keep the device's own default.
type Input struct {
	// Match is a device type (see InputTypes) or a glob on the device's
	// name.
	Match string

	Tap             Switch // tap-to-click
	Drag            Switch // tap-and-drag
	NaturalScroll   Switch
	DWT             Switch // disable while typing
	LeftHanded      Switch
	MiddleEmulation Switch // a left and right press together is a middle press

	AccelProfile AccelProfile
	AccelSpeed   *float64 // -1 to 1; nil keeps the device's default
}

// InputTypes are the device types an input line can match, as river reports
// them, plus touchpad: a pointer that can tap. pointer leaves touchpads out.
var InputTypes = []string{"keyboard", "pointer", "touchpad", "touch", "tablet"}

// Switch is an on or off libinput option.
type Switch int

const (
	SwitchDefault Switch = iota // keep the device's default
	SwitchOn
	SwitchOff
)

// AccelProfile is a libinput pointer acceleration profile.
type AccelProfile int

const (
	AccelDefault AccelProfile = iota // keep the device's default
	AccelFlat
	AccelAdaptive
)

func (p AccelProfile) String() string {
	switch p {
	case AccelFlat:
		return "flat"
	case AccelAdaptive:
		return "adaptive"
	}
	return "default"
}

// inputSwitches are the on/off options, in the order the generator writes
// them.
var inputSwitches = []struct {
	name string
	get  func(*Input) *Switch
}{
	{"tap", func(in *Input) *Switch { return &in.Tap }},
	{"drag", func(in *Input) *Switch { return &in.Drag }},
	{"natural-scroll", func(in *Input) *Switch { return &in.NaturalScroll }},
	{"dwt", func(in *Input) *Switch { return &in.DWT }},
	{"left-handed", func(in *Input) *Switch { return &in.LeftHanded }},
	{"middle-emulation", func(in *Input) *Switch { return &in.MiddleEmulation }},
}

// InputFor returns the options for a device of the given type and name:
// every matching input line applies, in order, so a later line wins.
func (c Config) InputFor(typ, name string) Input {
	var out Input
	for _, in := range c.Inputs {
		if !in.Matches(typ, name) {
			continue
		}
		for _, s := range inputSwitches {
			if v := *s.get(&in); v != SwitchDefault {
				*s.get(&out) = v
			}
		}
		if in.AccelProfile != AccelDefault {
			out.AccelProfile = in.AccelProfile
		}
		if in.AccelSpeed != nil {
			out.AccelSpeed = in.AccelSpeed
		}
	}
	return out
}

// Matches reports whether the line matches a device of the given type and
// name. A type word matches by type only, never as a name.
func (in Input) Matches(typ, name string) bool {
	if slices.Contains(InputTypes, in.Match) {
		return in.Match == typ
	}
	return Glob(in.Match, name)
}

// input parses "<type|name-glob> <option> <value>...".
func (p *parser) input(rest string) error {
	words, err := splitQuoted(rest)
	if err != nil {
		return err
	}
	if len(words) == 0 {
		return errors.New("missing device: input lines are input <type|name-glob> <option> <value>")
	}
	in := Input{Match: words[0]}
	if in.Match == "" {
		return errors.New("empty device match")
	}
	seen := map[string]bool{}
	opts := words[1:]
	if len(opts) == 0 {
		return errors.New("needs an option, such as tap on")
	}
	for i := 0; i < len(opts); i += 2 {
		name := opts[i]
		if i+1 >= len(opts) {
			return fmt.Errorf("%s: missing value", name)
		}
		v := opts[i+1]
		if seen[name] {
			return fmt.Errorf("%s is set twice", name)
		}
		seen[name] = true
		if err := in.set(name, v); err != nil {
			return err
		}
	}
	p.inputs = append(p.inputs, in)
	return nil
}

func (in *Input) set(name, v string) error {
	for _, s := range inputSwitches {
		if s.name != name {
			continue
		}
		switch v {
		case "on":
			*s.get(in) = SwitchOn
		case "off":
			*s.get(in) = SwitchOff
		default:
			return fmt.Errorf("%s: %q is not on or off", name, v)
		}
		return nil
	}
	switch name {
	case "accel-profile":
		switch v {
		case "flat":
			in.AccelProfile = AccelFlat
		case "adaptive":
			in.AccelProfile = AccelAdaptive
		default:
			return fmt.Errorf("accel-profile: %q is not flat or adaptive", v)
		}
		return nil
	case "accel-speed":
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f < -1 || f > 1 {
			return fmt.Errorf("accel-speed: %q is not a number from -1 to 1", v)
		}
		in.AccelSpeed = &f
		return nil
	}
	return fmt.Errorf("unknown option %q (options are %s, accel-profile and accel-speed)", name, switchNames())
}

func switchNames() string {
	names := make([]string, len(inputSwitches))
	for i, s := range inputSwitches {
		names[i] = s.name
	}
	return strings.Join(names, ", ")
}

func formatInput(in Input) string {
	words := []string{"input", quote(in.Match)}
	for _, s := range inputSwitches {
		switch *s.get(&in) {
		case SwitchOn:
			words = append(words, s.name, "on")
		case SwitchOff:
			words = append(words, s.name, "off")
		}
	}
	if in.AccelProfile != AccelDefault {
		words = append(words, "accel-profile", in.AccelProfile.String())
	}
	if in.AccelSpeed != nil {
		words = append(words, "accel-speed", strconv.FormatFloat(*in.AccelSpeed, 'f', -1, 64))
	}
	return strings.Join(words, " ")
}
