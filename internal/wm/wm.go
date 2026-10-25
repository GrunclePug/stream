// Package wm is the window manager: it holds the state river reports and
// answers river's manage and render sequences.
package wm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"syscall"
	"time"

	"github.com/grunclepug/stream/internal/config"
	"github.com/grunclepug/stream/internal/ipc"
	"github.com/grunclepug/stream/internal/proto"
	"hazelnut.eclair.cafe/wlcl"
)

// Protocol versions swm is written against. river 0.5.0 advertises these.
const (
	windowManagerVersion = 6
	xkbBindingsVersion   = 3
	layerShellVersion    = 1
	wlOutputVersion      = 4 // the first with the name event
)

// stopTimeout bounds how long shutdown waits for river to confirm stop.
const stopTimeout = 2 * time.Second

// WM is the window manager state. Every method runs on the goroutine that
// dispatches Wayland events, so nothing here needs locking.
type WM struct {
	proto.WlRegistryStub
	proto.RiverWindowManagerV1Stub
	proto.RiverInputManagerV1Stub
	proto.RiverLibinputConfigV1Stub

	log *slog.Logger

	// cfg is the config in use, read from cfgPath. cfgGen counts the configs
	// applied, so seats know when to remake their bindings.
	cfg     config.Config
	cfgPath string
	cfgGen  int

	cfgErrors []string // why the config file was last refused, for watch

	mode   string // the active binding mode
	locked bool   // the session is locked

	ipc *ipc.Server // nil when the socket couldn't be made

	// scratchSpawned is when each scratchpad's command was last started,
	// so calling it again while it starts doesn't start a second one.
	scratchSpawned map[string]time.Time

	persist persist

	registry   proto.WlRegistry
	manager    proto.RiverWindowManagerV1
	xkb        proto.RiverXkbBindingsV1
	layerShell proto.RiverLayerShellV1

	// The input globals, unset when river doesn't offer them. libinput is
	// bound after the input manager, whatever order river lists them in, so
	// river announces each device to the input manager first and every
	// libinput device finds its own.
	inputManager   proto.RiverInputManagerV1
	libinput       proto.RiverLibinputConfigV1
	libinputGlobal uint32 // its registry name, 0 when not offered

	// Advertised versions of the globals, so a too old river gets a clear
	// error instead of a protocol error later.
	managerOffered    uint32
	xkbOffered        uint32
	layerShellOffered uint32
	wlOutputs         map[uint32]uint32 // wl_output global name to version

	inputs []*inputDevice

	windows []*Window // every window, in the order river created them
	outputs []*Output
	seats   []*Seat

	// raised orders windows from bottom to top by when they were last
	// raised. It decides how floating windows stack.
	raised []*Window

	// parked holds, in stacking order, the windows of the last output while
	// no output is there (Window.parkedOn).
	parked []*Window

	// defaultOutput is the output layer surfaces open on when they don't ask
	// for one.
	defaultOutput *Output

	finished bool
	exiting  bool // quit asked river to end the session
	err      error
}

// Run connects to river and manages windows until ctx is cancelled or river
// stops the window manager. It reads the config file at cfgPath; when that
// fails, swm logs why and runs on the defaults.
func Run(ctx context.Context, log *slog.Logger, cfgPath string) (err error) {
	cfg, cfgErr := config.Load(cfgPath)
	if cfgErr != nil {
		logConfigError(log, cfgErr)
		log.Warn("using the default config", "path", cfgPath)
		cfg = config.Default()
	}
	logConfigWarnings(log, cfg)

	conn, err := wlcl.Connect(ctx, "")
	if err != nil {
		return fmt.Errorf("connect to river: %w", err)
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil && !errors.Is(cerr, wlcl.ErrClosed) && err == nil {
			err = fmt.Errorf("close connection: %w", cerr)
		}
	}()

	// WAYLAND_DEBUG would otherwise be inherited by every program swm starts.
	_ = os.Unsetenv("WAYLAND_DEBUG")

	display := proto.CreateDisplay(conn)
	wm := &WM{
		log:            log,
		cfg:            cfg,
		cfgPath:        cfgPath,
		mode:           config.NormalMode,
		wlOutputs:      map[uint32]uint32{},
		scratchSpawned: map[string]time.Time{},
	}
	wm.loadState()
	wm.registry = display.GetRegistry()
	wm.registry.SetUserData(wm)
	if err := wlcl.Roundtrip(ctx, display); err != nil {
		return fmt.Errorf("roundtrip: %w", err)
	}
	if err := wm.checkGlobals(); err != nil {
		return err
	}
	wm.manager.SetUserData(wm)
	wm.bindLibinput()
	wm.cfgErrors = errorLines(cfgErr)
	log.Info("connected to river")
	if len(wm.persist.saved.Windows) > 0 {
		// Restored windows wait for outputs to be named; make sure a manage
		// sequence comes when the wait is over.
		t := time.AfterFunc(restoreWait, wm.poke)
		defer t.Stop()
	}

	if path, err := ipc.SocketPath(); err != nil {
		log.Warn("no control socket: " + err.Error())
	} else if wm.ipc, err = ipc.Listen(path, wm.poke); err != nil {
		log.Warn("no control socket", "err", err)
	} else {
		wm.ipc.Serve()
		defer wm.ipc.Close()
		log.Debug("control socket", "path", path)
	}

	for !wm.finished {
		if err := conn.Dispatch(ctx); err != nil {
			switch {
			case wm.exiting:
				// quit ended the session: river disconnects every client,
				// and may signal swm too.
				log.Info("session ended")
				return nil
			case ctx.Err() != nil:
				return wm.stop(conn)
			}
			return fmt.Errorf("dispatch: %w", err)
		}
	}
	return wm.err
}

