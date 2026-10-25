package wm

import (
	"os"
	"slices"
	"time"

	"github.com/grunclepug/stream/internal/layout"
	"github.com/grunclepug/stream/internal/state"
)

// restoreWait bounds how long restored windows wait for every output to
// report its name, so they can go back to the right one.
const restoreWait = 2 * time.Second

// persist holds what swm saves across restarts.
type persist struct {
	path string // "" when there's nowhere to save

	// saved is the state the previous swm left: windows and outputs are
	// restored from it as they appear.
	saved   *state.State
	started time.Time

	// outputs remembers every output seen, by name, including unplugged
	// ones, so a monitor that comes back gets its tags and layouts back.
	outputs map[string]*state.Output

	// reorder lists the outputs restored windows joined in this manage
	// sequence; their layout and focus order is put back afterwards.
	reorder []*Output
	// focus is the window to focus once it is restored.
	focus string

	written  []byte // what the file holds, so unchanged state isn't rewritten
	writeErr string // the last error writing it, logged once
}

func (wm *WM) loadState() {
	p := &wm.persist
	p.saved, p.outputs, p.started = state.New(), map[string]*state.Output{}, time.Now()
	path, err := state.Path()
	if err != nil {
		wm.log.Warn("state won't survive a restart: " + err.Error())
		return
	}
	p.path = path
	saved, err := state.Load(path)
	if err != nil {
		wm.log.Warn("ignoring saved state", "err", err)
	}
	p.saved, p.outputs, p.focus = saved, saved.Outputs, saved.Focused
}

// restoreOutput gives a newly named output the tags and layouts it had.
func (o *Output) restoreOutput() {
	so := o.wm.persist.outputs[o.name]
	if so == nil {
		return
	}
	if so.Tagset[0]&allTags != 0 && so.Tagset[1]&allTags != 0 && so.Seltags&^1 == 0 {
		o.tagset = [2]uint32{so.Tagset[0] & allTags, so.Tagset[1] & allTags}
		o.seltags = so.Seltags
	}
	o.gap = max(so.Gap, 0)
	for i, t := range so.Tags {
		if i >= len(o.pertag) {
			break
		}
		pt := &o.pertag[i]
		for j, name := range t.Layouts {
			if k, ok := layout.KindNamed(name); ok {
				pt.kinds[j] = k
			}
		}
		pt.sel = t.Sel & 1
		pt.nmaster = max(t.NMaster, 0)
		if t.MFact >= 0.05 && t.MFact <= 0.95 {
			pt.mfact = t.MFact
		}
	}
	if o.wm.persist.saved.FocusedOutput == o.name {
		if s := o.wm.primarySeat(); s != nil {
			s.output = o
		}
	}
}

// restoreWindow puts a window the previous swm managed back where it was.
// wait means it can't yet: its output hasn't been named.
func (wm *WM) restoreWindow(w *Window) (restored, wait bool) {
	p := &wm.persist
	sw := p.saved.Windows[w.id]
	if w.id == "" || sw == nil {
		return false, false
	}
	o := wm.outputNamed(sw.Output)
	if o == nil && wm.unnamedOutputs() && time.Since(p.started) < restoreWait {
		return false, true
	}
	delete(p.saved.Windows, w.id)

	w.floating, w.weight = sw.Floating, sw.Weight
	w.float = layout.Rect{X: sw.Float.X, Y: sw.Float.Y, W: sw.Float.W, H: sw.Float.H}
	if _, ok := wm.cfg.Scratchpad(sw.Scratch); ok && wm.scratchWindow(sw.Scratch) == nil {
		w.scratch = sw.Scratch
	}
	wm.raise(w)
	if o == nil {
		// Its monitor is gone: like a window from an unplugged monitor, it
		// goes onto the focused one's shown tags.
		if o = wm.focusedOutput(); o == nil {
			return true, false
		}
		o.attachBottom(w)
	} else {
		o.attachBottom(w)
		if sw.Tags&allTags != 0 || w.scratch != "" {
			w.tags = sw.Tags & allTags
		}
	}
	if !slices.Contains(p.reorder, o) {
		p.reorder = append(p.reorder, o)
	}
	if sw.Fullscreen {
		w.setFullscreen(true)
	}
	wm.log.Debug("window restored", "id", w.id, "app_id", w.appID, "output", o.name, "tags", w.tags)
	return true, false
}

