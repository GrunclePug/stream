// Command swm is the stream window manager for the river Wayland compositor.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/grunclepug/stream/internal/config"
	"github.com/grunclepug/stream/internal/wm"
)

var Version = "dev"

func main() {
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, `usage: swm [-config path] [-log-level level] [-version]

The stream window manager for river. Run it from river's init, usually in
a loop that restarts it; streamctl controls it while it runs.

`)
		flag.PrintDefaults()
	}
	version := flag.Bool("version", false, "print the version and exit")
	logLevel := flag.String("log-level", "info", "log `level`: debug, info, warn or error")
	cfgPath := flag.String("config", "", "config file `path` (default $XDG_CONFIG_HOME/stream/config)")
	flag.Parse()
	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "swm: unexpected argument %q\n", flag.Arg(0))
		flag.Usage()
		os.Exit(2)
	}
	if *version {
		fmt.Println("swm", Version)
		return
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(*logLevel)); err != nil {
		fmt.Fprintf(os.Stderr, "swm: invalid -log-level %q\n", *logLevel)
		os.Exit(2)
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	if *cfgPath == "" {
		p, err := config.Path()
		if err != nil {
			log.Error(err.Error())
			os.Exit(1)
		}
		*cfgPath = p
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("starting", "version", Version)
	if err := wm.Run(ctx, log, *cfgPath); err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
}
