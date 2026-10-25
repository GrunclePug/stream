package wm

import (
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"syscall"
	"time"

	"github.com/grunclepug/stream/internal/command"
	"github.com/grunclepug/stream/internal/config"
	"github.com/grunclepug/stream/internal/layout"
)

// exec runs a command for seat s. target is the window a pointer binding
// was pressed over: as in dwm, it is focused first, and the command then
// acts on the focused window.
func (wm *WM) exec(s *Seat, c command.Command, target *Window) {
	wm.log.Debug("command", "cmd", c.String())
	if target != nil && target.output != nil {
		s.focusRaise(target)
	}
	o, w := s.output, s.focused
	var arg string
	if len(c.Args) > 0 {
		arg = c.Args[0]
	}

	switch c.Name {
	case "spawn":
		wm.spawn(arg)
	case "quit":
		wm.exiting = true
		wm.forgetState()
		wm.manager.ExitSession()
	case "reload":
		_ = wm.reload() // logged
	case "mode":
		wm.mode = arg
	case "focus-mon":
		if m := wm.outputNextTo(o, arg); m != nil && m != o {
			s.focusOutput(m)
		}
	case "move", "resize":
		s.drag(target, c.Name == "resize")
	}
	if o == nil {
		wm.log.Debug("command needs an output, and there is none", "cmd", c.String())
		return
	}

	switch c.Name {
	case "view":
		o.view(tagMask(arg))
		s.refocus()
	case "toggle-view":
		if t := o.tags() ^ tagMask(arg); t != 0 {
			o.tagset[o.seltags] = t
			s.refocus()
		}
	case "layout":
		o.setLayout(arg)
	case "mfact":
		if o.params().Kind == layout.Floating {
			break // as in dwm: the floating layout has no master area
		}
		t := o.lay()
		f := parseFloat(arg)
		if command.Relative(arg) {
			f += t.mfact
		}
		if f >= 0.05 && f <= 0.95 {
			t.mfact = f
		}
	case "nmaster":
		o.setNMaster(arg)
	case "gap":
		switch {
		case arg == "reset":
			o.gap = wm.cfg.Gap
		case command.Relative(arg):
			o.gap = max(o.gap+int32(parseInt(arg)), 0)
		default:
			o.gap = int32(parseInt(arg))
		}
	case "scratchpad":
		wm.toggleScratchpad(s, arg)
	case "reset":
		if arg == "" || arg == "mfact" {
			o.lay().mfact = wm.cfg.MFact
		}
		if arg == "" || arg == "weights" {
			for _, v := range o.visible() {
				v.weight = 0
			}
		}
	}
	if w == nil {
		return
	}

	switch c.Name {
	case "close":
		w.obj.Close()
	case "focus":
		if !w.fullscreen { // dwm's lockfullscreen
			s.focusRaise(neighbour(o.visible(), w, arg))
		}
	case "swap":
		vis := o.visible()
		if n := neighbour(vis, w, arg); n != w {
			i, j := slices.Index(o.clients, w), slices.Index(o.clients, n)
			o.clients[i], o.clients[j] = o.clients[j], o.clients[i]
		}
	case "zoom":
		o.zoom(s, w)
	case "toggle-float":
		if w.fullscreen {
			break
		}
		w.floating = !w.floating || w.fixedSize()
		if w.floating {
			wm.raise(w)
		}
	case "toggle-fullscreen":
		w.setFullscreen(!w.fullscreen)
	case "tag":
		w.tags = tagMask(arg)
		s.refocus()
	case "toggle-tag":
		if t := toggleTags(w.tags, arg, o.tags()); t != 0 {
			w.tags = t
			s.refocus()
		}
	case "tag-mon":
		if m := wm.outputNextTo(o, arg); m != nil && m != o {
			wm.sendToOutput(w, m, true)
			s.refocus()
		}
	}
}

// scratchSpawnWait is how long a scratchpad's command has to open its
// window before calling the scratchpad starts the command again.
const scratchSpawnWait = 5 * time.Second

