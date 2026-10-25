// Package ipc is how streamctl talks to a running swm: a Unix socket in
// $XDG_RUNTIME_DIR, one per river session.
//
// The protocol is lines of text. A client sends one line, a command such as
// "view 3", and gets back any warnings, one "warning: <message>" line each,
// then "ok" or "error: <message>". A client that
// sends "watch" instead gets swm's state as a JSON object, on one line, now
// and after every change, until it disconnects.
package ipc

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Watch is the line that turns a connection into a state stream.
const Watch = "watch"

// replyTimeout bounds how long a command waits for swm to run it.
const replyTimeout = 5 * time.Second

// watchBuffer is how many state updates a watcher may fall behind before it
// is disconnected, so a stuck client can't hold swm up.
const watchBuffer = 64

// watchRetry is how often a watcher tries to reach swm while it is gone.
const watchRetry = 250 * time.Millisecond

// SocketPath returns the socket of the river session this process runs in,
// named after its Wayland display.
func SocketPath() (string, error) {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		return "", errors.New("XDG_RUNTIME_DIR is not set")
	}
	display := os.Getenv("WAYLAND_DISPLAY")
	if display == "" {
		return "", errors.New("WAYLAND_DISPLAY is not set; is this running under river?")
	}
	return filepath.Join(dir, "stream-"+filepath.Base(display)+".sock"), nil
}

// Request is a command from a client, waiting for swm to run it.
type Request struct {
	Line  string
	reply chan string
}

// Reply answers the request: a nil err means it worked, with warnings
// about how, if any.
func (r Request) Reply(warnings []string, err error) {
	var b strings.Builder
	for _, w := range warnings {
		b.WriteString("warning: " + oneLine(w) + "\n")
	}
	if err != nil {
		b.WriteString("error: " + oneLine(err.Error()))
	} else {
		b.WriteString("ok")
	}
	r.reply <- b.String() // buffered: never blocks
}

func oneLine(s string) string { return strings.ReplaceAll(s, "\n", "; ") }

// Server accepts clients. Commands wait in an inbox until the window
// manager takes them, which it does on its own goroutine.
type Server struct {
	ln   net.Listener
	path string
	poke func() // asks the window manager to take the inbox

	mu       sync.Mutex
	closed   bool
	inbox    []Request
	watchers map[chan []byte]struct{}
	state    []byte // the last state published, for new watchers

	wg sync.WaitGroup
}

// Listen creates the socket at path. A socket left by a swm that is no
// longer running is replaced. poke is called, from another goroutine, when
// requests arrive.
func Listen(path string, poke func()) (*Server, error) {
	if c, err := net.Dial("unix", path); err == nil {
		c.Close()
		return nil, fmt.Errorf("%s is in use by another swm", path)
	}
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	return &Server{ln: ln, path: path, poke: poke, watchers: map[chan []byte]struct{}{}}, nil
}

// Serve accepts clients until Close.
func (s *Server) Serve() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			c, err := s.ln.Accept()
			if err != nil {
				return // closed
			}
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				s.handle(c)
			}()
		}
	}()
}

// Close stops accepting, disconnects watchers, removes the socket and
// waits for every client goroutine to finish.
func (s *Server) Close() {
	s.ln.Close()
	s.mu.Lock()
	s.closed = true
	for ch := range s.watchers {
		close(ch)
		delete(s.watchers, ch)
	}
	// Commands nobody will run get an answer, so their clients return.
	for _, r := range s.inbox {
		r.Reply(nil, errors.New("swm is stopping"))
	}
	s.inbox = nil
	s.mu.Unlock()
	s.wg.Wait()
	_ = os.Remove(s.path)
}

// Take returns the requests that arrived since the last call.
func (s *Server) Take() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	reqs := s.inbox
	s.inbox = nil
	return reqs
}

// Publish sends a new state to every watcher, unless it is the same as the
// last one.
func (s *Server) Publish(state []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if string(state) == string(s.state) {
		return
	}
	s.state = state
	for ch := range s.watchers {
		select {
		case ch <- state:
		default: // too far behind: drop it
			close(ch)
			delete(s.watchers, ch)
		}
	}
}

func (s *Server) handle(c net.Conn) {
	defer c.Close()
	_ = c.SetReadDeadline(time.Now().Add(replyTimeout))
	line, err := bufio.NewReader(c).ReadString('\n')
	if err != nil && (err != io.EOF || line == "") {
		return
	}
	line = strings.TrimSpace(line)
	if line == Watch {
		s.watch(c)
		return
	}

	r := Request{Line: line, reply: make(chan string, 1)}
	s.mu.Lock()
	if s.closed {
		r.Reply(nil, errors.New("swm is stopping"))
	} else {
		s.inbox = append(s.inbox, r)
	}
	s.mu.Unlock()
	s.poke()

	var reply string
	select {
	case reply = <-r.reply:
	case <-time.After(replyTimeout):
		reply = "error: swm did not answer"
	}
	_ = c.SetWriteDeadline(time.Now().Add(replyTimeout))
	fmt.Fprintln(c, reply)
}

