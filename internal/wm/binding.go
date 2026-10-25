package wm

import (
	"context"
	"slices"
	"time"

	"github.com/grunclepug/stream/internal/config"
	"github.com/grunclepug/stream/internal/proto"
)

// binding is one key or button of a mode on a seat. river runs one binding
// per key, so all of a key's configured bindings share one river binding.
type binding struct {
	proto.RiverXkbBindingV1Stub
	proto.RiverPointerBindingV1Stub

	seat *Seat
	cfgs []config.Binding
	mode string

	key     proto.RiverXkbBindingV1     // set for a key binding
	pointer proto.RiverPointerBindingV1 // set for a pointer binding
	enabled bool

	// Repeat state of a held -repeat binding. The timer only asks river for
	// a manage sequence; the repeat itself runs there.
	held      bool
	pressedAt time.Time
	repeats   int
	timer     *time.Timer
}

func newBinding(s *Seat, mode string, cfgs []config.Binding) *binding {
	b := &binding{seat: s, cfgs: cfgs, mode: mode}
	cfg := cfgs[0]
	mods := uint32(cfg.Mods.Resolve(s.wm.cfg.Mod))
	if cfg.Button != 0 {
		b.pointer = s.obj.GetPointerBinding(cfg.Button, mods)
		b.pointer.SetUserData(b)
	} else {
		b.key = s.wm.xkb.GetXkbBinding(s.obj, cfg.Keysym, mods)
		b.key.SetUserData(b)
	}
	return b
}

func (b *binding) setEnabled(on bool) {
	if on == b.enabled {
		return
	}
	b.enabled = on
	switch {
	case b.key.IsSet() && on:
		b.key.Enable()
	case b.key.IsSet():
		b.key.Disable()
	case on:
		b.pointer.Enable()
	default:
		b.pointer.Disable()
	}
	if !on {
		b.stopRepeat()
	}
}

func (b *binding) destroy() {
	b.stopRepeat()
	if b.key.IsSet() {
		b.key.Destroy()
	} else {
		b.pointer.Destroy()
	}
}

// press queues the binding's command for the next manage sequence. A
// pointer binding acts on the window under the pointer.
func (b *binding) press() {
	b.seat.wm.log.Debug("binding pressed", "keys", b.cfgs[0].Keys(), "mode", b.mode)
	var target *Window
	if b.pointer.IsSet() {
		target = b.seat.hovered
	}
	b.seat.pressed = append(b.seat.pressed, pressed{b, target})
	if slices.ContainsFunc(b.cfgs, func(c config.Binding) bool { return c.Repeat }) {
		b.held, b.pressedAt, b.repeats = true, time.Now(), 0
	}
}

// run runs the commands of the binding's -repeat bindings, or of all of
// them. While the session is locked only -locked bindings run.
func (b *binding) run(target *Window, repeat bool) {
	for _, c := range b.cfgs {
		if (repeat && !c.Repeat) || (b.seat.wm.locked && !c.Locked) {
			continue
		}
		b.seat.wm.exec(b.seat, c.Command, target)
	}
}

// locked reports whether the binding works while the session is locked.
func (b *binding) locked() bool {
	return slices.ContainsFunc(b.cfgs, func(c config.Binding) bool { return c.Locked })
}

// pressed is a binding pressed since the last manage sequence.
type pressed struct {
	b      *binding
	target *Window // nil for the focused window
}

func (b *binding) stopRepeat() {
	b.held = false
	if b.timer != nil {
		b.timer.Stop()
		b.timer = nil
	}
}

// repeat runs the command again if a repeat is due, and sets a timer for
// the next one. Repeats missed while swm was busy are skipped, not caught up.
func (b *binding) repeat(now time.Time) {
	if !b.held {
		return
	}
	cfg := b.seat.wm.cfg
	delay := time.Duration(cfg.RepeatDelay) * time.Millisecond
	interval := time.Second / time.Duration(cfg.RepeatRate)
	if since := now.Sub(b.pressedAt); since >= delay {
		if due := 1 + int((since-delay)/interval); due > b.repeats {
			b.repeats = due
			b.run(nil, true)
		}
	}
	next := b.pressedAt.Add(delay + time.Duration(b.repeats)*interval)
	if b.timer != nil {
		b.timer.Stop()
	}
	b.timer = time.AfterFunc(next.Sub(now), b.seat.wm.poke)
}

func (b *binding) HandleRiverXkbBindingV1Pressed(ctx context.Context)    { b.press() }
func (b *binding) HandleRiverXkbBindingV1Released(ctx context.Context)   { b.stopRepeat() }
func (b *binding) HandleRiverXkbBindingV1StopRepeat(ctx context.Context) { b.stopRepeat() }

func (b *binding) HandleRiverPointerBindingV1Pressed(ctx context.Context) { b.press() }
