package docker

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// Login authenticates against a registry using password-stdin. registry may be
// empty for Docker Hub.
func (c *Client) Login(ctx context.Context, out io.Writer, registry, username, password string) error {
	args := []string{"login", "-u", username, "--password-stdin"}
	if registry != "" {
		args = append(args, registry)
	}
	return c.Run(ctx, out, RunOpts{Stdin: password}, args...)
}

// RemoveContainersByEnv force-removes every container labeled for the env.
func (c *Client) RemoveContainersByEnv(ctx context.Context, out io.Writer, envID int64) error {
	ids, err := c.capture(ctx, "ps", "-aq",
		"--filter", fmt.Sprintf("label=%s=%d", LabelEnv, envID))
	if err != nil {
		return err
	}
	ids = strings.TrimSpace(ids)
	if ids == "" {
		return nil
	}
	args := append([]string{"rm", "-f", "-v"}, strings.Fields(ids)...)
	return c.Run(ctx, out, RunOpts{}, args...)
}

// RemoveVolumesByLabel removes all volumes carrying the given label selector
// (e.g. "edp.env=3"). Best effort; in-use volumes will error and are skipped.
func (c *Client) RemoveVolumesByLabel(ctx context.Context, out io.Writer, label string) error {
	ids, err := c.capture(ctx, "volume", "ls", "-q", "--filter", "label="+label)
	if err != nil {
		return err
	}
	return c.removeVolumeList(ctx, out, ids)
}

// RemoveVolumesByComposeProject removes volumes belonging to a compose project.
func (c *Client) RemoveVolumesByComposeProject(ctx context.Context, out io.Writer, project string) error {
	ids, err := c.capture(ctx, "volume", "ls", "-q",
		"--filter", "label=com.docker.compose.project="+project)
	if err != nil {
		return err
	}
	return c.removeVolumeList(ctx, out, ids)
}

// RemoveNamedVolumes removes a specific set of named volumes.
func (c *Client) RemoveNamedVolumes(ctx context.Context, out io.Writer, names []string) {
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if err := c.Run(ctx, out, RunOpts{}, "volume", "rm", "-f", n); err != nil {
			fmt.Fprintf(out, "warning: could not remove volume %s: %v\n", n, err)
		}
	}
}

func (c *Client) removeVolumeList(ctx context.Context, out io.Writer, ids string) error {
	ids = strings.TrimSpace(ids)
	if ids == "" {
		return nil
	}
	for _, v := range strings.Fields(ids) {
		if err := c.Run(ctx, out, RunOpts{}, "volume", "rm", "-f", v); err != nil {
			fmt.Fprintf(out, "warning: could not remove volume %s: %v\n", v, err)
		}
	}
	return nil
}

// PruneDanglingImages removes dangling images (best effort).
func (c *Client) PruneDanglingImages(ctx context.Context, out io.Writer) error {
	return c.Run(ctx, out, RunOpts{}, "image", "prune", "-f")
}

// ComposeDown tears a compose stack down by project name (no compose file
// needed — compose finds the services via the project label), removing its
// volumes and any orphaned containers.
func (c *Client) ComposeDown(ctx context.Context, out io.Writer, project string) error {
	return c.Run(ctx, out, RunOpts{}, "compose", "-p", project, "down", "-v", "--remove-orphans")
}

// ReapInstance force-removes every container carrying this instance's label
// (single-container envs + forwarder sidecars) and then drops the shared
// network. Scoped by edp.instance so it never touches another edp's containers.
// Compose stacks are torn down separately (see deploy.Reap), since compose
// creates their containers without our label.
func (c *Client) ReapInstance(ctx context.Context, out io.Writer) error {
	if c.instanceID == "" {
		return fmt.Errorf("no instance id; refusing to reap")
	}
	ids, err := c.capture(ctx, "ps", "-aq", "--filter", "label="+LabelInstance+"="+c.instanceID)
	if err != nil {
		return err
	}
	if ids = strings.TrimSpace(ids); ids != "" {
		args := append([]string{"rm", "-f", "-v"}, strings.Fields(ids)...)
		if err := c.Run(ctx, out, RunOpts{}, args...); err != nil {
			fmt.Fprintf(out, "warning: removing containers: %v\n", err)
		}
	}
	// Best effort: drop the shared network. Docker refuses if another instance
	// still has containers attached, which is exactly the safety we want.
	if o, err := c.captureCombined(ctx, "network", "rm", SharedNetwork); err != nil &&
		!strings.Contains(o, "not found") && !strings.Contains(o, "No such") {
		fmt.Fprintf(out, "note: shared network %q left in place: %s\n", SharedNetwork, strings.TrimSpace(o))
	}
	return nil
}

// SharedNetwork is the user-defined bridge network edp and the containers it
// proxies to share, so edp can reach them by container name.
const SharedNetwork = "edp"

// EnsureNetwork creates the shared network if it does not already exist.
func (c *Client) EnsureNetwork(ctx context.Context) error {
	if _, err := c.capture(ctx, "network", "inspect", SharedNetwork); err == nil {
		return nil
	}
	out, err := c.captureCombined(ctx, "network", "create", SharedNetwork)
	if err != nil && !strings.Contains(out, "already exists") {
		return fmt.Errorf("create network %s: %s", SharedNetwork, out)
	}
	return nil
}

// ConnectShared attaches a container to the shared network (idempotent — an
// "already connected" error is treated as success).
func (c *Client) ConnectShared(ctx context.Context, container string) error {
	out, err := c.captureCombined(ctx, "network", "connect", SharedNetwork, container)
	if err != nil && !strings.Contains(out, "already") {
		return fmt.Errorf("connect %s to %s: %s", container, SharedNetwork, out)
	}
	return nil
}
