package wm

import (
	"context"
	"slices"

	"github.com/grunclepug/stream/internal/layout"
	"github.com/grunclepug/stream/internal/proto"
)

// snap is how close, in pixels, a moved window's edge must come to the edge
// of the usable area to stick to it. dwm's default.
const snap = 32

// opKind is what an interactive pointer operation does.
type opKind int

const (
	opNone    opKind = iota
	opMove           // drag a floating window
	opResize         // resize a floating window by a corner or edge
	opReorder        // drag a tiled window into another window's slot
	opMFact          // drag the divider between the master and stack columns
	opWeights        // drag the divider between two windows in a column
)

func (k opKind) String() string {
	return [...]string{"none", "move", "resize", "reorder", "mfact", "weights"}[k]
}

// pointerOp is a drag in progress: river reports the pointer's total motion
// since it started, and the button release that ends it.
type pointerOp struct {
	kind     opKind
	w        *Window
	below    *Window     // opWeights: the window under the divider; w is above it
	x0, y0   int32       // pointer position at the start
	dx, dy   int32       // total motion since the start
	start    layout.Rect // w's floating geometry at the start
	edges    uint32      // opResize: the edges that follow the pointer
	released bool
}

// dragRequest is a window asking to be moved or resized, as when a client
// with its own title bar is dragged by it.
type dragRequest struct {
	w      *Window
	resize bool
	edges  uint32
}

// drag runs the move or resize command of a pointer binding pressed over
// target, or over no window, such as the gap between two.
func (s *Seat) drag(target *Window, resize bool) {
	switch {
	case target != nil:
		s.startDrag(target, resize, 0)
	case resize:
		if o := s.wm.outputAt(s.px, s.py); o != nil {
			s.startDivider(o)
		}
	}
}

// startDrag starts what Mod+drag does to w: floating windows move or
// resize; tiled windows are reordered, or the divider nearest the pointer
// moves. Tiled windows never float from a drag.
func (s *Seat) startDrag(w *Window, resize bool, edges uint32) {
	o := w.output
	if s.op.kind != opNone || o == nil || w.fullscreen || !w.visible() {
		return
	}
	op := pointerOp{w: w, x0: s.px, y0: s.py, start: w.float}
	switch {
	case o.isFloating(w) && !resize:
		op.kind = opMove
	case o.isFloating(w):
		op.kind = opResize
		op.edges = edges
		if edges == 0 {
			op.edges = nearestCorner(w.box(), s.px, s.py)
		}
	case !resize:
		op.kind = opReorder
	default:
		s.startDivider(o)
		return
	}
	s.begin(op)
}

// startDivider starts dragging the divider of o's tile layout nearest the
// pointer, which may be over a window or in the gap between two.
func (s *Seat) startDivider(o *Output) {
	if s.op.kind != opNone {
		return
	}
	kind, above, below := o.nearestDivider(s.px, s.py)
	op := pointerOp{kind: kind, w: above, below: below, x0: s.px, y0: s.py}
	switch kind {
	case opNone:
		return
	case opMFact:
		op.w = o.tiled()[0] // any window on o: it ties the drag to o
	}
	s.begin(op)
}

func (s *Seat) begin(op pointerOp) {
	w := op.w
	s.op = op
	s.obj.OpStartPointer()
	s.wm.log.Debug("drag", "op", op.kind.String(), "app_id", op.w.appID, "title", op.w.title)
	if op.kind == opResize {
		w.obj.InformResizeStart()
		w.resizing, w.anchor, w.anchorEdges = true, op.start, op.edges
	}
}

// nearestCorner returns the edges of the corner of r nearest x, y: the
// quadrant the point is in.
func nearestCorner(r layout.Rect, x, y int32) uint32 {
	var edges uint32 = proto.RiverWindowV1EdgesRight
	if x < r.X+r.W/2 {
		edges = proto.RiverWindowV1EdgesLeft
	}
	if y < r.Y+r.H/2 {
		return edges | proto.RiverWindowV1EdgesTop
	}
	return edges | proto.RiverWindowV1EdgesBottom
}

// manageDragRequests starts the drags windows asked for. Only floating windows
// get them; tiled windows stay where the layout puts them.
func (s *Seat) manageDragRequests() {
	for _, r := range s.requests {
		if r.w.output != nil && r.w.output.isFloating(r.w) {
			s.startDrag(r.w, r.resize, r.edges)
		}
	}
	s.requests = s.requests[:0]
}

// manageOp applies the pointer motion since the last manage sequence, and
// ends the operation once the button is released.
func (s *Seat) manageOp() {
	op := &s.op
	if op.kind == opNone {
		return
	}
	w := op.w
	if w.output == nil || !w.visible() || (op.below != nil && (op.below.output != w.output || !op.below.visible())) {
		s.endOp()
		return
	}
	o := w.output
	x, y := op.x0+op.dx, op.y0+op.dy

	switch op.kind {
	case opMove:
		f := op.start
		f.X, f.Y = f.X+op.dx, f.Y+op.dy
		within := o.usable
		if m := s.wm.outputAt(x, y); m != nil {
			within = m.usable
		}
		w.float = snapTo(f, within, s.wm.cfg.BorderWidth)
	case opResize:
		w.float = resized(op.start, op.edges, op.dx, op.dy, w)
	case opReorder:
		for _, t := range o.tiled() {
			if t != w && t.tiled && t.slot.Contains(x, y) {
				o.moveTo(w, t)
				break
			}
		}
	case opMFact:
		inner := o.usable.Inset(o.gap)
		if inner.W > o.gap {
			f := float64(x-inner.X-o.gap/2) / float64(inner.W-o.gap)
			o.lay().mfact = min(max(f, 0.05), 0.95)
		}
	case opWeights:
		a, b := w, op.below
		top, bottom := a.slot.Y, b.slot.Y+b.slot.H
		if total := bottom - top - o.gap; total > 0 {
			share := float64(y-top-o.gap/2) / float64(total)
			share = min(max(share, 0.05), 0.95)
			sum := a.weightOr1() + b.weightOr1()
			a.weight, b.weight = sum*share, sum*(1-share)
		}
	}

	if op.released {
		if op.kind == opMove {
			// A window dropped on another monitor moves there, onto its
			// shown tags, and the focus follows it, as in dwm.
			c := w.box()
			if m := s.wm.outputAt(c.X+c.W/2, c.Y+c.H/2); m != nil && m != o {
				s.wm.sendToOutput(w, m, false)
				s.focus(w)
			}
		}
		s.endOp()
	}
}

