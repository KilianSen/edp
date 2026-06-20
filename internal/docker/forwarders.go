package docker

import (
	"context"
	"fmt"
	"io"
	"strings"
)

const (
	LabelFwd     = "edp.fwd"      // value: env id
	LabelFwdSpec = "edp.fwd.spec" // value: "<listen>|<target>" — lets reconcile detect changes
)

// Forwarder describes a per-env TCP forwarder sidecar container.
type Forwarder struct {
	EnvID   string
	Name    string
	Spec    string // "<listen>|<target>"
	Running bool
}

// SelfImage returns the image of the edp container itself (so forwarder sidecars
// can run the same image). Best-effort; empty string if it can't be determined.
func (c *Client) SelfImage(ctx context.Context, selfHostname string) string {
	out, err := c.capture(ctx, "inspect", selfHostname, "--format", "{{.Config.Image}}")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// ListForwarders returns the existing forwarder sidecar containers.
func (c *Client) ListForwarders(ctx context.Context) ([]Forwarder, error) {
	out, err := c.capture(ctx, "ps", "-a",
		"--filter", "label="+LabelFwd,
		"--format", `{{.Label "edp.fwd"}}`+"\t"+`{{.Names}}`+"\t"+`{{.Label "edp.fwd.spec"}}`+"\t"+`{{.State}}`)
	if err != nil {
		return nil, err
	}
	var fwds []Forwarder
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 4 {
			continue
		}
		fwds = append(fwds, Forwarder{EnvID: parts[0], Name: parts[1], Spec: parts[2], Running: parts[3] == "running"})
	}
	return fwds, nil
}

// StartForwarder runs a one-port forwarder sidecar that publishes listen:listen,
// joins the shared network, and forwards to target via `edp forward`.
func (c *Client) StartForwarder(ctx context.Context, _ io.Writer, name, envID, listen, target, image string) error {
	spec := listen + "|" + target
	out, err := c.captureCombined(ctx, "run", "-d", "--name", name,
		"--restart", "unless-stopped",
		"--network", SharedNetwork,
		"-p", listen+":"+listen,
		"--label", LabelManaged+"=true",
		"--label", LabelFwd+"="+envID,
		"--label", LabelFwdSpec+"="+spec,
		image, "forward", listen, target)
	if err != nil {
		return fmt.Errorf("%v: %s", err, out)
	}
	return nil
}

// RemoveContainerByName force-removes a container by name (best effort).
func (c *Client) RemoveContainerByName(ctx context.Context, name string) error {
	_, err := c.captureCombined(ctx, "rm", "-f", name)
	if err != nil && !strings.Contains(err.Error(), "No such container") {
		return fmt.Errorf("rm %s: %w", name, err)
	}
	return nil
}
