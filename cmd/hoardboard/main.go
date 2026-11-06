// Command hoardboard runs the hoardboard server.
//
//	hoardboard               serve (configured by HOARDBOARD_* environment variables)
//	hoardboard healthcheck   exit 0 if the local server's /api/health is OK
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	_ "time/tzdata" // TZ works in an image without zoneinfo; "today" for History depends on it

	"github.com/Marcowich0/hoardboard/internal/config"
	"github.com/Marcowich0/hoardboard/internal/server"
	"github.com/Marcowich0/hoardboard/web"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log, os.Args[1:]); err != nil {
		log.Error("exit", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger, args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.FromEnv(os.Getenv)
	if err != nil {
		return err
	}

	switch {
	case len(args) == 0:
	case args[0] == "healthcheck":
		return server.Probe(ctx, cfg.Port)
	default:
		return fmt.Errorf("unknown command %q (want no command or healthcheck)", args[0])
	}

	if !web.Built {
		log.Warn("web app not embedded; serving stub page (build with -tags webdist)")
	}
	cfg.Revision = revision()
	srv, err := server.New(cfg, log)
	if err != nil {
		return err
	}
	defer srv.Close()
	return srv.ListenAndServe(ctx)
}

// stampedRevision is the commit the image was built from. The Dockerfile sets
// it at link time (-X main.stampedRevision=...), since its build context has
// no .git for go build to read it from.
var stampedRevision string

// revision is the commit hoardboard was built from: the stamped one, or else
// the one go build recorded from the checkout, with "-dirty" if the checkout
// had changes, or "" if neither is there.
func revision() string {
	if stampedRevision != "" {
		return stampedRevision
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	var rev, dirty string
	for _, s := range info.Settings {
		switch {
		case s.Key == "vcs.revision":
			rev = s.Value
		case s.Key == "vcs.modified" && s.Value == "true":
			dirty = "-dirty"
		}
	}
	if rev == "" {
		return ""
	}
	return rev + dirty
}
