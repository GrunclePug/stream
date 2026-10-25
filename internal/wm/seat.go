package wm

import (
	"context"
	"slices"
	"time"

	"github.com/grunclepug/stream/internal/config"
	"github.com/grunclepug/stream/internal/proto"
)

// layerFocus is which kind of keyboard focus a layer surface (a launcher, a
// bar) has on a seat.
type layerFocus int

const (
	layerFocusNone layerFocus = iota
	layerFocusExclusive
	layerFocusNonExclusive
)

// Seat is a group of input devices with one keyboard focus.
type Seat struct {
	proto.RiverSeatV1Stub
	proto.RiverLayerShellSeatV1Stub

	wm    *WM
	obj   proto.RiverSeatV1
	layer proto.RiverLayerShellSeatV1

	focused *Window
	output  *Output // the focused monitor, dwm's selmon

	// sent is the window river was last asked to focus. When river's focus
	// may differ (a layer surface took it), resend is set.
	sent   *Window
	resend bool

	layerFocus layerFocus

	// bindings are this seat's river binding objects for every mode, made
	// from config generation bindingsGen.
	bindings    []*binding
	bindingsGen int
	pressed     []pressed

	op       pointerOp
	requests []dragRequest

	// Pointer state reported since the last manage sequence.
	hovered    *Window
	interacted *Window
	px, py     int32
	moved      bool
	// lastHovered is the window the pointer was over when it last moved.
	lastHovered *Window

	removed bool
}

func newSeat(wm *WM, obj proto.RiverSeatV1) *Seat {
	s := &Seat{wm: wm, obj: obj, resend: true}
	obj.SetUserData(s)
	s.layer = wm.layerShell.GetSeat(obj)
	s.layer.SetUserData(s)
	if len(wm.outputs) > 0 {
		s.output = wm.outputs[0]
	}
	return s
}

func (s *Seat) destroyIfRemoved() bool {
	if !s.removed {
		return false
	}
	for _, b := range s.bindings {
		b.destroy()
	}
	s.layer.Destroy()
	s.obj.Destroy()
	return true
}

// forget drops every reference to a window that closed, and moves focus on
// to the window focused before it, as dwm does.
func (s *Seat) forget(w *Window) {
	if s.hovered == w {
		s.hovered = nil
	}
	if s.lastHovered == w {
		s.lastHovered = nil
	}
	if s.interacted == w {
		s.interacted = nil
	}
	for i := range s.pressed {
		if s.pressed[i].target == w {
			s.pressed[i].target = nil
		}
	}
	s.requests = slices.DeleteFunc(s.requests, func(r dragRequest) bool { return r.w == w })
	if s.op.w == w || s.op.below == w {
		s.endOp()
	}
	if s.sent == w {
		s.sent = nil
	}
	if s.focused == w {
		s.focused = nil
		if s.output != nil {
			s.focusRaise(s.output.lastFocused())
		}
	}
}

// focus gives w the keyboard focus and makes its output the focused one. A
// nil w leaves the seat with no window focused.
func (s *Seat) focus(w *Window) {
	s.focused = w
	if w == nil || w.output == nil {
		return
	}
	s.output = w.output
	w.output.touch(w)
}

// focusRaise focuses w and, if it floats, raises it: what dwm does on a
// click or a keyboard focus change, unlike on hover.
func (s *Seat) focusRaise(w *Window) {
	s.focus(w)
	if w != nil && w.output != nil && w.output.isFloating(w) {
		s.wm.raise(w)
	}
}

// ensureOutput gives a seat left without an output, as after a VT switch,
// its focused window's output, or else the first one.
func (s *Seat) ensureOutput() {
	switch {
	case s.output != nil || len(s.wm.outputs) == 0:
	case s.focused != nil && s.focused.output != nil:
		s.output = s.focused.output
	default:
		s.output = s.wm.outputs[0]
	}
}

// focusOutput moves focus to o and the window last focused there.
func (s *Seat) focusOutput(o *Output) {
	s.output = o
	s.focus(o.lastFocused())
}

// manageInput turns pointer events into focus changes. As in dwm, focus
// follows the pointer only when it moves onto another window. Motion is
// compared with the window under the pointer, not with pointer_enter, which
// river may report in a later manage sequence.
func (s *Seat) manageInput() {
	if w := s.interacted; w != nil && w.output != nil {
		s.focusRaise(w)
	}
	if s.moved {
		switch h := s.hovered; {
		case h != nil && h.output != nil && h != s.lastHovered:
			s.focus(h)
		case h == nil:
			if o := s.wm.outputAt(s.px, s.py); o != nil && o != s.output {
				s.focusOutput(o)
			}
		}
		s.lastHovered = s.hovered
	}
	s.interacted, s.moved = nil, false
}

