package wm

import (
	"context"
	"math/bits"
	"slices"

	"github.com/grunclepug/stream/internal/command"
	"github.com/grunclepug/stream/internal/layout"
	"github.com/grunclepug/stream/internal/proto"
)

// Output is a monitor: a logical output in river's layout, with the windows
// that live on it.
type Output struct {
	proto.RiverOutputV1Stub
	proto.RiverLayerShellOutputV1Stub
	proto.WlOutputStub

	wm       *WM
	obj      proto.RiverOutputV1
	layer    proto.RiverLayerShellOutputV1
	wlOutput proto.WlOutput

	name string
	area layout.Rect // the whole output
	// usable is the area left after layer surfaces' exclusive zones, such
	// as a bar's. Until river reports it, it's the whole output.
	usable    layout.Rect
	usableSet bool

	// clients is the layout order, master first (dwm's clients list).
	clients []*Window
	// focusOrder is the focus history, most recent first (dwm's stack).
	focusOrder []*Window

	// tagset holds the tags shown and the ones shown before, so view prev
	// can swap back; seltags picks the current one. As in dwm.
	tagset  [2]uint32
	seltags int
	// pertag is each tag's layout state. With several tags shown, the
	// lowest one's applies.
	pertag [command.NumTags]tagLayout
	gap    int32

	removed bool
}

// tagLayout is one tag's layout state: dwm's lt[2] and sellt (the layout
// and the one before it), nmaster and mfact.
type tagLayout struct {
	kinds   [2]layout.Kind
	sel     int
	nmaster int
	mfact   float64
}

// allTags is the mask with every tag set.
const allTags = 1<<command.NumTags - 1

func newOutput(wm *WM, obj proto.RiverOutputV1) *Output {
	o := &Output{wm: wm, obj: obj, tagset: [2]uint32{1, 1}, gap: wm.cfg.Gap}
	for i := range o.pertag {
		o.pertag[i] = tagLayout{
			kinds:   [2]layout.Kind{wm.cfg.Layout, wm.cfg.Layout},
			nmaster: wm.cfg.NMaster,
			mfact:   wm.cfg.MFact,
		}
	}
	obj.SetUserData(o)
	o.layer = wm.layerShell.GetOutput(obj)
	o.layer.SetUserData(o)
	return o
}

func (o *Output) destroy() {
	o.layer.Destroy()
	if o.wlOutput.IsSet() {
		o.wlOutput.Release()
	}
	o.obj.Destroy()
}

// attach makes w the master of this output's stack, on the shown tags, as
// dwm does with new windows.
func (o *Output) attach(w *Window) {
	w.output, w.tags = o, o.tags()
	o.clients = slices.Insert(o.clients, 0, w)
	o.focusOrder = append(o.focusOrder, w)
}

// attachBottom adds w at the end of the stack, for windows that move here
// from an output that went away.
func (o *Output) attachBottom(w *Window) {
	w.output, w.tags = o, o.tags()
	o.clients = append(o.clients, w)
	o.focusOrder = append(o.focusOrder, w)
}

func (o *Output) detach(w *Window) {
	o.clients = slices.DeleteFunc(o.clients, func(c *Window) bool { return c == w })
	o.focusOrder = slices.DeleteFunc(o.focusOrder, func(c *Window) bool { return c == w })
	w.output = nil
}

// touch moves w to the front of the focus history.
func (o *Output) touch(w *Window) {
	o.focusOrder = slices.DeleteFunc(o.focusOrder, func(c *Window) bool { return c == w })
	o.focusOrder = slices.Insert(o.focusOrder, 0, w)
}

// tags returns the tags shown.
func (o *Output) tags() uint32 {
	return o.tagset[o.seltags]
}

// lay returns the layout state in effect: the lowest shown tag's.
func (o *Output) lay() *tagLayout {
	return &o.pertag[bits.TrailingZeros32(o.tags())]
}

