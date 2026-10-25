package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/grunclepug/stream/internal/config"
)

const genConfigUsage = `usage: streamctl gen-config [-force] [-o [path]]
       streamctl gen-config -missing [-o path]
       streamctl gen-config -check [-o path]

Write swm's built-in defaults as a config file, to the path swm reads
($XDG_CONFIG_HOME/stream/config). An existing file is left alone unless
-force is given.

  -o path   write to path instead; a directory gets a file named config
  -o        write to standard output
  -force    overwrite an existing file
  -missing  print the default bindings the config lacks, as bind lines to
            paste into it
  -check    report the config's errors and warnings, such as a key bound
            twice or a program that isn't installed; exit 1 on errors
`

// genConfig runs streamctl gen-config. Its flags are parsed by hand: the
// flag package has no flag that takes an optional value, like -o here.
func genConfig(args []string, stdout io.Writer) error {
	var (
		force    bool
		missing  bool
		check    bool
		out      string
		toStdout bool
	)
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "-force", "--force":
			force = true
		case "-missing", "--missing":
			missing = true
		case "-check", "--check":
			check = true
		case "-o", "--o":
			if i+1 < len(args) && args[i+1] != "" && args[i+1][0] != '-' {
				out = args[i+1]
				i++
			} else {
				toStdout = true
			}
		case "-h", "-help", "--help":
			fmt.Fprint(stdout, genConfigUsage)
			return nil
		default:
			return usageError{fmt.Sprintf("gen-config: unexpected argument %q", a), genConfigUsage}
		}
	}

	if (missing || check) && (force || toStdout || (missing && check)) {
		return usageError{"gen-config: -missing and -check take only -o path", genConfigUsage}
	}

	data := config.Generate(config.Default())
	if toStdout {
		_, err := stdout.Write(data)
		return err
	}

	path := out
	if path == "" {
		p, err := config.Path()
		if err != nil {
			return err
		}
		path = p
	} else if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		path = filepath.Join(path, "config")
	}

	switch {
	case missing:
		return listMissing(path, stdout)
	case check:
		return checkConfig(path, stdout)
	}
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists; use -force to overwrite it", path)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "wrote", path)
	return nil
}

// usageError is a mistake in the arguments: it is printed with the usage.
type usageError struct {
	msg, usage string
}

func (e usageError) Error() string { return e.msg }

// checkConfig prints the problems with the config at path. Errors make it
// fail; warnings don't.
func checkConfig(path string, out io.Writer) error {
	warnings, err := config.Check(path)
	if errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(out, "%s does not exist; swm runs on its defaults.\n", path)
		return nil
	}
	if err != nil {
		lines := strings.Split(err.Error(), "\n")
		for _, l := range lines {
			fmt.Fprintln(out, "error:", l)
		}
		return fmt.Errorf("%s has %s; swm won't load it", path, plural(len(lines), "error"))
	}
	for _, w := range warnings {
		fmt.Fprintln(out, "warning:", w)
	}
	if len(warnings) == 0 {
		fmt.Fprintf(out, "%s: no problems found.\n", path)
	} else {
		fmt.Fprintf(out, "%s: %s; swm loads it.\n", path, plural(len(warnings), "warning"))
	}
	return nil
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
