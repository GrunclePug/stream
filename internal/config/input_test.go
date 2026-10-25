package config

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseInputs(t *testing.T) {
	path := write(t, t.TempDir(), "config", `
input touchpad tap on natural-scroll on dwt on
input	"SynPS/2 Synaptics TouchPad"	drag off left-handed on middle-emulation off
input pointer accel-profile flat accel-speed -0.5
`)
	got, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	speed := -0.5
	want := []Input{
		{Match: "touchpad", Tap: SwitchOn, NaturalScroll: SwitchOn, DWT: SwitchOn},
		{Match: "SynPS/2 Synaptics TouchPad", Drag: SwitchOff, LeftHanded: SwitchOn, MiddleEmulation: SwitchOff},
		{Match: "pointer", AccelProfile: AccelFlat, AccelSpeed: &speed},
	}
	if !reflect.DeepEqual(got.Inputs, want) {
		t.Errorf("inputs = %+v\nwant %+v", got.Inputs, want)
	}
	again, err := Parse(write(t, t.TempDir(), "config", string(Generate(got))))
	if err != nil || !reflect.DeepEqual(again, got) {
		t.Errorf("round trip: %v\n%+v", err, again.Inputs)
	}
}

func TestParseInputErrors(t *testing.T) {
	tests := []struct{ config, want string }{
		{"input", "missing device"},
		{"input touchpad", "needs an option"},
		{"input touchpad tap", "tap: missing value"},
		{"input touchpad tap yes", `"yes" is not on or off`},
		{"input touchpad tap on tap off", "tap is set twice"},
		{"input touchpad scroll on", `unknown option "scroll"`},
		{"input pointer accel-profile fast", "not flat or adaptive"},
		{"input pointer accel-speed 2", "not a number from -1 to 1"},
		{`input "Logi tap on`, `missing closing "`},
	}
	for _, tt := range tests {
		_, err := Parse(write(t, t.TempDir(), "config", tt.config))
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("Parse(%q) = %v, want an error containing %q", tt.config, err, tt.want)
		}
	}
}

func TestInputFor(t *testing.T) {
	slow, fast := -0.5, 0.5
	cfg := Config{Inputs: []Input{
		{Match: "touchpad", Tap: SwitchOn, NaturalScroll: SwitchOn},
		{Match: "*Synaptics*", NaturalScroll: SwitchOff, AccelSpeed: &slow},
		{Match: "pointer", AccelProfile: AccelFlat, AccelSpeed: &fast},
		{Match: "keyboard", Tap: SwitchOff}, // a type word never matches a name
	}}
	tests := []struct {
		typ, name string
		want      Input
	}{
		{"touchpad", "SynPS/2 Synaptics TouchPad", Input{Tap: SwitchOn, NaturalScroll: SwitchOff, AccelSpeed: &slow}},
		{"touchpad", "ELAN Touchpad", Input{Tap: SwitchOn, NaturalScroll: SwitchOn}},
		{"pointer", "Logitech USB Receiver", Input{AccelProfile: AccelFlat, AccelSpeed: &fast}},
		{"pointer", "keyboard", Input{AccelProfile: AccelFlat, AccelSpeed: &fast}},
		{"tablet", "Wacom", Input{}},
	}
	for _, tt := range tests {
		if got := cfg.InputFor(tt.typ, tt.name); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("InputFor(%q, %q) = %+v, want %+v", tt.typ, tt.name, got, tt.want)
		}
	}
}