func (wm *WM) checkGlobals() error {
	missing := func(name string, offered, want uint32) error {
		if offered == 0 {
			return fmt.Errorf("river does not offer %s; is swm running under river 0.5 or newer?", name)
		}
		return fmt.Errorf("river offers %s version %d, swm needs %d (river 0.5 or newer)", name, offered, want)
	}
	if !wm.manager.IsSet() {
		return missing(proto.RiverWindowManagerV1Name, wm.managerOffered, windowManagerVersion)
	}
	if !wm.xkb.IsSet() {
		return missing(proto.RiverXkbBindingsV1Name, wm.xkbOffered, xkbBindingsVersion)
	}
	if !wm.layerShell.IsSet() {
		return missing(proto.RiverLayerShellV1Name, wm.layerShellOffered, layerShellVersion)
	}
	return nil
}

// bindLibinput binds river's libinput config global, if it offers one along
// with the input manager. Without them swm leaves input devices alone.
func (wm *WM) bindLibinput() {
	if !wm.inputManager.IsSet() {
		wm.log.Info("river offers no input manager: input lines and key repeat don't apply")
		return
	}
	if wm.libinputGlobal == 0 {
		wm.log.Info("river offers no libinput config: input lines don't apply")
		return
	}
	wm.libinput = proto.As[proto.RiverLibinputConfigV1](wm.registry.Bind(wm.libinputGlobal, proto.RiverLibinputConfigV1Name, libinputConfigVersion))
	wm.libinput.SetUserData(wm)
}

// stop asks river to stop sending events and waits for the confirmation, so
// river doesn't treat the exit as a crash.
func (wm *WM) stop(conn *wlcl.Connection) error {
	wm.log.Info("stopping")
	for _, s := range wm.seats {
		for _, b := range s.bindings {
			b.stopRepeat()
		}
	}
	wm.manager.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), stopTimeout)
	defer cancel()
	for !wm.finished {
		if err := conn.Dispatch(ctx); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, syscall.ECONNRESET) {
				// river is exiting too, as when its session ends: nothing
				// is left to stop.
				wm.log.Info("river is gone")
				return nil
			}
			return fmt.Errorf("wait for river to stop: %w", err)
		}
	}
	return wm.err
}

func (wm *WM) HandleWlRegistryGlobal(ctx context.Context, name uint32, iface string, version uint32) {
	switch iface {
	case proto.RiverWindowManagerV1Name:
		wm.managerOffered = version
		if version >= windowManagerVersion {
			wm.manager = proto.As[proto.RiverWindowManagerV1](wm.registry.Bind(name, iface, windowManagerVersion))
		}
	case proto.RiverXkbBindingsV1Name:
		wm.xkbOffered = version
		if version >= xkbBindingsVersion {
			wm.xkb = proto.As[proto.RiverXkbBindingsV1](wm.registry.Bind(name, iface, xkbBindingsVersion))
		}
	case proto.RiverLayerShellV1Name:
		wm.layerShellOffered = version
		if version >= layerShellVersion {
			wm.layerShell = proto.As[proto.RiverLayerShellV1](wm.registry.Bind(name, iface, layerShellVersion))
		}
	case proto.RiverInputManagerV1Name:
		if version >= inputManagerVersion {
			wm.inputManager = proto.As[proto.RiverInputManagerV1](wm.registry.Bind(name, iface, inputManagerVersion))
			wm.inputManager.SetUserData(wm)
		}
	case proto.RiverLibinputConfigV1Name:
		if version >= libinputConfigVersion {
			wm.libinputGlobal = name
		}
	case proto.WlOutputName:
		wm.wlOutputs[name] = version
	}
}

