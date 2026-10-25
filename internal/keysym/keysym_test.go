package keysym

import "testing"

func TestFromName(t *testing.T) {
	tests := []struct {
		name string
		sym  uint32
	}{
		{"Return", 0xff0d},
		{"return", 0xff0d},
		{"j", 0x006a},
		{"J", 0x006a}, // the lower-case keysym wins, as in xkbcommon
		{"1", 0x0031},
		{"space", 0x0020},
		{"Page_Up", 0xff55}, // deprecated alias, still accepted
		{"XF86AudioRaiseVolume", 0x1008ff13},
		{"xf86audioraisevolume", 0x1008ff13},
	}
	for _, tt := range tests {
		sym, ok := FromName(tt.name)
		if !ok || sym != tt.sym {
			t.Errorf("FromName(%q) = %#x, %t, want %#x, true", tt.name, sym, ok, tt.sym)
		}
	}
	if sym, ok := FromName("NoSuchKey"); ok {
		t.Errorf("FromName(NoSuchKey) = %#x, true, want false", sym)
	}
}

func TestName(t *testing.T) {
	tests := []struct {
		sym  uint32
		name string
	}{
		{0xff0d, "Return"},
		{0x006a, "j"},
		{0xff55, "Prior"}, // not the deprecated Page_Up
		{0x1008ff13, "XF86AudioRaiseVolume"},
	}
	for _, tt := range tests {
		name, ok := Name(tt.sym)
		if !ok || name != tt.name {
			t.Errorf("Name(%#x) = %q, %t, want %q, true", tt.sym, name, ok, tt.name)
		}
	}
}

// TestRoundTrip checks that the name the config generator writes for any
// keysym the parser can produce parses back to the same keysym.
func TestRoundTrip(t *testing.T) {
	for _, e := range table {
		sym, _ := FromName(e.name)
		name, ok := Name(sym)
		if !ok {
			t.Fatalf("Name(%#x) has no name", sym)
		}
		if got, _ := FromName(name); got != sym {
			t.Errorf("%q: FromName(%q) = %#x, want %#x", e.name, name, got, sym)
		}
	}
}
