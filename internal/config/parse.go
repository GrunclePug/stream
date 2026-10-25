package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/grunclepug/stream/internal/command"
	"github.com/grunclepug/stream/internal/layout"
)

// maxIncludeDepth bounds nested includes, which also catches loops.
const maxIncludeDepth = 8

// maxErrors is how many errors a parse reports before giving up.
const maxErrors = 20

// Error is a problem on one line of a config file.
type Error struct {
	File string
	Line int
	Msg  string
}

func (e *Error) Error() string {
	if e.Line == 0 {
		return e.File + ": " + e.Msg // about the file as a whole
	}
	return fmt.Sprintf("%s:%d: %s", e.File, e.Line, e.Msg)
}

// Load reads the config file at path over the defaults. A missing file is not
// an error: swm runs on its defaults. Any error leaves the whole file
// unapplied, and lists every problem found, one per line.
func Load(path string) (Config, error) {
	cfg, err := Parse(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Default(), nil
	}
	return cfg, err
}

// Parse reads the config file at path over the defaults. Settings the file
// leaves out keep their defaults. If the file has any bindings, they replace
// all default bindings.
func Parse(path string) (Config, error) {
	p, err := parse(path)
	if err != nil {
		return Config{}, err
	}
	return p.cfg, nil
}

func parse(path string) (*parser, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	p := &parser{cfg: Default(), bindAt: map[string][]*Error{}}
	p.file(path, "", 0, 0)
	if len(p.errs) == 0 {
		p.finish()
	}
	if len(p.errs) > 0 {
		return nil, errors.Join(p.errs...)
	}
	return p, nil
}

type parser struct {
	cfg  Config
	errs []error

	// Bindings read so far. Once there is any, they replace the defaults.
	modes    []Mode
	bindings bool
	rules    []Rule

	scratchpads []Scratchpad
	scratchAt   []*Error // where each of scratchpads is defined
	// where each scratchpad command is used, checked once the file is read
	scratchUses []ref

	// where each mode was declared and where each mode command is used, for
	// errors found once the whole file is read
	declared map[string]*Error
	switches []ref
	// where each binding is, by mode, in the order of Mode.Bindings
	bindAt map[string][]*Error
}

// ref is a binding in mode from whose command names to, a mode or a
// scratchpad, at a place in the file.
type ref struct {
	from, to string
	at       *Error
}

func (p *parser) errorf(file string, line int, format string, args ...any) {
	if len(p.errs) == maxErrors {
		p.errs = append(p.errs, &Error{file, line, "too many errors"})
	}
	if len(p.errs) > maxErrors {
		return
	}
	p.errs = append(p.errs, &Error{file, line, fmt.Sprintf(format, args...)})
}

// file parses one file. from and fromLine are where it was included from.
func (p *parser) file(path, from string, fromLine, depth int) {
	f, err := os.Open(path)
	if err != nil {
		if from == "" {
			p.errs = append(p.errs, err)
		} else {
			p.errorf(from, fromLine, "include: %v", err)
		}
		return
	}
	defer f.Close()

	var (
		mode      *Mode // the mode block being read, if any
		modeStart int
		n         int
	)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		n++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		directive, rest := command.CutWord(line)

		if mode != nil {
			switch directive {
			case "}":
				if rest != "" {
					p.errorf(path, n, "unexpected %q after }", rest)
				}
				mode = nil
			case "bind":
				p.bind(path, n, mode.Name, rest)
			default:
				p.errorf(path, n, "only bind lines can go in a mode block, found %q", directive)
			}
			continue
		}

		switch directive {
		case "include":
			p.include(path, n, rest, depth)
		case "bind":
			p.bind(path, n, NormalMode, rest)
		case "scratchpad":
			if err := p.scratchpad(rest); err != nil {
				p.errorf(path, n, "scratchpad: %v", err)
			} else {
				p.scratchAt = append(p.scratchAt, &Error{File: path, Line: n})
			}
		case "rule":
			if err := p.rule(rest); err != nil {
				p.errorf(path, n, "rule: %v", err)
			}
		case "mode":
			if m := p.modeBlock(path, n, rest); m != "" {
				mode, modeStart = &Mode{Name: m}, n
			}
		case "}":
			p.errorf(path, n, "} without a mode block")
		default:
			if err := p.setting(directive, rest); err != nil {
				p.errorf(path, n, "%v", err)
			}
		}
	}
	if err := sc.Err(); err != nil {
		p.errorf(path, n, "read: %v", err)
	}
	if mode != nil {
		p.errorf(path, modeStart, "mode %s: missing }", mode.Name)
	}
}