// toggleScratchpad shows the scratchpad over the seat's output's shown tags,
// focused, or hides it if it's showing there. With no window yet, it starts
// the scratchpad's command.
func (wm *WM) toggleScratchpad(s *Seat, name string) {
	sp, ok := wm.cfg.Scratchpad(name)
	if !ok {
		return
	}
	o := s.output
	w := wm.scratchWindow(name)
	switch {
	case w == nil:
		if t, ok := wm.scratchSpawned[name]; ok && time.Since(t) < scratchSpawnWait {
			return
		}
		wm.scratchSpawned[name] = time.Now()
		wm.spawn(sp.Command)
	case w.output == o && w.visible():
		w.tags = 0
		s.refocus()
	default:
		if w.output != o {
			wm.sendToOutput(w, o, true)
		}
		w.tags, w.floating = o.tags(), true
		s.focusRaise(w)
	}
}

// tagMask turns a tag argument into a mask: "3" is tag 3, "all" every tag,
// and "prev" (for view) none.
func tagMask(arg string) uint32 {
	switch arg {
	case "all":
		return allTags
	case "prev":
		return 0
	}
	return 1 << (parseInt(arg) - 1)
}

// toggleTags returns a window's tags after toggle-tag: a tag is added or
// removed, and "all" puts the window on every tag, or, if it is already,
// back on just the shown ones.
func toggleTags(tags uint32, arg string, shown uint32) uint32 {
	if arg != "all" {
		return tags ^ tagMask(arg)
	}
	if tags == allTags {
		return shown
	}
	return allTags
}

// Arguments were checked when the command was parsed, so these can't fail.
func parseInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// neighbour returns the window after (dir "next") or before w in ws,
// wrapping around, or w itself if it isn't in ws.
func neighbour(ws []*Window, w *Window, dir string) *Window {
	i := slices.Index(ws, w)
	if i < 0 {
		return w
	}
	if dir == "next" {
		return ws[(i+1)%len(ws)]
	}
	return ws[(i-1+len(ws))%len(ws)]
}

// refocus focuses the window dwm's focus(NULL) would after the shown
// windows change: the most recently focused visible one on the seat's
// output.
func (s *Seat) refocus() {
	if s.output != nil {
		s.focusRaise(s.output.lastFocused())
	}
}

// view shows the tags in mask, or with mask 0 the tags shown before. As in
// dwm, viewing the tags already shown does nothing, so the previous tags
// stay what they were.
func (o *Output) view(mask uint32) {
	if mask == o.tags() {
		return
	}
	o.seltags ^= 1
	if mask != 0 {
		o.tagset[o.seltags] = mask
	}
}

// setLayout is dwm's setlayout: "prev", or the layout already in use,
// swaps back to the previous layout; another layout replaces the current
// one and remembers it as the previous.
func (o *Output) setLayout(name string) {
	t := o.lay()
	if name == "prev" {
		t.sel ^= 1
		return
	}
	k, _ := layout.KindNamed(name)
	if k != t.kinds[t.sel] {
		t.sel ^= 1
		t.kinds[t.sel] = k
	}
}

// zoom is dwm's zoom: the focused tiled window becomes the master, or, if
// it already is, the next tiled window does and takes focus.
func (o *Output) zoom(s *Seat, w *Window) {
	if o.isFloating(w) || w.fullscreen {
		return
	}
	tiled := o.tiled()
	if len(tiled) > 0 && tiled[0] == w {
		if len(tiled) < 2 {
			return
		}
		w = tiled[1]
	}
	o.clients = slices.DeleteFunc(o.clients, func(c *Window) bool { return c == w })
	o.clients = slices.Insert(o.clients, 0, w)
	s.focusRaise(w)
}

