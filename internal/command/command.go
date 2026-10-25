// Package command is the command language shared by key bindings and
// streamctl: "view 3", "mfact +0.05", "spawn foot".
package command

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// Command is a validated command. Args are normalised, so two commands that
// do the same thing compare equal.
type Command struct {
	Name string
	Args []string
}

// String returns the command as it is written in the config.
func (c Command) String() string {
	return strings.Join(append([]string{c.Name}, c.Args...), " ")
}

// Pointer reports whether the command only works on a pointer binding,
// because it follows the pointer until the button is released.
func (c Command) Pointer() bool {
	return c.Name == "move" || c.Name == "resize"
}

// spec checks and normalises one command's arguments.
type spec func(args []string) ([]string, error)

var specs = map[string]spec{
	"spawn":             nil, // handled in Parse: the rest of the line, verbatim
	"close":             none,
	"quit":              none,
	"reload":            none,
	"zoom":              none,
	"toggle-float":      none,
	"toggle-fullscreen": none,
	"reset":             optional("mfact", "weights"),
	"move":              none,
	"resize":            none,
	"focus":             oneOf("next", "prev"),
	"swap":              oneOf("next", "prev"),
	"focus-mon":         oneOf("next", "prev"),
	"tag-mon":           oneOf("next", "prev"),
	"view":              tags("all", "prev"),
	"tag":               tags("all"),
	"toggle-view":       tags(),
	"toggle-tag":        tags("all"),
	"layout":            oneOf("tile", "monocle", "floating", "prev"),
	"mfact":             mfact,
	"nmaster":           integer(0),
	"gap":               gap,
	"mode":              name,
	"scratchpad":        name,
}

// Names returns every command name, sorted.
func Names() []string {
	return slices.Sorted(maps.Keys(specs))
}

// Parse parses a command from its text, for example "mfact +0.05". The
// arguments of spawn are the rest of the text, kept as written: swm runs
// them with sh -c.
func Parse(text string) (Command, error) {
	name, rest := CutWord(text)
	if name == "" {
		return Command{}, errors.New("missing command")
	}
	s, ok := specs[name]
	if !ok {
		return Command{}, fmt.Errorf("unknown command %q", name)
	}
	if name == "spawn" {
		if rest == "" {
			return Command{}, errors.New("spawn: missing program")
		}
		return Command{Name: name, Args: []string{rest}}, nil
	}
	args, err := s(strings.Fields(rest))
	if err != nil {
		return Command{}, fmt.Errorf("%s: %w", name, err)
	}
	return Command{Name: name, Args: args}, nil
}

// CutWord splits s into its first word and the rest, at the first run of
// spaces or tabs. Both come back without surrounding space; the rest keeps
// any spacing inside it.
func CutWord(s string) (word, rest string) {
	s = strings.TrimSpace(s)
	i := strings.IndexAny(s, " \t")
	if i < 0 {
		return s, ""
	}
	return s[:i], strings.TrimSpace(s[i:])
}

func none(args []string) ([]string, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("takes no arguments, got %q", strings.Join(args, " "))
	}
	return nil, nil
}

func one(args []string) (string, error) {
	switch len(args) {
	case 0:
		return "", errors.New("missing argument")
	case 1:
		return args[0], nil
	}
	return "", fmt.Errorf("takes one argument, got %q", strings.Join(args, " "))
}

func oneOf(words ...string) spec {
	return func(args []string) ([]string, error) {
		a, err := one(args)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(words, a) {
			return nil, fmt.Errorf("%q is not one of %s", a, strings.Join(words, ", "))
		}
		return []string{a}, nil
	}
}

// optional accepts no argument or one of words.
func optional(words ...string) spec {
	return func(args []string) ([]string, error) {
		if len(args) == 0 {
			return nil, nil
		}
		return oneOf(words...)(args)
	}
}

// NumTags is how many tags each monitor has.
const NumTags = 9

// tags accepts a tag number from 1 to NumTags, or one of words.
func tags(words ...string) spec {
	return func(args []string) ([]string, error) {
		a, err := one(args)
		if err != nil {
			return nil, err
		}
		if slices.Contains(words, a) {
			return []string{a}, nil
		}
		n, err := strconv.Atoi(a)
		if err != nil || n < 1 || n > NumTags {
			if len(words) > 0 {
				return nil, fmt.Errorf("%q is not a tag from 1 to %d or one of %s", a, NumTags, strings.Join(words, ", "))
			}
			return nil, fmt.Errorf("%q is not a tag from 1 to %d", a, NumTags)
		}
		return []string{strconv.Itoa(n)}, nil
	}
}

// Relative reports whether a numeric argument is a change (+1, -0.05) rather
// than a new value.
func Relative(arg string) bool {
	return strings.HasPrefix(arg, "+") || strings.HasPrefix(arg, "-")
}

func mfact(args []string) ([]string, error) {
	a, err := one(args)
	if err != nil {
		return nil, err
	}
	f, err := strconv.ParseFloat(a, 64)
	if err != nil {
		return nil, fmt.Errorf("%q is not a number", a)
	}
	if !Relative(a) && (f < 0.05 || f > 0.95) {
		return nil, fmt.Errorf("%s is outside 0.05 to 0.95", a)
	}
	return []string{formatFloat(f, Relative(a))}, nil
}

func formatFloat(f float64, signed bool) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if signed && f >= 0 {
		s = "+" + s
	}
	return s
}

func integer(min int) spec {
	return func(args []string) ([]string, error) {
		a, err := one(args)
		if err != nil {
			return nil, err
		}
		n, err := strconv.Atoi(a)
		if err != nil {
			return nil, fmt.Errorf("%q is not a whole number", a)
		}
		if !Relative(a) && n < min {
			return nil, fmt.Errorf("%d is below %d", n, min)
		}
		s := strconv.Itoa(n)
		if Relative(a) && n >= 0 {
			s = "+" + s
		}
		return []string{s}, nil
	}
}

func gap(args []string) ([]string, error) {
	if len(args) == 1 && args[0] == "reset" {
		return args, nil
	}
	return integer(0)(args)
}

func name(args []string) ([]string, error) {
	a, err := one(args)
	if err != nil {
		return nil, err
	}
	if !ValidName(a) {
		return nil, fmt.Errorf("%q is not a valid name (letters, digits, - and _)", a)
	}
	return []string{a}, nil
}

// ValidName reports whether s can name a mode or a scratchpad.
func ValidName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
