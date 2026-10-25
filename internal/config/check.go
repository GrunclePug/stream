package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Check parses the config file at path and returns its warnings, plus one
// for each program a binding or scratchpad starts that isn't installed. swm
// doesn't check programs itself: it may start before PATH is complete.
func Check(path string) ([]*Error, error) {
	p, err := parse(path)
	if err != nil {
		return nil, err
	}
	warnings := p.cfg.Warnings
	for _, m := range p.cfg.Modes {
		for i, b := range m.Bindings {
			if b.Command.Name != "spawn" {
				continue
			}
			prog := missingProgram(b.Command.Args[0])
			switch {
			case prog == "":
			case p.bindings:
				at := p.bindAt[m.Name][i]
				warnings = append(warnings, &Error{at.File, at.Line, fmt.Sprintf("bind %s: %s is not installed (not found on PATH)", b.Keys(), prog)})
			default:
				warnings = append(warnings, &Error{File: path, Msg: fmt.Sprintf("the default binding %s runs %s, which is not installed (not found on PATH)", b.Keys(), prog)})
			}
		}
	}
	for i, sp := range p.cfg.Scratchpads {
		prog := missingProgram(sp.Command)
		switch {
		case prog == "":
		case p.scratchpads != nil:
			at := p.scratchAt[i]
			warnings = append(warnings, &Error{at.File, at.Line, fmt.Sprintf("scratchpad %s: %s is not installed (not found on PATH)", sp.Name, prog)})
		default:
			warnings = append(warnings, &Error{File: path, Msg: fmt.Sprintf("the default scratchpad %s runs %s, which is not installed (not found on PATH)", sp.Name, prog)})
		}
	}
	return warnings, nil
}

// missingProgram returns the program a shell command line runs if it can't
// be found, or "". Command lines whose program isn't a plain first word,
// such as ones starting with a variable, aren't checked.
func missingProgram(cmdline string) string {
	words := strings.Fields(cmdline)
	if len(words) > 0 && words[0] == "exec" {
		words = words[1:]
	}
	if len(words) == 0 || strings.ContainsAny(words[0], "$`'\"=;|&<>(){}*?[]\\") {
		return ""
	}
	prog := words[0]
	if prog == "~" || strings.HasPrefix(prog, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		prog = filepath.Join(home, prog[1:])
	}
	if _, err := exec.LookPath(prog); err != nil {
		return words[0]
	}
	return ""
}
