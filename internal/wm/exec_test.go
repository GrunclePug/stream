package wm

import (
	"slices"
	"testing"

	"github.com/grunclepug/stream/internal/layout"
)

func TestView(t *testing.T) {
	o := &Output{tagset: [2]uint32{1, 1}}
	steps := []struct {
		mask uint32
		want uint32
	}{
		{tagMask("3"), 1 << 2},
		{tagMask("3"), 1 << 2}, // already shown: nothing changes
		{tagMask("prev"), 1},
		{tagMask("prev"), 1 << 2},
		{tagMask("all"), allTags},
		{tagMask("prev"), 1 << 2},
	}
	for i, s := range steps {
		o.view(s.mask)
		if got := o.tags(); got != s.want {
			t.Errorf("step %d: tags = %#b, want %#b", i, got, s.want)
		}
	}
}

func TestSetLayout(t *testing.T) {
	o := &Output{tagset: [2]uint32{1, 1}}
	o.pertag[0].kinds = [2]layout.Kind{layout.Tile, layout.Tile}
	steps := []struct {
		name string
		want layout.Kind
	}{
		{"monocle", layout.Monocle},
		{"prev", layout.Tile},
		{"prev", layout.Monocle},
		{"monocle", layout.Monocle}, // already in use: stays
		{"floating", layout.Floating},
		{"prev", layout.Monocle},
		{"tile", layout.Tile},
		{"prev", layout.Monocle},
	}
	for i, s := range steps {
		o.setLayout(s.name)
		if got := o.params().Kind; got != s.want {
			t.Errorf("step %d (%s): layout = %s, want %s", i, s.name, got, s.want)
		}
	}
}

// TestPertag checks each tag keeps its own layout, and that with several
// tags shown the lowest one's applies.
func TestPertag(t *testing.T) {
	o := &Output{tagset: [2]uint32{1, 1}}
	for i := range o.pertag {
		o.pertag[i] = tagLayout{mfact: 0.55}
	}
	o.lay().mfact = 0.7
	o.view(tagMask("2"))
	if got := o.params().MFact; got != 0.55 {
		t.Errorf("tag 2 mfact = %v, want 0.55", got)
	}
	o.tagset[o.seltags] |= tagMask("1")
	if got := o.params().MFact; got != 0.7 {
		t.Errorf("tags 1+2 mfact = %v, want tag 1's 0.7", got)
	}
}

func TestNeighbour(t *testing.T) {
	a, b, c := &Window{}, &Window{}, &Window{}
	ws := []*Window{a, b, c}
	tests := []struct {
		w    *Window
		dir  string
		want *Window
	}{
		{a, "next", b},
		{c, "next", a},
		{a, "prev", c},
		{b, "prev", a},
		{&Window{}, "next", nil}, // not in the list: itself
	}
	for i, tt := range tests {
		want := tt.want
		if want == nil {
			want = tt.w
		}
		if got := neighbour(ws, tt.w, tt.dir); got != want {
			t.Errorf("case %d: wrong window", i)
		}
	}
}

func TestSetNMaster(t *testing.T) {
	o := &Output{tagset: [2]uint32{1, 1}}
	o.pertag[0].nmaster = 1
	add := func(n int, floating bool) {
		for range n {
			o.clients = append(o.clients, &Window{output: o, tags: 1, floating: floating})
		}
	}
	add(3, false)
	add(1, true) // floating windows aren't masters
	add(1, false)
	o.clients[4].tags = 2 // nor are windows on other tags

	steps := []struct {
		arg  string
		want int
	}{
		{"+1", 2},
		{"+1", 3},
		{"+1", 3}, // only three tiling windows
		{"5", 3},
		{"-1", 2},
		{"0", 0},
		{"-1", 0},
		{"3", 3},
	}
	for i, s := range steps {
		o.setNMaster(s.arg)
		if got := o.pertag[0].nmaster; got != s.want {
			t.Errorf("step %d (%s): nmaster = %d, want %d", i, s.arg, got, s.want)
		}
	}

	// Two windows close: the count is kept, but a change starts from the
	// one master on screen.
	o.clients = o.clients[2:]
	if o.pertag[0].nmaster != 3 {
		t.Errorf("nmaster = %d after windows closed, want 3 kept", o.pertag[0].nmaster)
	}
	o.setNMaster("-1")
	if got := o.pertag[0].nmaster; got != 0 {
		t.Errorf("nmaster = %d after -1 with one window, want 0", got)
	}
}