func (p *parser) include(path string, line int, arg string, depth int) {
	optional := false
	if word, rest := command.CutWord(arg); strings.HasPrefix(word, "-") {
		if word != "-optional" {
			p.errorf(path, line, "include: unknown flag %q (the flag is -optional)", word)
			return
		}
		optional, arg = true, rest
	}
	if arg == "" {
		p.errorf(path, line, "include: missing path")
		return
	}
	if depth+1 >= maxIncludeDepth {
		p.errorf(path, line, "include: nested more than %d deep (an include loop?)", maxIncludeDepth)
		return
	}
	target, err := expandPath(arg, filepath.Dir(path))
	if err != nil {
		p.errorf(path, line, "include: %v", err)
		return
	}
	if _, err := os.Stat(target); optional && errors.Is(err, fs.ErrNotExist) {
		return // -optional: a missing file is fine, other problems aren't
	}
	p.file(target, path, line, depth+1)
}

// expandPath resolves ~/ to the home directory and relative paths against
// dir, the directory of the file that names the path.
func expandPath(path, dir string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[1:]), nil
	}
	if !filepath.IsAbs(path) {
		return filepath.Join(dir, path), nil
	}
	return path, nil
}

// modeBlock checks the opening line of a mode block, "mode <name> {", and
// returns the mode's name, or "" after reporting an error.
func (p *parser) modeBlock(path string, line int, rest string) string {
	name, brace := command.CutWord(rest)
	if brace != "{" {
		p.errorf(path, line, "mode blocks are written mode <name> {")
		return ""
	}
	switch {
	case !command.ValidName(name):
		p.errorf(path, line, "mode: %q is not a valid name (letters, digits, - and _)", name)
		return ""
	case name == NormalMode:
		p.errorf(path, line, "mode normal: top-level bind lines are the normal mode")
		return ""
	}
	if prev, ok := p.declared[name]; ok {
		p.errorf(path, line, "mode %s: already declared at %s:%d", name, prev.File, prev.Line)
		return ""
	}
	if p.declared == nil {
		p.declared = map[string]*Error{}
	}
	p.declared[name] = &Error{File: path, Line: line}
	p.bindings = true
	p.modeNamed(name)
	return name
}

// modeNamed returns the mode called name, adding it if it's new.
func (p *parser) modeNamed(name string) *Mode {
	if len(p.modes) == 0 {
		p.modes = append(p.modes, Mode{Name: NormalMode})
	}
	for i := range p.modes {
		if p.modes[i].Name == name {
			return &p.modes[i]
		}
	}
	p.modes = append(p.modes, Mode{Name: name})
	return &p.modes[len(p.modes)-1]
}

// bind parses "[-repeat] [-locked] [-also] <keys> <command>".
func (p *parser) bind(path string, line int, mode, rest string) {
	p.bindings = true
	var b Binding
	for {
		word, after := command.CutWord(rest)
		if !strings.HasPrefix(word, "-") {
			break
		}
		switch word {
		case "-repeat":
			b.Repeat = true
		case "-locked":
			b.Locked = true
		case "-also":
			b.Also = true
		default:
			p.errorf(path, line, "bind: unknown flag %q (flags are -repeat, -locked and -also)", word)
			return
		}
		rest = after
	}
	keys, text := command.CutWord(rest)
	if keys == "" {
		p.errorf(path, line, "bind: missing keys")
		return
	}
	k, err := parseKeys(keys)
	if err != nil {
		p.errorf(path, line, "bind: %v", err)
		return
	}
	cmd, err := command.Parse(text)
	if err != nil {
		p.errorf(path, line, "bind %s: %v", keys, err)
		return
	}
	b.Mods, b.Keysym, b.Button, b.Command = k.Mods, k.Keysym, k.Button, cmd
	switch {
	case b.Button != 0 && b.Repeat:
		p.errorf(path, line, "bind %s: -repeat only works on keys", keys)
		return
	case b.Button == 0 && cmd.Pointer():
		p.errorf(path, line, "bind %s: %s follows the pointer, so it needs a pointer button such as BTN_LEFT", keys, cmd.Name)
		return
	}

	m := p.modeNamed(mode)
	m.Bindings = append(m.Bindings, b)
	p.bindAt[mode] = append(p.bindAt[mode], &Error{File: path, Line: line})
	if cmd.Name == "mode" {
		p.switches = append(p.switches, ref{mode, cmd.Args[0], &Error{File: path, Line: line}})
	}
	if cmd.Name == "scratchpad" {
		p.scratchUses = append(p.scratchUses, ref{mode, cmd.Args[0], &Error{File: path, Line: line}})
	}
}

