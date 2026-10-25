package config

import (
	"errors"
	"fmt"
	"slices"
)

// ErrDefaultBindings is what FindMissing returns for a config file with no
// bind lines: swm uses every default binding, so none is missing.
var ErrDefaultBindings = errors.New("the config has no bind lines, so swm uses all the default bindings")

// Missing is a default binding that a config file lacks: no binding in its
// mode runs the same command, arguments included.
type Missing struct {
	Mode    string
	Binding Binding
	NewMode bool   // the file has no such mode, so the binding needs a new mode block
	Note    string // why it won't work as it is, such as its key being taken
}

// FindMissing returns the default bindings the config file at path lacks,
// in the order of the defaults. The file has to parse.
func FindMissing(path string) ([]Missing, error) {
	p, err := parse(path)
	if err != nil {
		return nil, err
	}
	if !p.bindings {
		return nil, ErrDefaultBindings
	}
	cfg := p.cfg
	var ms []Missing
	for _, dm := range Default().Modes {
		i := slices.IndexFunc(cfg.Modes, func(m Mode) bool { return m.Name == dm.Name })
		var have []Binding
		if i >= 0 {
			have = cfg.Modes[i].Bindings
		}
		for _, b := range dm.Bindings {
			cmd := b.Command.String()
			if slices.ContainsFunc(have, func(o Binding) bool { return o.Command.String() == cmd }) {
				continue
			}
			m := Missing{Mode: dm.Name, Binding: b, NewMode: i < 0}
			if j := slices.IndexFunc(have, func(o Binding) bool { return SameKey(o, b, cfg.Mod) }); j >= 0 {
				m.Note = fmt.Sprintf("%s is taken by %s: pick another key", b.Keys(), have[j].Command)
			} else if b.Command.Name == "scratchpad" {
				if _, ok := cfg.Scratchpad(b.Command.Args[0]); !ok {
					m.Note = fmt.Sprintf("needs scratchpad %s: define it", b.Command.Args[0])
				}
			}
			ms = append(ms, m)
		}
	}
	return ms, nil
}
