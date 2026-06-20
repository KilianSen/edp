// Package portproxy provides raw TCP port forwarding for environments. Each env
// with a listen port gets its own small "sidecar" forwarder container
// (edp-fwd-<id>) that publishes only that one host port and pipes connections to
// the env's container over the shared Docker network. This keeps the open-port
// count minimal (only ports actually in use) and fully dynamic — adding or
// removing a forward needs no edp restart and no pre-published port range.
//
// The sidecar runs the edp image itself via `edp forward <listen> <target>`, so
// there is no extra image dependency.
package portproxy

import (
	"context"
	"fmt"
	"log"
	"time"

	"edp/internal/docker"
	"edp/internal/naming"
	"edp/internal/store"
)

const reconcileEvery = 5 * time.Second

type Manager struct {
	st    *store.Store
	dk    *docker.Client
	image string // edp image used for sidecars; empty disables forwarding
}

func New(st *store.Store, dk *docker.Client, image string) *Manager {
	return &Manager{st: st, dk: dk, image: image}
}

// Run reconciles forwarder sidecars against env config until ctx is cancelled.
// The sidecars are intentionally left running on shutdown (restart=unless-stopped)
// so forwards survive an edp restart; they are reclaimed on the next reconcile.
func (m *Manager) Run(ctx context.Context) {
	if m.image == "" {
		log.Print("portproxy: edp image unknown; TCP port forwarding disabled")
		return
	}
	t := time.NewTicker(reconcileEvery)
	defer t.Stop()
	m.reconcile(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.reconcile(ctx)
		}
	}
}

type spec struct {
	name   string
	envID  string
	listen string
	target string
}

func (s spec) value() string { return s.listen + "|" + s.target }

func (m *Manager) reconcile(ctx context.Context) {
	envs, err := m.st.ListEnvironments(ctx)
	if err != nil {
		log.Printf("portproxy: list envs: %v", err)
		return
	}
	desired := map[string]spec{}
	for _, e := range envs {
		if e.ListenPort == "" || e.ProxyPort == "" {
			continue
		}
		id := fmt.Sprint(e.ID)
		desired[id] = spec{
			name:   "edp-fwd-" + id,
			envID:  id,
			listen: e.ListenPort,
			target: naming.ContainerName(e.Name) + ":" + e.ProxyPort,
		}
	}

	current, err := m.dk.ListForwarders(ctx)
	if err != nil {
		log.Printf("portproxy: list forwarders: %v", err)
		return
	}
	have := map[string]docker.Forwarder{}
	for _, f := range current {
		have[f.EnvID] = f
	}

	// remove forwarders that are gone or whose spec changed
	for id, f := range have {
		d, ok := desired[id]
		if !ok || d.value() != f.Spec {
			if err := m.dk.RemoveContainerByName(ctx, f.Name); err != nil {
				log.Printf("portproxy: remove %s: %v", f.Name, err)
			} else {
				log.Printf("portproxy: removed forwarder %s", f.Name)
			}
			delete(have, id)
		}
	}

	// create new/updated forwarders (also recreate ones that exist but aren't
	// running — e.g. a previous start failed on a port conflict)
	for id, d := range desired {
		if f, ok := have[id]; ok && f.Spec == d.value() && f.Running {
			continue // already correct and healthy
		}
		_ = m.dk.RemoveContainerByName(ctx, d.name) // clear any stale leftover
		if err := m.dk.StartForwarder(ctx, nil, d.name, d.envID, d.listen, d.target, m.image); err != nil {
			log.Printf("portproxy: start %s (:%s -> %s): %v", d.name, d.listen, d.target, err)
			continue
		}
		log.Printf("portproxy: forwarding :%s -> %s via %s", d.listen, d.target, d.name)
	}
}
