package ipc

import (
	"bufio"
	"context"
	"errors"
	"io"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// serve runs a server whose "window manager" answers every request from a
// goroutine when poked, the way swm does from its manage sequence.
func serve(t *testing.T) (*Server, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "swm.sock")
	return serveAt(t, path), path
}

func serveAt(t *testing.T, path string) *Server {
	t.Helper()
	var s *Server
	poked := make(chan struct{}, 16)
	s, err := Listen(path, func() { poked <- struct{}{} })
	if err != nil {
		t.Fatal(err)
	}
	s.Serve()
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-poked:
				for _, r := range s.Take() {
					switch r.Line {
					case "fail":
						r.Reply(nil, errors.New("it failed\non two lines"))
					case "warn":
						r.Reply([]string{"first", "second\nline"}, nil)
					default:
						r.Reply(nil, nil)
					}
				}
			case <-done:
				return
			}
		}
	}()
	t.Cleanup(func() {
		s.Close()
		close(done)
	})
	return s
}

func TestSend(t *testing.T) {
	_, path := serve(t)
	ctx := context.Background()
	if w, err := Send(ctx, path, "view 3"); err != nil || w != nil {
		t.Errorf("Send = %v, %v", w, err)
	}
	_, err := Send(ctx, path, "fail")
	if err == nil || err.Error() != "it failed; on two lines" {
		t.Errorf("Send(fail) = %v", err)
	}
	w, err := Send(ctx, path, "warn")
	if err != nil || !slices.Equal(w, []string{"first", "second; line"}) {
		t.Errorf("Send(warn) = %q, %v", w, err)
	}
}

func TestSendNoServer(t *testing.T) {
	_, err := Send(context.Background(), filepath.Join(t.TempDir(), "none.sock"), "view 3")
	if err == nil {
		t.Error("no error without a server")
	}
}

func TestListenInUse(t *testing.T) {
	_, path := serve(t)
	if _, err := Listen(path, func() {}); err == nil {
		t.Error("a second server took a socket in use")
	}
}

// lines returns a function that reads the next line from r, failing the
// test if none comes.
func lines(t *testing.T, r io.Reader) func() string {
	sc := bufio.NewScanner(r)
	return func() string {
		t.Helper()
		got := make(chan string, 1)
		go func() {
			sc.Scan()
			got <- sc.Text()
		}()
		select {
		case l := <-got:
			return l
		case <-time.After(2 * time.Second):
			t.Fatal("no state")
			return ""
		}
	}
}

func alive() bool { return true }

func TestWatch(t *testing.T) {
	s, path := serve(t)
	s.Publish([]byte(`{"n":1}`))

	ctx, cancel := context.WithCancel(context.Background())
	pr, pw := io.Pipe()
	errc := make(chan error, 1)
	go func() { errc <- WatchState(ctx, path, pw, alive) }()
	next := lines(t, pr)

	if l := next(); l != `{"n":1}` {
		t.Errorf("first state %q", l)
	}
	s.Publish([]byte(`{"n":1}`)) // unchanged: not sent
	s.Publish([]byte(`{"n":2}`))
	if l := next(); l != `{"n":2}` {
		t.Errorf("second state %q", l)
	}
	cancel()
	pr.Close()
	if err := <-errc; err != nil && !errors.Is(err, io.ErrClosedPipe) {
		t.Errorf("WatchState after cancel: %v", err)
	}
}

// A watcher started before swm, or across a restart of it, waits for it.
func TestWatchReconnects(t *testing.T) {
	path := filepath.Join(t.TempDir(), "swm.sock")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pr, pw := io.Pipe()
	defer pr.Close()
	go WatchState(ctx, path, pw, alive)
	next := lines(t, pr)

	s := serveAt(t, path)
	s.Publish([]byte(`{"n":1}`))
	if l := next(); l != `{"n":1}` {
		t.Errorf("state from the first swm %q", l)
	}
	s.Close()
	s = serveAt(t, path)
	s.Publish([]byte(`{"n":2}`))
	if l := next(); l != `{"n":2}` {
		t.Errorf("state from the second swm %q", l)
	}
}

func TestWatchSessionGone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "swm.sock")
	err := WatchState(context.Background(), path, io.Discard, func() bool { return false })
	if err == nil {
		t.Error("no error once the session is gone")
	}
}

func TestWait(t *testing.T) {
	path := filepath.Join(t.TempDir(), "swm.sock")
	errc := make(chan error, 1)
	go func() { errc <- Wait(context.Background(), path, alive) }()
	time.Sleep(2 * watchRetry)
	select {
	case err := <-errc:
		t.Fatalf("Wait returned before swm started: %v", err)
	default:
	}
	s := serveAt(t, path)
	s.Publish([]byte(`{"n":1}`))
	select {
	case err := <-errc:
		if err != nil {
			t.Errorf("Wait: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Wait still waiting after swm started")
	}
	if err := Wait(context.Background(), filepath.Join(t.TempDir(), "none.sock"), func() bool { return false }); err == nil {
		t.Error("no error once the session is gone")
	}
}