func (wm *WM) HandleWlRegistryGlobalRemove(ctx context.Context, name uint32) {
	delete(wm.wlOutputs, name)
}

func (wm *WM) HandleRiverWindowManagerV1Unavailable(ctx context.Context) {
	wm.finished = true
	wm.err = errors.New("river refused window management: another window manager is running")
}

func (wm *WM) HandleRiverWindowManagerV1Finished(ctx context.Context) {
	wm.finished = true
}

func (wm *WM) HandleRiverWindowManagerV1SessionLocked(ctx context.Context) {
	wm.log.Debug("session locked")
	wm.locked = true
}

func (wm *WM) HandleRiverWindowManagerV1SessionUnlocked(ctx context.Context) {
	wm.log.Debug("session unlocked")
	wm.locked = false
}

// poke asks river for a manage sequence. It is the one thing other
// goroutines (the repeat timers) do: requests are safe to send from any
// goroutine, state is not.
func (wm *WM) poke() {
	wm.manager.ManageDirty()
	// An error here means the connection is gone, which the dispatch loop
	// reports.
	_ = wm.manager.Conn().Flush()
}

func (wm *WM) HandleRiverWindowManagerV1Window(ctx context.Context, id proto.RiverWindowV1) {
	wm.windows = append(wm.windows, newWindow(wm, id))
}

func (wm *WM) HandleRiverWindowManagerV1Output(ctx context.Context, id proto.RiverOutputV1) {
	wm.outputs = append(wm.outputs, newOutput(wm, id))
}

func (wm *WM) HandleRiverWindowManagerV1Seat(ctx context.Context, id proto.RiverSeatV1) {
	wm.seats = append(wm.seats, newSeat(wm, id))
}

// HandleRiverWindowManagerV1ManageStart applies everything river reported
// since the last manage sequence. The order matters: objects that went away
// are removed first, so nothing below sees them.
func (wm *WM) HandleRiverWindowManagerV1ManageStart(ctx context.Context) {
	wm.reapWindows()
	wm.reapOutputs()
	wm.seats = slices.DeleteFunc(wm.seats, (*Seat).destroyIfRemoved)

	wm.adoptOrphans()
	for _, s := range wm.seats {
		s.ensureOutput()
	}
	// Pointer motion alone doesn't start a manage sequence, so the pointer
	// may have moved to another output since the last one. That decides
	// where new windows open, so it comes first.
	for _, s := range wm.seats {
		s.manageInput()
	}
	for _, w := range wm.windows {
		if w.new {
			wm.manageNew(w)
		}
		w.manageRequests()
	}
	wm.finishRestore()
	for _, s := range wm.seats {
		s.runBindings()
	}
	wm.runRequests()
	for _, s := range wm.seats {
		s.manageDragRequests()
		s.manageOp()
	}
	for _, o := range wm.outputs {
		o.arrange()
	}
	for _, s := range wm.seats {
		s.syncFocus()
		s.syncBindings()
	}
	wm.syncDefaultOutput()

	wm.manager.ManageFinish()
	wm.publish()
	wm.saveState()
}

func (wm *WM) HandleRiverWindowManagerV1RenderStart(ctx context.Context) {
	for _, o := range wm.outputs {
		o.render()
	}
	wm.manager.RenderFinish()
}

// primarySeat is the seat whose focused output new windows open on.
func (wm *WM) primarySeat() *Seat {
	if len(wm.seats) == 0 {
		return nil
	}
	return wm.seats[0]
}

// focusedOutput is the output new windows open on.
func (wm *WM) focusedOutput() *Output {
	if s := wm.primarySeat(); s != nil && s.output != nil {
		return s.output
	}
	if len(wm.outputs) > 0 {
		return wm.outputs[0]
	}
	return nil
}