// runBindings runs the commands of the bindings pressed since the last
// manage sequence, then any repeats that are due.
func (s *Seat) runBindings() {
	for _, p := range s.pressed {
		p.b.run(p.target, false)
	}
	s.pressed = s.pressed[:0]
	now := time.Now()
	for _, b := range s.bindings {
		b.repeat(now)
	}
}

// syncBindings makes river's bindings match the config, and enables those
// of the active mode. While the session is locked only -locked bindings
// work.
func (s *Seat) syncBindings() {
	wm := s.wm
	if s.bindings == nil || s.bindingsGen != wm.cfgGen {
		for _, b := range s.bindings {
			b.destroy()
		}
		s.bindings = s.bindings[:0]
		for _, m := range wm.cfg.Modes {
			for _, cfgs := range sameKey(m.Bindings, wm.cfg.Mod) {
				s.bindings = append(s.bindings, newBinding(s, m.Name, cfgs))
			}
		}
		s.bindingsGen = wm.cfgGen
	}
	for _, b := range s.bindings {
		b.setEnabled(b.mode == wm.mode && (!wm.locked || b.locked()))
	}
}

// sameKey groups a mode's bindings by key, in config order: bindings on
// the same key, once Mod is resolved, share a group.
func sameKey(bs []config.Binding, mod config.Modifiers) [][]config.Binding {
	var groups [][]config.Binding
	for _, b := range bs {
		i := slices.IndexFunc(groups, func(g []config.Binding) bool { return config.SameKey(g[0], b, mod) })
		if i < 0 {
			groups = append(groups, []config.Binding{b})
		} else {
			groups[i] = append(groups[i], b)
		}
	}
	return groups
}

// syncFocus tells river about a focus change, and only then: requesting
// focus in the same manage sequence a layer surface asks for it would take
// it away from that surface.
func (s *Seat) syncFocus() {
	if s.layerFocus == layerFocusExclusive || (!s.resend && s.sent == s.focused) {
		return
	}
	if s.focused != nil {
		s.wm.log.Debug("focus", "app_id", s.focused.appID, "title", s.focused.title)
		s.obj.FocusWindow(s.focused.obj)
	} else {
		s.wm.log.Debug("focus cleared")
		s.obj.ClearFocus()
	}
	s.sent, s.resend = s.focused, false
}

func (wm *WM) raise(w *Window) {
	wm.raised = slices.DeleteFunc(wm.raised, func(r *Window) bool { return r == w })
	wm.raised = append(wm.raised, w)
}

// focusedBySeat reports whether w has keyboard focus on any seat, which
// gives it the focused border colour.
func (wm *WM) focusedBySeat(w *Window) bool {
	for _, s := range wm.seats {
		if s.focused == w && s.layerFocus == layerFocusNone {
			return true
		}
	}
	return false
}

// outputAt returns the output containing the point x, y, if any.
func (wm *WM) outputAt(x, y int32) *Output {
	for _, o := range wm.outputs {
		if o.area.Contains(x, y) {
			return o
		}
	}
	return nil
}

func (s *Seat) HandleRiverSeatV1Removed(ctx context.Context) {
	s.removed = true
}

func (s *Seat) HandleRiverSeatV1PointerEnter(ctx context.Context, window proto.RiverWindowV1) {
	s.hovered = window.UserData().(*Window)
	s.wm.log.Debug("pointer enter", "app_id", s.hovered.appID)
}

func (s *Seat) HandleRiverSeatV1PointerLeave(ctx context.Context) {
	s.wm.log.Debug("pointer leave")
	s.hovered = nil
}

func (s *Seat) HandleRiverSeatV1PointerPosition(ctx context.Context, x, y int32) {
	s.px, s.py, s.moved = x, y, true
}

func (s *Seat) HandleRiverSeatV1WindowInteraction(ctx context.Context, window proto.RiverWindowV1) {
	s.interacted = window.UserData().(*Window)
}

func (s *Seat) HandleRiverLayerShellSeatV1FocusExclusive(ctx context.Context) {
	s.wm.log.Debug("layer surface took exclusive focus")
	s.layerFocus = layerFocusExclusive
}

// HandleRiverLayerShellSeatV1FocusNonExclusive leaves river's focus with the
// layer surface: swm considers its own request already sent, so it only asks
// again when focus actually moves.
func (s *Seat) HandleRiverLayerShellSeatV1FocusNonExclusive(ctx context.Context) {
	s.wm.log.Debug("layer surface took focus")
	s.layerFocus = layerFocusNonExclusive
	s.sent, s.resend = s.focused, false
}

// HandleRiverLayerShellSeatV1FocusNone gives focus back to swm's focused
// window once the layer surface lets go.
func (s *Seat) HandleRiverLayerShellSeatV1FocusNone(ctx context.Context) {
	s.wm.log.Debug("layer surface let focus go")
	s.layerFocus = layerFocusNone
	s.resend = true
}