func (s *Server) watch(c net.Conn) {
	ch := make(chan []byte, watchBuffer)
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	if s.state != nil {
		ch <- s.state
	}
	s.watchers[ch] = struct{}{}
	s.mu.Unlock()

	// The client sends nothing more; a read returning means it went away.
	gone := make(chan struct{})
	go func() {
		_ = c.SetReadDeadline(time.Time{})
		_, _ = io.Copy(io.Discard, c)
		close(gone)
	}()
	defer func() {
		s.mu.Lock()
		if _, ok := s.watchers[ch]; ok {
			close(ch)
			delete(s.watchers, ch)
		}
		s.mu.Unlock()
		c.Close()
		<-gone
	}()
	for {
		select {
		case state, ok := <-ch:
			if !ok {
				return
			}
			if _, err := c.Write(append(state, '\n')); err != nil {
				return
			}
		case <-gone:
			return
		}
	}
}

// Send runs one command in the swm at path and returns its warnings and its
// error, if any.
func Send(ctx context.Context, path, line string) (warnings []string, err error) {
	c, err := dial(ctx, path)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	if _, err := fmt.Fprintln(c, line); err != nil {
		return nil, err
	}
	rd := bufio.NewReader(c)
	for {
		reply, err := rd.ReadString('\n')
		if err != nil && (err != io.EOF || reply == "") {
			return warnings, fmt.Errorf("read reply: %w", err)
		}
		reply = strings.TrimSpace(reply)
		if msg, ok := strings.CutPrefix(reply, "warning: "); ok {
			warnings = append(warnings, msg)
			continue
		}
		if msg, ok := strings.CutPrefix(reply, "error: "); ok {
			return warnings, errors.New(msg)
		}
		if reply != "ok" {
			return warnings, fmt.Errorf("unexpected reply %q", reply)
		}
		return warnings, nil
	}
}

// WatchState copies swm's state stream to w, one JSON object per line,
// until ctx is cancelled or writing to w fails. When swm isn't running, or
// goes away, it waits for swm to come back, so a bar keeps working across
// swm restarts. It gives up once alive reports that the session is gone.
func WatchState(ctx context.Context, path string, w io.Writer, alive func() bool) error {
	for {
		err := watchOnce(ctx, path, w)
		if ctx.Err() != nil {
			return nil
		}
		if we := (writeError{}); errors.As(err, &we) {
			return we.err
		}
		if !alive() {
			return errors.New("the river session is gone")
		}
		select {
		case <-time.After(watchRetry):
		case <-ctx.Done():
			return nil
		}
	}
}

// Wait blocks until swm at path answers, waiting for it to start if it
// hasn't. It fails once alive reports that the session is gone.
func Wait(ctx context.Context, path string, alive func() bool) error {
	err := WatchState(ctx, path, stopWriter{}, alive)
	if errors.Is(err, errAnswered) {
		return nil
	}
	if err == nil {
		return ctx.Err()
	}
	return err
}

var errAnswered = errors.New("swm answered")

// stopWriter ends a watch at the first state.
type stopWriter struct{}

func (stopWriter) Write([]byte) (int, error) { return 0, errAnswered }

// writeError is a failure writing the stream out, which ends WatchState
// rather than reconnecting.
type writeError struct{ err error }

func (e writeError) Error() string { return e.err.Error() }

// watchOnce copies the state stream from one connection to swm.
func watchOnce(ctx context.Context, path string, w io.Writer) error {
	c, err := dial(ctx, path)
	if err != nil {
		return err
	}
	stop := context.AfterFunc(ctx, func() { c.Close() })
	defer stop()
	defer c.Close()
	if _, err := fmt.Fprintln(c, Watch); err != nil {
		return err
	}
	sc := bufio.NewScanner(c)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		if _, err := fmt.Fprintln(w, sc.Text()); err != nil {
			return writeError{err}
		}
	}
	return sc.Err()
}

// SessionAlive reports whether the river session this process runs in is
// still there: whether its Wayland socket takes connections.
func SessionAlive() bool {
	display := os.Getenv("WAYLAND_DISPLAY")
	if !filepath.IsAbs(display) {
		display = filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), display)
	}
	c, err := net.Dial("unix", display)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

func dial(ctx context.Context, path string) (net.Conn, error) {
	var d net.Dialer
	c, err := d.DialContext(ctx, "unix", path)
	if err != nil {
		return nil, fmt.Errorf("connect to swm (%s): %w; is swm running?", path, err)
	}
	return c, nil
}