func (wm *WM) unnamedOutputs() bool {
	for _, o := range wm.outputs {
		if o.name == "" {
			return true
		}
	}
	return false
}

// finishRestore puts the layout and focus order of outputs that got
// restored windows back as saved, with windows that are new to swm first,
// and focuses the window that had focus.
func (wm *WM) finishRestore() {
	p := &wm.persist
	for _, o := range p.reorder {
		so := p.outputs[o.name]
		if so == nil {
			continue
		}
		sortBy(o.clients, so.Clients)
		sortBy(o.focusOrder, so.FocusOrder)
	}
	p.reorder = p.reorder[:0]

	if p.focus == "" {
		return
	}
	s := wm.primarySeat()
	for _, w := range wm.windows {
		if w.id != p.focus || w.output == nil {
			continue
		}
		if s != nil && w.visible() {
			s.focus(w)
		}
		p.focus = ""
	}
}

// sortBy orders ws by where their identifiers are in order; windows not in
// it go first, in the order they had.
func sortBy(ws []*Window, order []string) {
	slices.SortStableFunc(ws, func(a, b *Window) int {
		return slices.Index(order, a.id) - slices.Index(order, b.id)
	})
}

// saveState writes the state file if anything in it changed.
func (wm *WM) saveState() {
	p := &wm.persist
	if p.path == "" {
		return
	}
	st := state.New()
	st.Outputs = p.outputs
	for _, o := range wm.outputs {
		if o.name != "" {
			st.Outputs[o.name] = o.snapshot()
		}
	}
	for _, w := range wm.windows {
		if w.id == "" {
			continue
		}
		if sw := p.saved.Windows[w.id]; sw != nil {
			st.Windows[w.id] = sw // still waiting to be restored
			continue
		}
		output := w.parkedOn
		if w.output != nil {
			output = w.output.name
		}
		if output == "" {
			continue
		}
		st.Windows[w.id] = &state.Window{
			Output:     output,
			Tags:       w.tags,
			Floating:   w.floating,
			Float:      state.Rect{X: w.float.X, Y: w.float.Y, W: w.float.W, H: w.float.H},
			Weight:     w.weight,
			Fullscreen: w.fullscreen,
			Scratch:    w.scratch,
		}
	}
	if s := wm.primarySeat(); s != nil {
		if s.focused != nil {
			st.Focused = s.focused.id
		}
		if s.output != nil {
			st.FocusedOutput = s.output.name
		}
	}

	data, err := st.Encode()
	if err == nil && string(data) == string(p.written) {
		return
	}
	if err == nil {
		err = state.Write(p.path, data)
	}
	if err != nil {
		if err.Error() != p.writeErr {
			wm.log.Warn("save state", "err", err)
			p.writeErr = err.Error()
		}
		return
	}
	p.written, p.writeErr = data, ""
}

// forgetState removes the state file: the session is over, and nothing in
// it will be seen again.
func (wm *WM) forgetState() {
	if wm.persist.path != "" {
		_ = os.Remove(wm.persist.path)
		wm.persist.path = ""
	}
}

func (o *Output) snapshot() *state.Output {
	so := &state.Output{Tagset: o.tagset, Seltags: o.seltags, Gap: o.gap}
	for _, t := range o.pertag {
		so.Tags = append(so.Tags, state.Tag{
			Layouts: [2]string{t.kinds[0].String(), t.kinds[1].String()},
			Sel:     t.sel,
			NMaster: t.nmaster,
			MFact:   t.mfact,
		})
	}
	for _, w := range o.clients {
		so.Clients = append(so.Clients, w.id)
	}
	for _, w := range o.focusOrder {
		so.FocusOrder = append(so.FocusOrder, w.id)
	}
	return so
}
