// Command gittreemux is a single, self-contained binary that surfaces every
// workmux worktree and agent running on the machine as a local web dashboard,
// and lets the user act on them from the browser.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lwaddicor/gittreemux/internal/actionlog"
	"github.com/lwaddicor/gittreemux/internal/api"
	"github.com/lwaddicor/gittreemux/internal/discover"
	"github.com/lwaddicor/gittreemux/internal/focus"
	"github.com/lwaddicor/gittreemux/internal/workmux"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "serve":
		os.Exit(runServe(os.Args[2:]))
	case "version", "-V", "--version":
		fmt.Println("gittreemux — cross-project workmux dashboard")
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `gittreemux — cross-project workmux dashboard

Usage:
  gittreemux serve [flags]
  gittreemux version

Flags for 'serve':
`)
	defaultFlagSet().PrintDefaults()
}

func defaultFlagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.String("listen", "127.0.0.1:8787", "host:port to bind (loopback by default)")
	fs.String("prefix", "wm-", "workmux tmux window name prefix used for discovery")
	fs.Int("concurrency", 8, "max concurrent project reads")
	fs.Duration("cache-ttl", 2*time.Second, "per-project result cache TTL")
	fs.String("log", "", "optional action log file path (default: stderr)")
	fs.String("workmux", "workmux", "path to the workmux binary")
	return fs
}

func runServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	listen := fs.String("listen", "127.0.0.1:8787", "host:port to bind (loopback by default)")
	prefix := fs.String("prefix", "wm-", "workmux tmux window name prefix used for discovery")
	concurrency := fs.Int("concurrency", 8, "max concurrent project reads")
	cacheTTL := fs.Duration("cache-ttl", 2*time.Second, "per-project result cache TTL")
	logPath := fs.String("log", "", "optional action log file path (default: stderr)")
	wmBin := fs.String("workmux", "workmux", "path to the workmux binary")
	fs.Parse(args)

	client := &workmux.Client{Bin: *wmBin}
	if ver, err := client.Version(); err != nil {
		log.Printf("warning: %v", err)
	} else {
		log.Printf("found %s", ver)
	}

	alc := actionlog.New(*logPath)
	defer alc.Close()

	wd, _ := os.Getwd()
	disc := discover.New(discover.Options{
		StartDir:    wd,
		Prefix:      *prefix,
		Concurrency: *concurrency,
		CacheTTL:    *cacheTTL,
		Workmux:     client,
	})

	srv := &api.Server{Discoverer: disc, Workmux: client, Log: alc, Focus: focus.New()}
	httpSrv := &http.Server{
		Addr:              *listen,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	log.Printf("gittreemux dashboard listening on http://%s/", *listen)

	select {
	case <-ctx.Done():
		log.Printf("shutting down…")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
	case err := <-errCh:
		log.Printf("server error: %v", err)
		return 1
	}
	return 0
}