// manageNew sets up a window river just announced and puts it on the
// focused output's shown tags as the new master, focused, as dwm does.
// Matching rules can float or tile it and send it to other tags or another
// output; it is only focused if it opens where the user is looking.
func (wm *WM) manageNew(w *Window) {
	restored, wait := wm.restoreWindow(w)
	if wait {
		return // still new: tried again next manage sequence
	}
	w.new = false
	w.obj.SetCapabilities(proto.RiverWindowV1CapabilitiesFullscreen)
	if restored {
		return
	}
	w.floating = w.parent != nil || w.fixedSize()
	o := wm.focusedOutput()
	var tags uint32
	for _, r := range wm.cfg.Rules {
		if !r.Match(w.appID, w.title) {
			continue
		}
		switch r.Float {
		case config.FloatOn:
			w.floating = true
		case config.FloatOff:
			w.floating = false
		}
		tags |= r.Tags
		if m := wm.outputNamed(r.Output); r.Output != "" && m != nil {
			o = m
		}
	}
	// The first window with a scratchpad's app-id is that scratchpad, and
	// opens floating where the user is, whatever the rules say.
	if sp, ok := wm.freeScratchpad(w.appID); ok {
		w.scratch, w.floating, tags = sp.Name, true, 0
		o = wm.focusedOutput()
		delete(wm.scratchSpawned, sp.Name)
	}
	wm.raise(w) // new windows map on top, as in dwm
	wm.log.Debug("window", "id", w.id, "app_id", w.appID, "title", w.title, "floating", w.floating, "scratchpad", w.scratch)

	if o == nil {
		return // adopted when an output appears
	}
	o.attach(w)
	if tags != 0 {
		w.tags = tags
	}
	if s := wm.primarySeat(); s != nil && (s.output == nil || o == s.output) && w.visible() {
		s.focus(w)
	}
}

// freeScratchpad returns the scratchpad with the given app-id, if no
// window is it yet.
func (wm *WM) freeScratchpad(appID string) (config.Scratchpad, bool) {
	for _, sp := range wm.cfg.Scratchpads {
		if sp.AppID == appID && wm.scratchWindow(sp.Name) == nil {
			return sp, true
		}
	}
	return config.Scratchpad{}, false
}

func (wm *WM) scratchWindow(name string) *Window {
	for _, w := range wm.windows {
		if w.scratch == name && !w.new {
			return w
		}
	}
	return nil
}

// outputNamed returns the output with the given name, if it is connected.
func (wm *WM) outputNamed(name string) *Output {
	for _, o := range wm.outputs {
		if o.name == name {
			return o
		}
	}
	return nil
}

// adoptOrphans gives windows without an output one. Parked windows go back
// to their output with their tags and order once it's named, or else to the
// focused output; windows that opened with no output go to the focused one.
func (wm *WM) adoptOrphans() {
	o := wm.focusedOutput()
	if o == nil {
		return
	}
	if len(wm.parked) > 0 && !wm.unnamedOutputs() {
		for _, w := range wm.parked {
			if m := wm.outputNamed(w.parkedOn); m != nil {
				tags := w.tags
				m.attachBottom(w)
				w.tags = tags
			} else {
				o.attachBottom(w)
			}
			w.parkedOn = ""
		}
		wm.parked = nil
	}
	for _, w := range wm.windows {
		if w.output == nil && !w.new && w.parkedOn == "" {
			o.attach(w)
		}
	}
}

func (wm *WM) reapWindows() {
	wm.windows = slices.DeleteFunc(wm.windows, func(w *Window) bool {
		if !w.closed {
			return false
		}
		wm.log.Debug("window closed", "id", w.id, "app_id", w.appID)
		if w.output != nil {
			w.output.detach(w)
		}
		wm.raised = slices.DeleteFunc(wm.raised, func(r *Window) bool { return r == w })
		wm.parked = slices.DeleteFunc(wm.parked, func(p *Window) bool { return p == w })
		for _, other := range wm.windows {
			if other.parent == w {
				other.parent = nil
			}
		}
		for _, s := range wm.seats {
			s.forget(w)
		}
		w.destroy()
		return true
	})
}

func (wm *WM) reapOutputs() {
	var gone []*Output
	wm.outputs = slices.DeleteFunc(wm.outputs, func(o *Output) bool {
		if o.removed {
			gone = append(gone, o)
		}
		return o.removed
	})
	for _, o := range gone {
		wm.log.Debug("output removed", "name", o.name)
		var fallback *Output
		if len(wm.outputs) > 0 {
			fallback = wm.outputs[0]
		}
		for _, w := range o.clients {
			w.output = nil
			if fallback != nil {
				fallback.attachBottom(w)
			} else {
				// No other output: the windows wait for this one, as on a
				// VT switch, when river removes every output.
				w.parkedOn = o.name
				wm.parked = append(wm.parked, w)
			}
		}
		for _, s := range wm.seats {
			if s.output == o {
				s.output = fallback
			}
		}
		if wm.defaultOutput == o {
			wm.defaultOutput = nil
		}
		o.destroy()
	}
}

// syncDefaultOutput makes the focused output the one layer surfaces such as
// launchers and notifications open on.
func (wm *WM) syncDefaultOutput() {
	o := wm.focusedOutput()
	if o == nil || o == wm.defaultOutput {
		return
	}
	o.layer.SetDefault()
	wm.defaultOutput = o
}
