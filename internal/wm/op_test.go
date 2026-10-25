package wm

import (
	"slices"
	"testing"

	"github.com/grunclepug/stream/internal/layout"
	"github.com/grunclepug/stream/internal/proto"
)

const (
	left   = proto.RiverWindowV1EdgesLeft
	right  = proto.RiverWindowV1EdgesRight
	top    = proto.RiverWindowV1EdgesTop
	bottom = proto.RiverWindowV1EdgesBottom
)

func TestNearestCorner(t *testing.T) {
	r := layout.Rect{X: 100, Y: 100, W: 200, H: 100}
	tests := []struct {
		x, y int32
		want uint32
	}{
		{110, 110, top | left},
		{290, 110, top | right},
		{110, 190, bottom | left},
		{290, 190, bottom | right},
	}
	for _, tt := range tests {
		if got := nearestCorner(r, tt.x, tt.y); got != tt.want {
			t.Errorf("nearestCorner(%d, %d) = %#b, want %#b", tt.x, tt.y, got, tt.want)
		}
	}
}

func TestResized(t *testing.T) {
	start := layout.Rect{X: 100, Y: 100, W: 200, H: 100}
	w := &Window{minW: 50, minH: 20}
	tests := []struct {
		edges  uint32
		dx, dy int32
		want   layout.Rect
	}{
		{bottom | right, 30, 10, layout.Rect{X: 100, Y: 100, W: 230, H: 110}},
		{top | left, 30, 10, layout.Rect{X: 130, Y: 110, W: 170, H: 90}}, // bottom-right stays
		{top | left, -30, -10, layout.Rect{X: 70, Y: 90, W: 230, H: 110}},
		{bottom | right, -500, -500, layout.Rect{X: 100, Y: 100, W: 50, H: 20}}, // size hints
		{top | left, 500, 500, layout.Rect{X: 250, Y: 180, W: 50, H: 20}},
		{right, 30, 10, layout.Rect{X: 100, Y: 100, W: 230, H: 100}}, // an edge, not a corner
	}
	for _, tt := range tests {
		if got := resized(start, tt.edges, tt.dx, tt.dy, w); got != tt.want {
			t.Errorf("resized(%#b, %d, %d) = %v, want %v", tt.edges, tt.dx, tt.dy, got, tt.want)
		}
	}
}

func TestSnapTo(t *testing.T) {
	within := layout.Rect{X: 0, Y: 30, W: 1000, H: 600}
	tests := []struct {
		f, want layout.Rect
	}{
		{layout.Rect{X: 20, Y: 50, W: 100, H: 100}, layout.Rect{X: 1, Y: 31, W: 100, H: 100}},      // top-left
		{layout.Rect{X: 880, Y: 500, W: 100, H: 100}, layout.Rect{X: 899, Y: 529, W: 100, H: 100}}, // bottom-right
		{layout.Rect{X: 400, Y: 300, W: 100, H: 100}, layout.Rect{X: 400, Y: 300, W: 100, H: 100}}, // too far
	}
	for _, tt := range tests {
		if got := snapTo(tt.f, within, 1); got != tt.want {
			t.Errorf("snapTo(%v) = %v, want %v", tt.f, got, tt.want)
		}
	}
}

func TestMoveTo(t *testing.T) {
	a, b, c, d := &Window{title: "a"}, &Window{title: "b"}, &Window{title: "c"}, &Window{title: "d"}
	tests := []struct {
		w, t *Window
		want string
	}{
		{d, a, "dabc"}, // the third window dragged to the top
		{a, c, "bcad"},
		{b, c, "acbd"},
		{c, b, "acbd"},
	}
	for _, tt := range tests {
		o := &Output{clients: []*Window{a, b, c, d}}
		o.moveTo(tt.w, tt.t)
		var got string
		for _, w := range o.clients {
			got += w.title
		}
		if got != tt.want {
			t.Errorf("move %s to %s: %s, want %s", tt.w.title, tt.t.title, got, tt.want)
		}
	}
}

// TestNearestDivider lays out a master and a stack of two, the way arrange
// does, and checks which divider a press picks.
func TestNearestDivider(t *testing.T) {
	o := &Output{tagset: [2]uint32{1, 1}, gap: 10, usable: layout.Rect{W: 1000, H: 600}}
	o.pertag[0] = tagLayout{nmaster: 1, mfact: 0.5}
	var ws []*Window
	for range 3 {
		w := &Window{output: o, tags: 1}
		ws = append(ws, w)
		o.clients = append(o.clients, w)
	}
	for i, r := range layout.Arrange(o.params(), o.usable, []float64{1, 1, 1}) {
		ws[i].slot, ws[i].tiled = r, true
	}
	// master 10..495, stack 505..990; stack windows 10..300 and 310..590
	tests := []struct {
		x, y   int32
		kind   opKind
		above  *Window
		bottom *Window
	}{
		{480, 100, opMFact, nil, nil},
		{700, 290, opWeights, ws[1], ws[2]},
		{700, 50, opMFact, nil, nil},  // nearer the master divider than the stack's
		{100, 300, opMFact, nil, nil}, // the master column has one window
	}
	for _, tt := range tests {
		kind, a, b := o.nearestDivider(tt.x, tt.y)
		if kind != tt.kind || a != tt.above || b != tt.bottom {
			t.Errorf("nearestDivider(%d, %d) = %d, want %d", tt.x, tt.y, kind, tt.kind)
		}
	}
	if !slices.Equal(o.tiled(), ws) {
		t.Fatal("not all tiled")
	}
}
