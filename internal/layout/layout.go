// Package layout arranges tiled windows. Every layout is a pure function from
// an area and a window count to one rectangle per window, in stack order.
// Rectangles are outer boxes: they include the window's border.
package layout

// Rect is a rectangle in the compositor's logical coordinate space.
type Rect struct {
	X, Y, W, H int32
}

// Inset shrinks r by d on every side.
func (r Rect) Inset(d int32) Rect {
	return Rect{r.X + d, r.Y + d, max(r.W-2*d, 0), max(r.H-2*d, 0)}
}

// Contains reports whether the point x, y lies inside r.
func (r Rect) Contains(x, y int32) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// Kind names a layout.
type Kind int

const (
	Tile Kind = iota
	Monocle
	Floating
)

// KindNamed returns the layout with the given name.
func KindNamed(name string) (Kind, bool) {
	for _, k := range []Kind{Tile, Monocle, Floating} {
		if k.String() == name {
			return k, true
		}
	}
	return 0, false
}

// String returns the name used in the config and in streamctl.
func (k Kind) String() string {
	switch k {
	case Tile:
		return "tile"
	case Monocle:
		return "monocle"
	case Floating:
		return "floating"
	}
	return "unknown"
}

// Params are the per-tag settings a layout uses.
type Params struct {
	Kind    Kind
	NMaster int     // windows in the master column
	MFact   float64 // share of the width the master column takes, 0.05 to 0.95
	Gap     int32   // space between windows and at the area's edges
}

// Arrange returns one rectangle per tiled window in area, one for each
// weight. A window's weight is its share of its column's height, relative to
// the other windows in the column (dwm's cfacts); weights of 0 count as 1.
// The floating layout arranges nothing and returns nil.
func Arrange(p Params, area Rect, weights []float64) []Rect {
	n := len(weights)
	if n == 0 {
		return nil
	}
	switch p.Kind {
	case Tile:
		return tile(area, weights, p.NMaster, p.MFact, p.Gap)
	case Monocle:
		return monocle(area, n)
	}
	return nil
}

// tile is dwm's tile layout with fullgaps: a master column on the left and a
// stack on the right, gap between windows and around the edges.
func tile(area Rect, weights []float64, nmaster int, mfact float64, gap int32) []Rect {
	n := len(weights)
	inner := area.Inset(gap)
	nmaster = min(max(nmaster, 0), n)
	nstack := n - nmaster

	master, stack := inner, inner
	if nmaster > 0 && nstack > 0 {
		mw := MasterWidth(inner.W, mfact, gap)
		master.W = mw
		stack.X = inner.X + mw + gap
		stack.W = max(inner.W-mw-gap, 0)
	}

	rects := make([]Rect, 0, n)
	rects = append(rects, column(master, weights[:nmaster], gap)...)
	rects = append(rects, column(stack, weights[nmaster:], gap)...)
	return rects
}

// MasterWidth is the width of the master column in an area width wide
// (inside the outer gap), mfact clamped to 0.05 to 0.95.
func MasterWidth(width int32, mfact float64, gap int32) int32 {
	mfact = min(max(mfact, 0.05), 0.95)
	return int32(float64(width-gap) * mfact)
}

// weightUnits is how finely weights divide a column. The split itself is
// integer arithmetic, so equal weights always give dwm's pixel-exact
// result, with leftover pixels going to the lower windows.
const weightUnits = 1000

// column splits area vertically into one rectangle per weight, separated by
// gap.
func column(area Rect, weights []float64, gap int32) []Rect {
	k := len(weights)
	units := make([]int64, k)
	var total int64
	for i, w := range weights {
		if w <= 0 {
			w = 1
		}
		units[i] = max(int64(w*weightUnits+0.5), 1)
		total += units[i]
	}
	rects := make([]Rect, 0, k)
	y := area.Y
	left := int64(max(area.H-gap*int32(k-1), 0))
	for i := range k {
		h := left * units[i] / total
		rects = append(rects, Rect{area.X, y, area.W, int32(h)})
		y += int32(h) + gap
		left -= h
		total -= units[i]
	}
	return rects
}

// monocle gives every window the whole area. Like dwm with fullgaps, it
// ignores the gap.
func monocle(area Rect, n int) []Rect {
	rects := make([]Rect, n)
	for i := range rects {
		rects[i] = area
	}
	return rects
}
