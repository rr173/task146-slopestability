// Command task146-slopestability serves the slope-stability HTTP API backed by
// SQLite, and provides a --smoke-test that exercises the full contract (slope
// + layers + slip surface + Bishop/Fellenius analysis + critical search +
// reinforcement weakest-link + pore-pressure priority + monitoring recompute +
// alert state machine + restart recovery + frontend page) without real-time
// sleeps.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"task146-slopestability/internal/clock"
	"task146-slopestability/internal/httpapi"
	"task146-slopestability/internal/selfcheck"
	"task146-slopestability/internal/service"
	"task146-slopestability/internal/store"
	"task146-slopestability/internal/webfs"
)

// webFS is the embedded static frontend (native HTML/CSS/JS, no build step).
var webFS = webfs.FS()

// osExit is indirected so the selfcheck (which calls Run from tests) can still
// run under a test process; in main we use os.Exit directly.
var osExit = os.Exit

func main() {
	smoke := flag.Bool("smoke-test", false, "run self-check and exit")
	migrateOnly := flag.Bool("migrate-only", false, "apply schema and exit")
	dbPath := flag.String("db", "slopestab.db", "SQLite database file path")
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	if *smoke {
		if err := selfcheck.Run(); err != nil {
			fmt.Println("smoke-test: FAIL:", err)
			osExit(1)
		}
		fmt.Println("smoke-test: ok")
		osExit(0)
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	svc := service.NewWithClock(st, clock.Real{})
	// Restart-recovery: recompute every derived figure from the persisted
	// authoritative inputs so a process that died between writes converges.
	if rc, dr, err := svc.Reconcile().ReconcileAll(context.Background()); err != nil {
		log.Printf("reconcile on startup: %v", err)
	} else if rc > 0 {
		log.Printf("reconcile: recomputed %d slope(s), %d drift(s)", rc, dr)
	}

	if *migrateOnly {
		fmt.Println("migrate-only: schema applied")
		osExit(0)
	}

	mux := httpapi.NewMux(httpapi.Services{Svc: svc}, webFS)
	srv := &http.Server{Addr: *addr, Handler: mux}

	go func() {
		log.Printf("task146-slopestability listening on %s", *addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("shutting down")
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
}
