// Command streamctl controls a running swm.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"

	"github.com/grunclepug/stream/internal/command"
	"github.com/grunclepug/stream/internal/ipc"
)

var Version = "dev"

const usage = `usage: streamctl <command> [arguments]

Run a command in the running swm, such as:

  streamctl view 3
  streamctl spawn foot
  streamctl reload

Every command a binding can run works, except move and resize, which
follow the pointer. Also:

  watch        print swm's state as JSON, one line per change; waits
               for swm when it isn't running
  wait         wait until swm is running, such as before starting a bar
  gen-config   write swm's default config file
  version      print the version
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	err := run(os.Args[1], os.Args[2:])
	if ue := (usageError{}); errors.As(err, &ue) {
		fmt.Fprintf(os.Stderr, "streamctl: %s\n\n%s", ue.msg, ue.usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "streamctl:", err)
		os.Exit(1)
	}
}

func run(name string, args []string) error {
	switch name {
	case "gen-config":
		return genConfig(args, os.Stdout)
	case "version", "-version":
		fmt.Println("streamctl", Version)
		return nil
	case "help", "-h", "-help":
		fmt.Print(usage)
		return nil
	}

	switch {
	case (name == "watch" || name == "wait") && len(args) > 0:
		return usageError{name + " takes no arguments", usage}
	case name != "watch" && name != "wait" && !slices.Contains(command.Names(), name):
		return usageError{fmt.Sprintf("unknown command %q", name), usage}
	}

	path, err := ipc.SocketPath()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	switch name {
	case "watch":
		return ipc.WatchState(ctx, path, os.Stdout, ipc.SessionAlive)
	case "wait":
		return ipc.Wait(ctx, path, ipc.SessionAlive)
	}
	warnings, err := ipc.Send(ctx, path, strings.Join(append([]string{name}, args...), " "))
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "streamctl: warning:", w)
	}
	return err
}