// rule parses "[-app-id <glob>] [-title <glob>] <action>...", where the
// actions are float, tile, tag <n> and monitor <name>.
func (p *parser) rule(rest string) error {
	words, err := splitQuoted(rest)
	if err != nil {
		return err
	}
	var r Rule
	value := func(i int) (string, error) {
		if i+1 >= len(words) {
			return "", fmt.Errorf("%s: missing value", words[i])
		}
		return words[i+1], nil
	}
	actions := 0
	for i := 0; i < len(words); i++ {
		w := words[i]
		switch w {
		case "-app-id", "-title", "tag", "monitor":
			v, err := value(i)
			if err != nil {
				return err
			}
			i++
			switch w {
			case "-app-id":
				r.AppID = v
			case "-title":
				r.Title = v
			case "tag":
				n, err := strconv.Atoi(v)
				if err != nil || n < 1 || n > command.NumTags {
					return fmt.Errorf("tag: %q is not a tag from 1 to %d", v, command.NumTags)
				}
				r.Tags |= 1 << (n - 1)
				actions++
			case "monitor":
				r.Output = v
				actions++
			}
		case "float", "tile":
			if r.Float != FloatDefault {
				return errors.New("use one of float and tile")
			}
			r.Float = FloatOn
			if w == "tile" {
				r.Float = FloatOff
			}
			actions++
		default:
			return fmt.Errorf("unknown word %q (rules are rule [-app-id <glob>] [-title <glob>] float|tile|tag <n>|monitor <name> ...)", w)
		}
	}
	switch {
	case r.AppID == "" && r.Title == "":
		return errors.New("needs -app-id or -title to match windows")
	case actions == 0:
		return errors.New("needs an action: float, tile, tag <n> or monitor <name>")
	}
	p.rules = append(p.rules, r)
	return nil
}

// scratchpad parses "<name> <app-id> <command>".
func (p *parser) scratchpad(rest string) error {
	name, rest := command.CutWord(rest)
	appID, cmd := command.CutWord(rest)
	switch {
	case !command.ValidName(name):
		return fmt.Errorf("%q is not a valid name (letters, digits, - and _); scratchpads are scratchpad <name> <app-id> <command>", name)
	case appID == "" || cmd == "":
		return errors.New("scratchpads are scratchpad <name> <app-id> <command>")
	}
	for _, sp := range p.scratchpads {
		switch {
		case sp.Name == name:
			return fmt.Errorf("%s is defined twice", name)
		case sp.AppID == appID:
			return fmt.Errorf("%s and %s have the same app-id %s", sp.Name, name, appID)
		}
	}
	p.scratchpads = append(p.scratchpads, Scratchpad{Name: name, AppID: appID, Command: cmd})
	return nil
}

// splitQuoted splits s at spaces, keeping "double-quoted words" together.
func splitQuoted(s string) ([]string, error) {
	var words []string
	for s = strings.TrimSpace(s); s != ""; s = strings.TrimSpace(s) {
		if s[0] != '"' {
			w, rest := command.CutWord(s)
			words, s = append(words, w), rest
			continue
		}
		end := strings.IndexByte(s[1:], '"')
		if end < 0 {
			return nil, errors.New("missing closing \"")
		}
		words, s = append(words, s[1:end+1]), s[end+2:]
	}
	return words, nil
}

// finish applies the bindings once the whole file is read, and checks what
// only the whole file can tell: that modes exist and can be left.
func (p *parser) finish() {
	if p.rules != nil {
		p.cfg.Rules = p.rules
	}
	if p.scratchpads != nil {
		p.cfg.Scratchpads = p.scratchpads
	}
	if !p.bindings {
		// The default bindings stay, so they must still find their
		// scratchpads.
		for _, b := range p.cfg.Modes[0].Bindings {
			if b.Command.Name != "scratchpad" {
				continue
			}
			if _, ok := p.cfg.Scratchpad(b.Command.Args[0]); !ok {
				p.errs = append(p.errs, fmt.Errorf("the default binding %s runs scratchpad %s, which this config no longer defines; define it or add bind lines", b.Keys(), b.Command.Args[0]))
			}
		}
		return
	}
	for _, u := range p.scratchUses {
		if _, ok := p.cfg.Scratchpad(u.to); !ok {
			p.errorf(u.at.File, u.at.Line, "scratchpad %s: no such scratchpad (define it with scratchpad %s <app-id> <command>)", u.to, u.to)
		}
	}
	known := func(name string) bool {
		return slices.ContainsFunc(p.modes, func(m Mode) bool { return m.Name == name })
	}
	for _, s := range p.switches {
		if !known(s.to) {
			p.errorf(s.at.File, s.at.Line, "mode %s: no such mode (declare it with mode %s { ... })", s.to, s.to)
		}
	}
	// Every mode needs a way back to normal, or entering it would leave the
	// keyboard stuck.
	back := map[string]bool{NormalMode: true}
	for changed := true; changed; {
		changed = false
		for _, s := range p.switches {
			if back[s.to] && !back[s.from] {
				back[s.from], changed = true, true
			}
		}
	}
	for _, m := range p.modes {
		if !back[m.Name] {
			at := p.declared[m.Name]
			p.errorf(at.File, at.Line, "mode %s: no binding leads back to mode normal", m.Name)
		}
	}
	p.cfg.Modes = p.modes
	p.duplicates()
}