func (o *Output) params() layout.Params {
	t := o.lay()
	return layout.Params{Kind: t.kinds[t.sel], NMaster: t.nmaster, MFact: t.mfact, Gap: o.gap}
}

// visible returns the windows on the shown tags, in layout order.
func (o *Output) visible() []*Window {
	var vis []*Window
	for _, w := range o.clients {
		if w.visible() {
			vis = append(vis, w)
		}
	}
	return vis
}

// tiled returns the visible windows the layout places, in layout order:
// dwm's nexttiled walk.
func (o *Output) tiled() []*Window {
	var tiled []*Window
	for _, w := range o.clients {
		if w.visible() && !w.fullscreen && !o.isFloating(w) {
			tiled = append(tiled, w)
		}
	}
	return tiled
}

// setNMaster sets the shown tags' master count, from 0 to the tiling windows
// shown. A relative change starts from the masters on screen.
func (o *Output) setNMaster(arg string) {
	shown := 0
	for _, w := range o.clients {
		if w.visible() && !w.floating {
			shown++
		}
	}
	t := o.lay()
	n := parseInt(arg)
	if command.Relative(arg) {
		n += min(t.nmaster, shown)
	}
	t.nmaster = min(max(n, 0), shown)
}

// lastFocused is the window to focus when focus lands on this output
// without a window being chosen: the most recently focused visible one.
func (o *Output) lastFocused() *Window {
	for _, w := range o.focusOrder {
		if w.visible() {
			return w
		}
	}
	return nil
}

// isFloating reports whether w is placed by its own geometry rather than by
// the layout.
func (o *Output) isFloating(w *Window) bool {
	return w.floating || o.params().Kind == layout.Floating
}

// arrange runs the layout and proposes the resulting sizes. Positions are
// set in render, once river reports the sizes the windows took.
func (o *Output) arrange() {
	bw := o.wm.cfg.BorderWidth
	var tiled []*Window
	for _, w := range o.clients {
		wasTiled := w.tiled
		w.tiled = false
		switch {
		case !w.visible(), w.fullscreen:
		case o.isFloating(w):
			if wasTiled {
				// A tiled window that starts floating, by itself or with
				// the floating layout, stays where it was, as in dwm.
				w.float = w.slot.Inset(bw)
			}
			if w.float.W == 0 {
				w.propose(0, 0) // let it choose; it's centered in render
			} else {
				w.propose(w.float.W, w.float.H)
			}
			w.setTiledEdges(proto.RiverWindowV1EdgesNone)
		default:
			tiled = append(tiled, w)
		}
	}

	weights := make([]float64, len(tiled))
	for i, w := range tiled {
		weights[i] = w.weightOr1()
	}
	for i, r := range layout.Arrange(o.params(), o.usable, weights) {
		w := tiled[i]
		w.tiled, w.slot = true, r
		content := r.Inset(bw)
		w.propose(max(content.W, 1), max(content.H, 1))
		w.setTiledEdges(allEdges)
	}
}

