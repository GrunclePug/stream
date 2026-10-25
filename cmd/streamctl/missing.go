package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"

	"github.com/grunclepug/stream/internal/config"
)

// listMissing prints the default bindings the config at path lacks, as bind
// lines ready to paste into it.
func listMissing(path string, out io.Writer) error {
	ms, err := config.FindMissing(path)
	switch {
	case errors.Is(err, config.ErrDefaultBindings):
		fmt.Fprintf(out, "%s: %v.\n", path, err)
		return nil
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%s does not exist; streamctl gen-config writes one", path)
	case err != nil:
		return fmt.Errorf("fix the config first:\n%w", err)
	case len(ms) == 0:
		fmt.Fprintf(out, "%s has every default binding.\n", path)
		return nil
	}

	fmt.Fprintf(out, "# Default bindings missing from %s.\n", path)
	fmt.Fprintln(out, "# Paste the ones you want into it, then reload.")
	mode, open := "", false
	for _, m := range ms {
		if m.Mode != mode {
			if open {
				fmt.Fprintln(out, "}")
			}
			mode, open = m.Mode, false
			fmt.Fprintln(out)
			switch {
			case mode == config.NormalMode:
			case m.NewMode:
				fmt.Fprintf(out, "mode %s {\n", mode)
				open = true
			default:
				fmt.Fprintf(out, "# Inside your mode %s block:\n", mode)
			}
		}
		indent := ""
		if mode != config.NormalMode {
			indent = "\t"
		}
		if m.Note != "" {
			fmt.Fprintf(out, "%s# %s\n", indent, m.Note)
		}
		fmt.Fprintf(out, "%s%s\n", indent, m.Binding.Line())
	}
	if open {
		fmt.Fprintln(out, "}")
	}
	return nil
}