// sendToOutput moves w to output m, onto m's shown tags (dwm's sendmon).
// It becomes m's master. With translate, a floating window keeps its place
// relative to the output; without, it stays where it is on screen.
func (wm *WM) sendToOutput(w *Window, m *Output, translate bool) {
	from := w.output.area
	w.output.detach(w)
	m.attach(w)
	if w.fullscreen {
		w.obj.Fullscreen(m.obj)
	}
	if translate && w.float.W > 0 {
		// A floating window keeps its place relative to the output, kept
		// inside the usable area.
		f := &w.float
		f.X = m.area.X + f.X - from.X
		f.Y = m.area.Y + f.Y - from.Y
		u := m.usable
		f.X = max(min(f.X, u.X+u.W-f.W), u.X)
		f.Y = max(min(f.Y, u.Y+u.H-f.H), u.Y)
	}
}

// outputNextTo returns the output after (dir "next") or before o, ordered
// left to right, then top to bottom.
func (wm *WM) outputNextTo(o *Output, dir string) *Output {
	if o == nil || len(wm.outputs) == 0 {
		return nil
	}
	sorted := wm.leftToRight()
	i := slices.Index(sorted, o)
	if dir == "next" {
		return sorted[(i+1)%len(sorted)]
	}
	return sorted[(i-1+len(sorted))%len(sorted)]
}

// leftToRight returns the outputs ordered left to right, then top to bottom.
func (wm *WM) leftToRight() []*Output {
	sorted := slices.Clone(wm.outputs)
	slices.SortStableFunc(sorted, func(a, b *Output) int {
		if a.area.X != b.area.X {
			return int(a.area.X - b.area.X)
		}
		return int(a.area.Y - b.area.Y)
	})
	return sorted
}

// spawn runs a shell command in its own session, so it outlives swm and
// doesn't get swm's signals.
func (wm *WM) spawn(cmdline string) {
	cmd := exec.Command("/bin/sh", "-c", cmdline)
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		wm.log.Error("spawn", "cmd", cmdline, "err", err)
		return
	}
	// Wait reaps the child. The goroutine ends when the child exits.
	go func() { _ = cmd.Wait() }()
}

// reload reads the config file again. A broken file changes nothing: the
// config in use stays, and the errors are logged. Runtime state such as
// each tag's layout keeps its current values.
func (wm *WM) reload() error {
	cfg, err := config.Load(wm.cfgPath)
	wm.cfgErrors = errorLines(err)
	if err != nil {
		logConfigError(wm.log, err)
		wm.log.Warn("config not reloaded", "path", wm.cfgPath)
		return err
	}
	wm.cfg = cfg
	wm.cfgGen++
	logConfigWarnings(wm.log, cfg)
	// A window whose scratchpad is gone becomes an ordinary window, shown
	// if it was hidden.
	for _, w := range wm.windows {
		if _, ok := cfg.Scratchpad(w.scratch); w.scratch != "" && !ok {
			w.scratch = ""
			if w.tags == 0 && w.output != nil {
				w.tags = w.output.tags()
			}
		}
	}
	if !wm.hasMode(wm.mode) {
		wm.mode = config.NormalMode
	}
	wm.log.Info("config reloaded", "path", wm.cfgPath)
	return nil
}

func (wm *WM) hasMode(name string) bool {
	for _, m := range wm.cfg.Modes {
		if m.Name == name {
			return true
		}
	}
	return false
}

// logConfigError logs each problem in a config file on its own line.
func logConfigError(log *slog.Logger, err error) {
	for _, line := range errorLines(err) {
		log.Error("config", "err", line)
	}
}

// warningLines lists the config's warnings, one per line.
func warningLines(cfg config.Config) []string {
	var lines []string
	for _, w := range cfg.Warnings {
		lines = append(lines, w.Error())
	}
	return lines
}

func logConfigWarnings(log *slog.Logger, cfg config.Config) {
	for _, line := range warningLines(cfg) {
		log.Warn("config", "warning", line)
	}
}

// errorLines splits a joined error into its parts, one per line.
func errorLines(err error) []string {
	if err == nil {
		return nil
	}
	var multi interface{ Unwrap() []error }
	if !errors.As(err, &multi) {
		return []string{err.Error()}
	}
	var lines []string
	for _, e := range multi.Unwrap() {
		lines = append(lines, e.Error())
	}
	return lines
}