// render positions, borders and stacks this output's windows. Floating
// windows go above tiled ones, in the order they were last raised, and
// fullscreen windows go above everything.
func (o *Output) render() {
	bw := o.wm.cfg.BorderWidth
	var tiled, floating, fullscreen []*Window
	for _, w := range o.clients {
		w.setHidden(!w.visible())
		switch {
		case w.hidden:
		case w.fullscreen:
			fullscreen = append(fullscreen, w)
		case w.tiled:
			tiled = append(tiled, w)
		default:
			floating = append(floating, w)
		}
	}

	for _, w := range tiled {
		content := w.slot.Inset(bw)
		w.node.SetPosition(content.X, content.Y)
		// A window that won't shrink to its slot is cut off at the slot's
		// edge, so it can't cover its neighbours.
		if w.width > content.W || w.height > content.H {
			w.obj.SetContentClipBox(0, 0, content.W, content.H)
		} else {
			w.obj.SetContentClipBox(0, 0, 0, 0)
		}
		o.setBorder(w, bw)
	}
	for _, w := range floating {
		o.place(w)
		w.node.SetPosition(w.float.X, w.float.Y)
		w.obj.SetContentClipBox(0, 0, 0, 0)
		o.setBorder(w, bw)
	}

	// Stack bottom to top. In monocle the focused window covers the rest.
	if o.params().Kind == layout.Monocle {
		if f := o.lastFocused(); f != nil && f.tiled {
			tiled = slices.DeleteFunc(tiled, func(w *Window) bool { return w == f })
			tiled = append(tiled, f)
		}
	}
	for _, w := range tiled {
		w.node.PlaceTop()
	}
	for _, w := range o.wm.raised {
		if w.output == o && slices.Contains(floating, w) {
			w.node.PlaceTop()
		}
	}
	for _, w := range fullscreen {
		w.node.PlaceTop()
	}
}

// place gives a floating window that has no position yet one: centred over
// its parent if it has one here, otherwise centred in the usable area.
func (o *Output) place(w *Window) {
	if w.width <= 0 || w.height <= 0 {
		return
	}
	if w.float.W == 0 {
		within := o.usable
		if p := w.parent; p != nil && p.output == o {
			within = p.box()
		}
		w.float = layout.Rect{
			X: within.X + (within.W-w.width)/2,
			Y: within.Y + (within.H-w.height)/2,
		}
	}
	w.float.W, w.float.H = w.width, w.height
	if w.resizing {
		if w.anchorEdges&proto.RiverWindowV1EdgesLeft != 0 {
			w.float.X = w.anchor.X + w.anchor.W - w.width
		}
		if w.anchorEdges&proto.RiverWindowV1EdgesTop != 0 {
			w.float.Y = w.anchor.Y + w.anchor.H - w.height
		}
	}
}

func (o *Output) setBorder(w *Window, bw int32) {
	c := o.wm.cfg.BorderUnfocused
	if o.wm.focusedBySeat(w) {
		c = o.wm.cfg.BorderFocused
	}
	r, g, b, a := c.RGBA32()
	w.obj.SetBorders(allEdges, bw, r, g, b, a)
}

func (o *Output) HandleRiverOutputV1Removed(ctx context.Context) {
	o.removed = true
}

func (o *Output) HandleRiverOutputV1Position(ctx context.Context, x, y int32) {
	o.area.X, o.area.Y = x, y
	if !o.usableSet {
		o.usable = o.area
	}
}

func (o *Output) HandleRiverOutputV1Dimensions(ctx context.Context, width, height int32) {
	o.area.W, o.area.H = width, height
	if !o.usableSet {
		o.usable = o.area
	}
}

// HandleRiverOutputV1WlOutput binds the matching wl_output, only to learn
// the output's name (HDMI-A-1, eDP-1, ...).
func (o *Output) HandleRiverOutputV1WlOutput(ctx context.Context, name uint32) {
	version, ok := o.wm.wlOutputs[name]
	if !ok || version < wlOutputVersion {
		return
	}
	o.wlOutput = proto.As[proto.WlOutput](o.wm.registry.Bind(name, proto.WlOutputName, wlOutputVersion))
	o.wlOutput.SetUserData(o)
}

func (o *Output) HandleWlOutputName(ctx context.Context, name string) {
	o.name = name
	o.restoreOutput()
	// Windows waiting for this output's name are placed in the next manage
	// sequence.
	o.wm.manager.ManageDirty()
	o.wm.log.Debug("output", "name", name, "x", o.area.X, "y", o.area.Y, "width", o.area.W, "height", o.area.H)
}

func (o *Output) HandleRiverLayerShellOutputV1NonExclusiveArea(ctx context.Context, x, y, width, height int32) {
	o.usable = layout.Rect{X: x, Y: y, W: width, H: height}
	o.usableSet = true
}