func TestToggleTags(t *testing.T) {
	tests := []struct {
		tags  uint32
		arg   string
		shown uint32
		want  uint32
	}{
		{0b1, "3", 0b1, 0b101},
		{0b101, "3", 0b1, 0b1},
		{0b1, "all", 0b1, allTags},
		{allTags, "all", 0b100, 0b100}, // back to the shown tags
		{allTags, "all", 0b110, 0b110}, // several shown
		{0b10, "all", 0b100, allTags},  // on some tags: all of them
	}
	for _, tt := range tests {
		if got := toggleTags(tt.tags, tt.arg, tt.shown); got != tt.want {
			t.Errorf("toggleTags(%#b, %s, %#b) = %#b, want %#b", tt.tags, tt.arg, tt.shown, got, tt.want)
		}
	}
}

// A seat left without an output, as after a VT switch on one monitor, gets
// one back once an output is there again.
func TestEnsureOutput(t *testing.T) {
	wm := &WM{}
	s := &Seat{wm: wm}
	s.ensureOutput() // no outputs: nothing to pick
	if s.output != nil {
		t.Fatal("output picked with none there")
	}

	a, b := &Output{wm: wm}, &Output{wm: wm}
	wm.outputs = []*Output{a, b}
	w := &Window{output: b}
	s.focused = w
	s.ensureOutput()
	if s.output != b {
		t.Error("seat didn't take its focused window's output")
	}

	s.output, s.focused = nil, nil
	s.ensureOutput()
	if s.output != a {
		t.Error("seat without a focused window didn't take the first output")
	}

	s.output = b
	s.ensureOutput()
	if s.output != b {
		t.Error("seat's output changed while it had one")
	}
}

// Windows set aside when the last output went away (a VT switch) go back to
// it with their tags and order; with another output instead, they go to its
// shown tags.
func TestAdoptParked(t *testing.T) {
	park := func(wm *WM, tags ...uint32) []*Window {
		var ws []*Window
		for _, tg := range tags {
			w := &Window{wm: wm, tags: tg, parkedOn: "eDP-1"}
			ws = append(ws, w)
			wm.windows = append(wm.windows, w)
		}
		wm.parked = slices.Clone(ws)
		return ws
	}

	wm := &WM{}
	ws := park(wm, 1<<2, 1, 0) // tag 3, tag 1, a hidden scratchpad
	o := &Output{wm: wm, tagset: [2]uint32{1, 1}}
	wm.outputs = []*Output{o}
	wm.adoptOrphans() // not named yet: the windows wait
	if ws[0].output != nil {
		t.Fatal("parked window adopted by an unnamed output")
	}
	o.name = "eDP-1"
	wm.adoptOrphans()
	for i, want := range []uint32{1 << 2, 1, 0} {
		if ws[i].output != o || ws[i].tags != want {
			t.Errorf("window %d: output %v, tags %#b; want back on eDP-1 with %#b", i, ws[i].output, ws[i].tags, want)
		}
	}
	if !slices.Equal(o.clients, ws) {
		t.Error("stacking order not kept")
	}
	if len(wm.parked) != 0 {
		t.Error("windows still parked")
	}

	wm = &WM{}
	ws = park(wm, 1<<2)
	other := &Output{wm: wm, name: "HDMI-A-1", tagset: [2]uint32{1 << 4, 1 << 4}}
	wm.outputs = []*Output{other}
	wm.adoptOrphans()
	if ws[0].output != other || ws[0].tags != 1<<4 {
		t.Errorf("on another output: tags %#b, want its shown tags %#b", ws[0].tags, uint32(1<<4))
	}
}
