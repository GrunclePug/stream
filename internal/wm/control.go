package wm

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/bits"

	"github.com/grunclepug/stream/internal/command"
)

// runRequests runs the commands streamctl sent since the last manage
// sequence, as the primary seat, and answers each one.
func (wm *WM) runRequests() {
	if wm.ipc == nil {
		return
	}
	for _, r := range wm.ipc.Take() {
		err := wm.runLine(r.Line)
		var warnings []string
		if c, _ := command.Parse(r.Line); err == nil && c.Name == "reload" {
			warnings = warningLines(wm.cfg)
		}
		r.Reply(warnings, err)
	}
}

func (wm *WM) runLine(line string) error {
	c, err := command.Parse(line)
	if err != nil {
		return err
	}
	s := wm.primarySeat()
	switch {
	case c.Pointer():
		return fmt.Errorf("%s only works on a pointer binding", c.Name)
	case c.Name == "mode" && !wm.hasMode(c.Args[0]):
		return fmt.Errorf("no mode named %s", c.Args[0])
	case c.Name == "reload":
		return wm.reload()
	case s == nil:
		return errors.New("river has no seat")
	}
	wm.exec(s, c, nil)
	return nil
}

// watchVersion is the version of the watch format. It changes only when a
// field is removed or changes meaning; new fields can appear without it.
const watchVersion = 1

// watchState is what streamctl watch prints, one JSON object per change.
type watchState struct {
	Version        int           `json:"version"`
	Mode           string        `json:"mode"`
	Outputs        []outputState `json:"outputs"`
	ConfigErrors   []string      `json:"config_errors"`
	ConfigWarnings []string      `json:"config_warnings"`
}

type outputState struct {
	Name     string `json:"name"`
	Focused  bool   `json:"focused"`
	Layout   string `json:"layout"`
	NMaster  int    `json:"nmaster"`  // of the shown tags, even when fewer windows tile
	Tags     []int  `json:"tags"`     // the tags shown
	Occupied []int  `json:"occupied"` // the tags that have windows
	Title    string `json:"title"`    // of the output's focused window
	AppID    string `json:"app_id"`
}

// publish sends the state to streamctl watch clients, if it changed.
func (wm *WM) publish() {
	if wm.ipc == nil {
		return
	}
	st := watchState{Version: watchVersion, Mode: wm.mode, Outputs: []outputState{}, ConfigErrors: wm.cfgErrors}
	st.ConfigWarnings = warningLines(wm.cfg)
	if st.ConfigWarnings == nil {
		st.ConfigWarnings = []string{}
	}
	if st.ConfigErrors == nil {
		st.ConfigErrors = []string{}
	}
	s := wm.primarySeat()
	for _, o := range wm.leftToRight() {
		out := outputState{
			Name:     o.name,
			Layout:   o.params().Kind.String(),
			NMaster:  o.lay().nmaster,
			Tags:     tagList(o.tags()),
			Occupied: []int{},
		}
		var occupied uint32
		for _, w := range o.clients {
			occupied |= w.tags
		}
		out.Occupied = tagList(occupied)
		focused := o.lastFocused()
		if s != nil && s.output == o {
			out.Focused = true
			focused = s.focused
		}
		if focused != nil {
			out.Title, out.AppID = focused.title, focused.appID
		}
		st.Outputs = append(st.Outputs, out)
	}
	data, err := json.Marshal(st)
	if err != nil {
		wm.log.Error("encode state", "err", err)
		return
	}
	wm.ipc.Publish(data)
}

// tagList turns a tag mask into tag numbers, lowest first.
func tagList(mask uint32) []int {
	tags := []int{}
	for mask &= allTags; mask != 0; {
		i := bits.TrailingZeros32(mask)
		tags = append(tags, i+1)
		mask &^= 1 << i
	}
	return tags
}
