package command

import (
	"slices"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		text string
		want Command
	}{
		{"close", Command{Name: "close"}},
		{"  zoom  ", Command{Name: "zoom"}},
		{"spawn foot", Command{Name: "spawn", Args: []string{"foot"}}},
		{"spawn  foot -e 'htop  -d 5'", Command{Name: "spawn", Args: []string{"foot -e 'htop  -d 5'"}}},
		{"focus next", Command{Name: "focus", Args: []string{"next"}}},
		{"view 3", Command{Name: "view", Args: []string{"3"}}},
		{"view 03", Command{Name: "view", Args: []string{"3"}}},
		{"view all", Command{Name: "view", Args: []string{"all"}}},
		{"view prev", Command{Name: "view", Args: []string{"prev"}}},
		{"tag all", Command{Name: "tag", Args: []string{"all"}}},
		{"toggle-tag 9", Command{Name: "toggle-tag", Args: []string{"9"}}},
		{"mfact +0.05", Command{Name: "mfact", Args: []string{"+0.05"}}},
		{"mfact -.050", Command{Name: "mfact", Args: []string{"-0.05"}}},
		{"mfact 0.55", Command{Name: "mfact", Args: []string{"0.55"}}},
		{"nmaster +1", Command{Name: "nmaster", Args: []string{"+1"}}},
		{"nmaster -1", Command{Name: "nmaster", Args: []string{"-1"}}},
		{"nmaster 0", Command{Name: "nmaster", Args: []string{"0"}}},
		{"gap reset", Command{Name: "gap", Args: []string{"reset"}}},
		{"gap +2", Command{Name: "gap", Args: []string{"+2"}}},
		{"layout prev", Command{Name: "layout", Args: []string{"prev"}}},
		{"mode resize", Command{Name: "mode", Args: []string{"resize"}}},
	}
	for _, tt := range tests {
		got, err := Parse(tt.text)
		if err != nil {
			t.Errorf("Parse(%q): %v", tt.text, err)
			continue
		}
		if got.Name != tt.want.Name || !slices.Equal(got.Args, tt.want.Args) {
			t.Errorf("Parse(%q) = %#v, want %#v", tt.text, got, tt.want)
		}
		// The normalised form parses back to the same command.
		again, err := Parse(got.String())
		if err != nil || again.String() != got.String() {
			t.Errorf("Parse(%q) = %q, %v, want %q", got.String(), again.String(), err, got.String())
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, text := range []string{
		"",
		"nosuch",
		"spawn",
		"close now",
		"focus left",
		"focus",
		"focus next prev",
		"view 0",
		"view 10",
		"toggle-view all",
		"tag prev",
		"layout grid",
		"mfact 1",
		"mfact 0.01",
		"mfact abc",
		"nmaster -",
		"nmaster 1.5",
		"gap -",
		"mode",
		"mode bad/name",
	} {
		if c, err := Parse(text); err == nil {
			t.Errorf("Parse(%q) = %#v, want an error", text, c)
		}
	}
}
