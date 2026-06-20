package deploy

import (
	"context"
	"fmt"
	"io"

	"edp/internal/docker"
	"edp/internal/naming"
	"edp/internal/store"
)

// Reap is the teardown counterpart to a deploy: it removes everything this edp
// instance created so removing edp doesn't leave dangling containers. It tears
// down each compose stack by project (compose creates those containers without
// our label), then force-removes the instance's single-container envs and
// forwarder sidecars and drops the shared network in one labeled sweep.
//
// It is driven by the instance's own store and the edp.instance label, so it is
// scoped to this instance and safe to run when several edp instances share a
// host. Used by `edp reap` and the opt-in EDP_REAP_ON_EXIT shutdown hook.
func Reap(ctx context.Context, st *store.Store, dk *docker.Client, out io.Writer) error {
	envs, err := st.ListEnvironments(ctx)
	if err != nil {
		return err
	}
	for _, env := range envs {
		if env.DeployType != store.DeployCompose {
			continue
		}
		project := naming.ComposeProject(env.ID)
		fmt.Fprintf(out, "tearing down compose stack %s (%s)\n", project, env.Name)
		if err := dk.ComposeDown(ctx, out, project); err != nil {
			fmt.Fprintf(out, "warning: compose down %s: %v\n", project, err)
		}
	}
	return dk.ReapInstance(ctx, out)
}
