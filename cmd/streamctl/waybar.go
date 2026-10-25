package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/grunclepug/stream/internal/command"
	"github.com/grunclepug/stream/internal/ipc"
)

const waybarUsage = `usage: streamctl waybar <module>

Print one waybar custom module's output, as JSON, one line per change.
Use it as the module's exec, with "return-type": "json". Modules:

  tag <n>   tag n, with the class tag and focused, occupied or empty
  layout    the layout, with the master count when it isn't 1
  mode      the binding mode, empty in the normal mode
  title     the focused window's title
  config    a notice of config errors or warnings, details in the tooltip

The monitor is the one in WAYBAR_OUTPUT_NAME, which waybar sets for each
bar, or the focused one. Like watch, it waits for swm while it's gone.
`

// state is the part of streamctl watch's output the modules use.
type state struct {
	Mode           string        `json:"mode"`
	Outputs        []outputState `json:"outputs"`
	ConfigErrors   []string      `json:"config_errors"`
	ConfigWarnings []string      `json:"config_warnings"`
}

type outputState struct {
	Name     string `json:"name"`
	Focused  bool   `json:"focused"`
	Layout   string `json:"layout"`
	NMaster  int    `json:"nmaster"`
	Tags     []int  `json:"tags"`
	Occupied []int  `json:"occupied"`
	Title    string `json:"title"`
}

// module is what waybar reads: its text, CSS classes and tooltip.
type module struct {
	Text    string   `json:"text"`
	Class   []string `json:"class,omitempty"`
	Tooltip string   `json:"tooltip,omitempty"`
}

// render turns a state into one module's output for the named output.
type render func(st state, output string) module

func waybarModule(args []string) (render, error) {
	if len(args) == 0 {
		return nil, usageError{"waybar needs a module", waybarUsage}
	}
	name, rest := args[0], args[1:]
	if name == "tag" {
		if len(rest) != 1 {
			return nil, usageError{"waybar tag needs a tag number", waybarUsage}
		}
		n, err := strconv.Atoi(rest[0])
		if err != nil || n < 1 || n > command.NumTags {
			return nil, usageError{fmt.Sprintf("%q is not a tag from 1 to %d", rest[0], command.NumTags), waybarUsage}
		}
		return tagModule(n), nil
	}
	if len(rest) > 0 {
		return nil, usageError{fmt.Sprintf("waybar %s takes no arguments", name), waybarUsage}
	}
	switch name {
	case "layout":
		return func(st state, output string) module {
			o, ok := st.output(output)
			if !ok {
				return module{}
			}
			if o.NMaster == 1 {
				return module{Text: o.Layout}
			}
			return module{Text: fmt.Sprintf("%s %d", o.Layout, o.NMaster)}
		}, nil
	case "mode":
		return func(st state, _ string) module {
			if st.Mode == "normal" {
				return module{}
			}
			return module{Text: st.Mode}
		}, nil
	case "title":
		return func(st state, output string) module {
			o, _ := st.output(output)
			return module{Text: o.Title}
		}, nil
	case "config":
		return func(st state, _ string) module {
			details := strings.Join(slices.Concat(st.ConfigErrors, st.ConfigWarnings), "\n")
			switch {
			case len(st.ConfigErrors) > 0:
				return module{Text: "config error", Class: []string{"error"}, Tooltip: details}
			case len(st.ConfigWarnings) > 0:
				return module{Text: "config warning", Class: []string{"warning"}, Tooltip: details}
			}
			return module{}
		}, nil
	}
	return nil, usageError{fmt.Sprintf("unknown waybar module %q", name), waybarUsage}
}

func tagModule(n int) render {
	text := strconv.Itoa(n)
	return func(st state, output string) module {
		o, _ := st.output(output)
		class := "empty"
		switch {
		case slices.Contains(o.Tags, n):
			class = "focused"
		case slices.Contains(o.Occupied, n):
			class = "occupied"
		}
		return module{Text: text, Class: []string{"tag", class}}
	}
}

// output returns the output called name, or the focused one when name is
// empty or not connected.
func (st state) output(name string) (outputState, bool) {
	for _, out := range st.Outputs {
		if out.Name == name {
			return out, true
		}
	}
	for _, out := range st.Outputs {
		if out.Focused {
			return out, true
		}
	}
	return outputState{}, false
}

// waybarWriter takes watch's state lines and writes the module's output,
// only when it changes, so waybar doesn't redraw for nothing.
type waybarWriter struct {
	out    io.Writer
	render render
	output string
	last   []byte
}

func (w *waybarWriter) Write(line []byte) (int, error) {
	var st state
	if err := json.Unmarshal(line, &st); err != nil {
		return 0, fmt.Errorf("read swm's state: %w", err)
	}
	b, err := json.Marshal(w.render(st, w.output))
	if err != nil {
		return 0, err
	}
	b = append(b, '\n')
	if !bytes.Equal(b, w.last) {
		if _, err := w.out.Write(b); err != nil {
			return 0, err
		}
		w.last = b
	}
	return len(line), nil
}

func runWaybar(ctx context.Context, path string, args []string, out io.Writer) error {
	r, err := waybarModule(args)
	if err != nil {
		return err
	}
	// End with the process that started it, even when that's a shell waybar
	// stops on reload without stopping its children.
	if _, _, errno := syscall.RawSyscall(syscall.SYS_PRCTL, syscall.PR_SET_PDEATHSIG, uintptr(syscall.SIGTERM), 0); errno != 0 {
		return fmt.Errorf("prctl: %w", errno)
	}
	if os.Getppid() == 1 {
		return nil // the parent is already gone
	}
	w := &waybarWriter{out: out, render: r, output: os.Getenv("WAYBAR_OUTPUT_NAME")}
	return ipc.WatchState(ctx, path, w, ipc.SessionAlive)
}
