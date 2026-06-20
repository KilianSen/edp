// Command edp is the Easy Deploy Platform: a single container that manages the
// lifecycle of test environments (build/pull, deploy, redeploy, sweep) with a
// dashboard, webhook, and REST API.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"edp/internal/clearsite"
	"edp/internal/config"
	"edp/internal/deploy"
	"edp/internal/docker"
	"edp/internal/events"
	"edp/internal/hooks"
	"edp/internal/logbus"
	"edp/internal/portproxy"
	"edp/internal/scheduler"
	"edp/internal/server"
	"edp/internal/statushub"
	"edp/internal/store"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("edp ")

	// Sidecar mode: `edp forward <listen> <target>` runs a TCP forwarder and
	// nothing else (used by the per-env port-forward sidecar containers).
	if len(os.Args) >= 4 && os.Args[1] == "forward" {
		portproxy.RunForwarder(os.Args[2], os.Args[3])
		return
	}

	// Teardown mode: `edp reap` removes every container/stack/network this
	// instance created and exits. Reads the instance id from the persisted DB,
	// so it works even as a one-shot (e.g. `docker compose run --rm edp reap`).
	if len(os.Args) >= 2 && os.Args[1] == "reap" {
		runReap()
		return
	}

	cfg := config.Load()

	for _, d := range []string{cfg.DataDir, cfg.WorkspaceDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			log.Fatalf("create dir %s: %v", d, err)
		}
	}

	st, err := store.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// Fail over any deploys/hook-runs that were interrupted by a previous stop,
	// so nothing is stuck "running" after a restart.
	if err := st.ResetInterrupted(context.Background()); err != nil {
		log.Printf("reset interrupted state: %v", err)
	}

	instanceID, err := st.InstanceID(context.Background())
	if err != nil {
		log.Printf("instance id: %v (teardown scoping disabled)", err)
	}

	dk := docker.New(cfg.DockerBin, instanceID)
	bus := logbus.New()
	hookBus := logbus.New()
	hub := statushub.New()
	clearFlags := clearsite.New()
	engine := deploy.New(st, dk, bus, clearFlags, cfg.WorkspaceDir, cfg.PythonBin)
	hookRunner := hooks.New(st, hookBus, cfg.WorkspaceDir, cfg.PythonBin)

	sched := scheduler.New(st, engine, hookRunner)
	sched.Start()
	defer sched.Stop()

	// Watch Docker events to push live container status to the dashboard.
	watchCtx, stopWatch := context.WithCancel(context.Background())
	defer stopWatch()
	go events.New(dk, hub).Run(watchCtx)

	// Ensure the shared proxy network exists and attach edp itself to it, so the
	// reverse proxy and TCP forwarder sidecars can reach proxied containers by name.
	selfHost, _ := os.Hostname()
	if err := dk.EnsureNetwork(watchCtx); err != nil {
		log.Printf("proxy network: %v", err)
	} else if selfHost != "" {
		if err := dk.ConnectShared(watchCtx, selfHost); err != nil {
			log.Printf("proxy network self-connect: %v", err)
		}
	}

	// Raw TCP port forwarding via per-env sidecar containers running the edp
	// image. EDP_IMAGE overrides the auto-detected self image.
	fwdImage := cfg.Image
	if fwdImage == "" && selfHost != "" {
		fwdImage = dk.SelfImage(watchCtx, selfHost)
	}
	go portproxy.New(st, dk, fwdImage).Run(watchCtx)

	srv, err := server.New(cfg, st, engine, bus, dk, hub, hookRunner, clearFlags)
	if err != nil {
		log.Fatalf("init server: %v", err)
	}

	// Load any environments declared via EDP_IMPORT (idempotent).
	srv.Bootstrap(context.Background())

	httpSrv := &http.Server{
		Addr:        cfg.Addr,
		Handler:     srv.Handler(),
		ReadTimeout: 15 * time.Second,
	}

	go func() {
		log.Printf("listening on %s (data=%s workspace=%s)", cfg.Addr, cfg.DataDir, cfg.WorkspaceDir)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)

	if cfg.ReapOnExit {
		stopWatch() // stop the reconcile/events loops so they don't fight the teardown
		log.Println("EDP_REAP_ON_EXIT set: tearing down managed containers and stacks...")
		if err := deploy.Reap(ctx, st, dk, log.Writer()); err != nil {
			log.Printf("reap on exit: %v", err)
		}
	}
}

// runReap performs a one-shot teardown of this instance's environments and
// exits. It mirrors normal startup wiring but only needs the store (for the
// instance id and compose stacks) and a docker client.
func runReap() {
	cfg := config.Load()
	st, err := store.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	instanceID, err := st.InstanceID(context.Background())
	if err != nil {
		log.Fatalf("instance id: %v", err)
	}
	dk := docker.New(cfg.DockerBin, instanceID)

	log.Println("reaping edp-managed containers, stacks, and network...")
	if err := deploy.Reap(context.Background(), st, dk, log.Writer()); err != nil {
		log.Fatalf("reap: %v", err)
	}
	log.Println("reap complete")
}