func (s *Seat) endOp() {
	s.wm.log.Debug("drag end", "op", s.op.kind.String(), "dx", s.op.dx, "dy", s.op.dy)
	if s.op.kind == opResize {
		s.op.w.obj.InformResizeEnd()
		s.op.w.resizing = false
	}
	s.obj.OpEnd()
	s.op = pointerOp{}
}

// snapTo moves the content geometry f so its outer edges stick to the edges
// of within when they are closer than snap, as dwm's movemouse does.
func snapTo(f, within layout.Rect, bw int32) layout.Rect {
	outer := layout.Rect{X: f.X - bw, Y: f.Y - bw, W: f.W + 2*bw, H: f.H + 2*bw}
	switch {
	case abs(within.X-outer.X) < snap:
		outer.X = within.X
	case abs(within.X+within.W-(outer.X+outer.W)) < snap:
		outer.X = within.X + within.W - outer.W
	}
	switch {
	case abs(within.Y-outer.Y) < snap:
		outer.Y = within.Y
	case abs(within.Y+within.H-(outer.Y+outer.H)) < snap:
		outer.Y = within.Y + within.H - outer.H
	}
	f.X, f.Y = outer.X+bw, outer.Y+bw
	return f
}

func abs(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

// resized returns start with the given edges moved by dx, dy, kept within
// the window's size hints.
func resized(start layout.Rect, edges uint32, dx, dy int32, w *Window) layout.Rect {
	r := start
	if edges&proto.RiverWindowV1EdgesLeft != 0 {
		r.W -= dx
	} else if edges&proto.RiverWindowV1EdgesRight != 0 {
		r.W += dx
	}
	if edges&proto.RiverWindowV1EdgesTop != 0 {
		r.H -= dy
	} else if edges&proto.RiverWindowV1EdgesBottom != 0 {
		r.H += dy
	}
	r.W = clampSize(r.W, w.minW, w.maxW)
	r.H = clampSize(r.H, w.minH, w.maxH)
	// The opposite edges stay put: render places the window from them once
	// it has taken its new size.
	if edges&proto.RiverWindowV1EdgesLeft != 0 {
		r.X = start.X + start.W - r.W
	}
	if edges&proto.RiverWindowV1EdgesTop != 0 {
		r.Y = start.Y + start.H - r.H
	}
	return r
}

func clampSize(v, lo, hi int32) int32 {
	v = max(v, lo, 1)
	if hi > 0 {
		v = min(v, hi)
	}
	return v
}

// moveTo moves w to t's place in the layout order; the windows between
// shift along.
func (o *Output) moveTo(w, t *Window) {
	from, to := slices.Index(o.clients, w), slices.Index(o.clients, t)
	if from < 0 || to < 0 {
		return
	}
	o.clients = slices.Insert(slices.Delete(o.clients, from, from+1), to, w)
}

// nearestDivider finds the divider of the tile layout nearest x, y: the one
// between the master and stack columns, or one between two windows in the
// column under the pointer.
func (o *Output) nearestDivider(x, y int32) (kind opKind, above, below *Window) {
	if o.params().Kind != layout.Tile {
		return opNone, nil, nil
	}
	tiled := o.tiled()
	nmaster := min(o.lay().nmaster, len(tiled))
	best := int32(-1)
	consider := func(dist int32, k opKind, a, b *Window) {
		if best < 0 || dist < best {
			best, kind, above, below = dist, k, a, b
		}
	}
	if nmaster > 0 && nmaster < len(tiled) {
		m := tiled[0].slot
		consider(abs(x-(m.X+m.W+o.gap/2)), opMFact, nil, nil)
	}
	for _, col := range [][]*Window{tiled[:nmaster], tiled[nmaster:]} {
		if len(col) < 2 {
			continue
		}
		c := col[0].slot
		if x < c.X-o.gap || x >= c.X+c.W+o.gap {
			continue
		}
		for i := range len(col) - 1 {
			a := col[i].slot
			consider(abs(y-(a.Y+a.H+o.gap/2)), opWeights, col[i], col[i+1])
		}
	}
	return kind, above, below
}

func (s *Seat) HandleRiverSeatV1OpDelta(ctx context.Context, dx, dy int32) {
	s.op.dx, s.op.dy = dx, dy
}

func (s *Seat) HandleRiverSeatV1OpRelease(ctx context.Context) {
	s.op.released = true
}

func (w *Window) HandleRiverWindowV1PointerMoveRequested(ctx context.Context, seat proto.RiverSeatV1) {
	s := seat.UserData().(*Seat)
	s.requests = append(s.requests, dragRequest{w: w})
}

func (w *Window) HandleRiverWindowV1PointerResizeRequested(ctx context.Context, seat proto.RiverSeatV1, edges uint32) {
	s := seat.UserData().(*Seat)
	s.requests = append(s.requests, dragRequest{w: w, resize: true, edges: edges})
}
