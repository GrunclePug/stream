package layout

import (
	"slices"
	"testing"
)

func TestArrange(t *testing.T) {
	screen := Rect{0, 0, 1000, 600}
	tests := []struct {
		name string
		p    Params
		area Rect
		n    int
		want []Rect
	}{
		{
			name: "no windows",
			p:    Params{Kind: Tile, NMaster: 1, MFact: 0.5},
			area: screen,
			n:    0,
			want: nil,
		},
		{
			name: "one window fills the area",
			p:    Params{Kind: Tile, NMaster: 1, MFact: 0.5},
			area: screen,
			n:    1,
			want: []Rect{{0, 0, 1000, 600}},
		},
		{
			name: "one window with gap",
			p:    Params{Kind: Tile, NMaster: 1, MFact: 0.5, Gap: 10},
			area: screen,
			n:    1,
			want: []Rect{{10, 10, 980, 580}},
		},
		{
			name: "master and stack",
			p:    Params{Kind: Tile, NMaster: 1, MFact: 0.5},
			area: screen,
			n:    3,
			want: []Rect{{0, 0, 500, 600}, {500, 0, 500, 300}, {500, 300, 500, 300}},
		},
		{
			name: "master and stack with gap",
			p:    Params{Kind: Tile, NMaster: 1, MFact: 0.5, Gap: 10},
			area: screen,
			n:    3,
			want: []Rect{{10, 10, 485, 580}, {505, 10, 485, 285}, {505, 305, 485, 285}},
		},
		{
			name: "leftover height goes to the lower windows",
			p:    Params{Kind: Tile, NMaster: 1, MFact: 0.5},
			area: Rect{0, 0, 1000, 100},
			n:    4,
			want: []Rect{{0, 0, 500, 100}, {500, 0, 500, 33}, {500, 33, 500, 33}, {500, 66, 500, 34}},
		},
		{
			name: "two masters",
			p:    Params{Kind: Tile, NMaster: 2, MFact: 0.6},
			area: screen,
			n:    3,
			want: []Rect{{0, 0, 600, 300}, {0, 300, 600, 300}, {600, 0, 400, 600}},
		},
		{
			name: "more masters than windows fills the width",
			p:    Params{Kind: Tile, NMaster: 3, MFact: 0.5},
			area: screen,
			n:    2,
			want: []Rect{{0, 0, 1000, 300}, {0, 300, 1000, 300}},
		},
		{
			name: "no master: one full-width stack",
			p:    Params{Kind: Tile, NMaster: 0, MFact: 0.5},
			area: screen,
			n:    2,
			want: []Rect{{0, 0, 1000, 300}, {0, 300, 1000, 300}},
		},
		{
			name: "mfact is clamped",
			p:    Params{Kind: Tile, NMaster: 1, MFact: 2},
			area: screen,
			n:    2,
			want: []Rect{{0, 0, 950, 600}, {950, 0, 50, 600}},
		},
		{
			name: "offset area",
			p:    Params{Kind: Tile, NMaster: 1, MFact: 0.5},
			area: Rect{1920, 22, 1000, 600},
			n:    2,
			want: []Rect{{1920, 22, 500, 600}, {2420, 22, 500, 600}},
		},
		{
			name: "monocle ignores the gap",
			p:    Params{Kind: Monocle, Gap: 10},
			area: screen,
			n:    2,
			want: []Rect{{0, 0, 1000, 600}, {0, 0, 1000, 600}},
		},
		{
			name: "floating arranges nothing",
			p:    Params{Kind: Floating},
			area: screen,
			n:    2,
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Arrange(tt.p, tt.area, ones(tt.n))
			if !slices.Equal(got, tt.want) {
				t.Errorf("Arrange() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestArrangeStaysInArea(t *testing.T) {
	area := Rect{100, 50, 1366, 746}
	for n := 1; n <= 12; n++ {
		for nmaster := 0; nmaster <= 4; nmaster++ {
			p := Params{Kind: Tile, NMaster: nmaster, MFact: 0.55, Gap: 8}
			for i, r := range Arrange(p, area, ones(n)) {
				if r.X < area.X || r.Y < area.Y || r.X+r.W > area.X+area.W || r.Y+r.H > area.Y+area.H {
					t.Errorf("n=%d nmaster=%d: rect %d %v leaves %v", n, nmaster, i, r, area)
				}
			}
		}
	}
}

func ones(n int) []float64 {
	w := make([]float64, n)
	for i := range w {
		w[i] = 1
	}
	return w
}

func TestArrangeWeights(t *testing.T) {
	p := Params{Kind: Tile, NMaster: 1, MFact: 0.5, Gap: 0}
	area := Rect{0, 0, 1000, 900}
	tests := []struct {
		weights []float64
		heights []int32 // of the stack windows
	}{
		{[]float64{1, 1, 1, 1}, []int32{300, 300, 300}},
		{[]float64{1, 0, 0, 0}, []int32{300, 300, 300}}, // 0 counts as 1
		{[]float64{1, 2, 1, 0.5}, []int32{514, 257, 129}},
		{[]float64{7, 1, 1}, []int32{450, 450}}, // the master's weight is its own column's
	}
	for _, tt := range tests {
		rects := Arrange(p, area, tt.weights)
		var got []int32
		var sum int32
		for _, r := range rects[1:] {
			got = append(got, r.H)
			sum += r.H
		}
		if !slices.Equal(got, tt.heights) {
			t.Errorf("weights %v: stack heights %v, want %v", tt.weights, got, tt.heights)
		}
		if sum != area.H {
			t.Errorf("weights %v: heights sum to %d, want %d", tt.weights, sum, area.H)
		}
	}
}
