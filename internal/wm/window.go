package wm

import (
	"context"

	"github.com/grunclepug/stream/internal/layout"
	"github.com/grunclepug/stream/internal/proto"
	"hazelnut.eclair.cafe/wlcl"
)

const allEdges = proto.RiverWindowV1EdgesTop | proto.RiverWindowV1EdgesBottom |
	proto.RiverWindowV1EdgesLeft | proto.RiverWindowV1EdgesRight

// Window is a river window: an xdg_toplevel or an Xwayland window.
type Window struct {
	proto.RiverWindowV1Stub

	wm   *WM
	obj  proto.RiverWindowV1
	node proto.RiverNodeV1

	id     string // river's identifier: unique and never reused
	appID  string
	title  string
	parent *Window

	// Size hints; 0 means no preference.
	minW, minH, maxW, maxH int32

	// width and height are the content dimensions river last reported.
	width, height int32

	output   *Output
	parkedOn string // the output it waits for, when its output went away
	tags     uint32 // a bitmask: bit 0 is tag 1; 0 for a hidden scratchpad
	scratch  string // the scratchpad this window is, if any
	floating bool
	// float is the content geometry of a floating window, or of any window
	// in the floating layout. A zero width means not placed yet.
	float layout.Rect
	// slot is the outer box the layout gave a tiled window this round.
	slot   layout.Rect
	tiled  bool
	hidden bool // hidden on river's side, because its tags aren't shown
	// weight is the window's share of its column's height relative to its
	// neighbours (dwm's cfacts); 0 means the default, 1.
	weight float64

	// While a resize drags its left or top edge, render keeps the opposite
	// edges of anchor in place, whatever size the window takes.
	resizing    bool
	anchor      layout.Rect
	anchorEdges uint32

	fullscreen        bool
	fullscreenRequest bool
	exitRequest       bool

	// What swm last told river, so unchanged state isn't resent.
	proposedW, proposedH int32
	proposed             bool
	tiledEdges           uint32
	decorationHint       uint32
	decorationPending    bool

	new    bool // not seen by a manage sequence yet
	closed bool
}

func newWindow(wm *WM, obj proto.RiverWindowV1) *Window {
	w := &Window{wm: wm, obj: obj, node: obj.GetNode(), new: true}
	obj.SetUserData(w)
	return w
}

func (w *Window) destroy() {
	w.node.Destroy()
	w.obj.Destroy()
}

// fixedSize reports whether the window can't be resized, which makes dwm
// float it.
func (w *Window) fixedSize() bool {
	return w.maxW > 0 && w.maxH > 0 && w.minW == w.maxW && w.minH == w.maxH
}

// manageRequests applies what the window asked for since the last manage
// sequence.
func (w *Window) manageRequests() {
	if w.decorationPending {
		w.decorationPending = false
		if w.decorationHint != proto.RiverWindowV1DecorationHintOnlySupportsCsd {
			w.obj.UseSsd()
		}
	}
	if w.output == nil {
		return
	}
	if w.fullscreenRequest {
		w.setFullscreen(true)
	}
	if w.exitRequest {
		w.setFullscreen(false)
	}
	w.fullscreenRequest, w.exitRequest = false, false
}

// setFullscreen makes the window cover its output, or puts it back in the
// layout.
func (w *Window) setFullscreen(on bool) {
	if on == w.fullscreen {
		return
	}
	w.fullscreen = on
	if on {
		w.obj.Fullscreen(w.output.obj)
		w.obj.InformFullscreen()
		return
	}
	w.obj.ExitFullscreen()
	w.obj.InformNotFullscreen()
	w.proposed = false // the next arrange must propose and position again
}

func (w *Window) weightOr1() float64 {
	if w.weight <= 0 {
		return 1
	}
	return w.weight
}

// visible reports whether the window is on one of its output's shown tags.
func (w *Window) visible() bool {
	return w.output != nil && w.tags&w.output.tags() != 0
}

func (w *Window) setHidden(hidden bool) {
	if hidden == w.hidden {
		return
	}
	w.hidden = hidden
	if hidden {
		w.obj.Hide()
	} else {
		w.obj.Show()
	}
}

// propose asks the window to take content dimensions width x height, unless
// that's what it was last asked. Zero lets the window choose.
func (w *Window) propose(width, height int32) {
	if w.proposed && w.proposedW == width && w.proposedH == height {
		return
	}
	w.obj.ProposeDimensions(width, height)
	w.proposedW, w.proposedH, w.proposed = width, height, true
}

// box is the window's content geometry as it is on screen.
func (w *Window) box() layout.Rect {
	if w.tiled {
		return w.slot.Inset(w.wm.cfg.BorderWidth)
	}
	return w.float
}

// setTiledEdges tells the window which of its edges touch the tiled layout.
func (w *Window) setTiledEdges(edges uint32) {
	if w.tiledEdges != edges {
		w.obj.SetTiled(edges)
		w.tiledEdges = edges
	}
}

func (w *Window) HandleRiverWindowV1Closed(ctx context.Context) {
	w.closed = true
}

func (w *Window) HandleRiverWindowV1Identifier(ctx context.Context, identifier string) {
	w.id = identifier
}

func (w *Window) HandleRiverWindowV1AppId(ctx context.Context, appID wlcl.NullString) {
	w.appID = appID.String
}

func (w *Window) HandleRiverWindowV1Title(ctx context.Context, title wlcl.NullString) {
	w.title = title.String
}

func (w *Window) HandleRiverWindowV1Parent(ctx context.Context, parent proto.RiverWindowV1) {
	w.parent = nil
	if parent.IsSet() {
		w.parent = parent.UserData().(*Window)
	}
}

func (w *Window) HandleRiverWindowV1DimensionsHint(ctx context.Context, minW, minH, maxW, maxH int32) {
	w.minW, w.minH, w.maxW, w.maxH = minW, minH, maxW, maxH
}

func (w *Window) HandleRiverWindowV1Dimensions(ctx context.Context, width, height int32) {
	w.width, w.height = width, height
}

// HandleRiverWindowV1DecorationHint asks every window that can do it for
// server-side decorations: swm draws borders and nothing else, so windows
// lose their title bars.
func (w *Window) HandleRiverWindowV1DecorationHint(ctx context.Context, hint uint32) {
	w.decorationHint, w.decorationPending = hint, true
}

func (w *Window) HandleRiverWindowV1FullscreenRequested(ctx context.Context, output proto.RiverOutputV1) {
	w.fullscreenRequest, w.exitRequest = true, false
}

func (w *Window) HandleRiverWindowV1ExitFullscreenRequested(ctx context.Context) {
	w.exitRequest, w.fullscreenRequest = true, false
}