// duplicates warns about keys bound more than once in a mode, unless the
// later bindings say -also. Both commands run, so it may be meant.
func (p *parser) duplicates() {
	for _, m := range p.cfg.Modes {
		at := p.bindAt[m.Name]
		for i, b := range m.Bindings {
			if b.Also {
				continue
			}
			for j, earlier := range m.Bindings[:i] {
				if !SameKey(earlier, b, p.cfg.Mod) {
					continue
				}
				first := at[j]
				where := fmt.Sprintf("line %d", first.Line)
				if first.File != at[i].File {
					where = fmt.Sprintf("%s:%d", first.File, first.Line)
				}
				p.cfg.Warnings = append(p.cfg.Warnings, &Error{at[i].File, at[i].Line, fmt.Sprintf(
					"bind %s: the key also runs %s (%s); both run, in order. Add -also to this binding if that's meant",
					b.Keys(), earlier.Command, where)})
				break
			}
		}
	}
}

// setting parses one "<name> <value>" line.
func (p *parser) setting(name, value string) error {
	set, ok := settings[name]
	if !ok {
		return fmt.Errorf("unknown setting %q", name)
	}
	if value == "" {
		return fmt.Errorf("%s: missing value", name)
	}
	if err := set(&p.cfg, value); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

var settings = map[string]func(c *Config, v string) error{
	"border-width": func(c *Config, v string) error {
		n, err := parseInt(v, 0, 100)
		c.BorderWidth = int32(n)
		return err
	},
	"border-focused": func(c *Config, v string) (err error) {
		c.BorderFocused, err = parseColor(v)
		return err
	},
	"border-unfocused": func(c *Config, v string) (err error) {
		c.BorderUnfocused, err = parseColor(v)
		return err
	},
	"layout": func(c *Config, v string) error {
		k, ok := layout.KindNamed(v)
		if !ok {
			return fmt.Errorf("%q is not tile, monocle or floating", v)
		}
		c.Layout = k
		return nil
	},
	"nmaster": func(c *Config, v string) (err error) {
		c.NMaster, err = parseInt(v, 0, 100)
		return err
	},
	"mfact": func(c *Config, v string) error {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f < 0.05 || f > 0.95 {
			return fmt.Errorf("%q is not a number from 0.05 to 0.95", v)
		}
		c.MFact = f
		return nil
	},
	"gap": func(c *Config, v string) error {
		n, err := parseInt(v, 0, 1000)
		c.Gap = int32(n)
		return err
	},
	"mod": func(c *Config, v string) error {
		m, ok := modifierFromName(v)
		if !ok || m == Mod {
			return fmt.Errorf("%q is not a modifier (super, alt, ctrl, shift, mod3, mod5)", v)
		}
		c.Mod = m
		return nil
	},
	"repeat-delay": func(c *Config, v string) (err error) {
		c.RepeatDelay, err = parseInt(v, 1, 10000)
		return err
	},
	"repeat-rate": func(c *Config, v string) (err error) {
		c.RepeatRate, err = parseInt(v, 1, 1000)
		return err
	},
}

func parseInt(v string, lo, hi int) (int, error) {
	n, err := strconv.Atoi(v)
	if err != nil || n < lo || n > hi {
		return 0, fmt.Errorf("%q is not a whole number from %d to %d", v, lo, hi)
	}
	return n, nil
}

// parseColor parses #rrggbb or #rrggbbaa.
func parseColor(v string) (Color, error) {
	hex, ok := strings.CutPrefix(v, "#")
	if ok && (len(hex) == 6 || len(hex) == 8) {
		if n, err := strconv.ParseUint(hex, 16, 32); err == nil {
			if len(hex) == 6 {
				n = n<<8 | 0xff
			}
			return Color(n), nil
		}
	}
	return 0, fmt.Errorf("%q is not a colour (#rrggbb or #rrggbbaa)", v)
}
