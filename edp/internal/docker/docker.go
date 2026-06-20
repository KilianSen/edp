// Package docker wraps the Docker CLI. edp shells out to `docker` / `docker
// compose` rather than using the Go SDK: it keeps the dependency tree tiny and
// means compose "just works" with the same semantics the user already knows.
package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"edp/internal/sh"
)

const (
	LabelManaged  = "edp.managed"
	LabelEnv      = "edp.env"
	LabelInstance = "edp.instance" // value: this edp instance's id; scopes teardown
)

type Client struct {
	bin        string
	instanceID string
}

func New(bin, instanceID string) *Client {
	if bin == "" {
		bin = "docker"
	}
	return &Client{bin: bin, instanceID: instanceID}
}

// InstanceID is the stable id stamped on every container this edp creates.
func (c *Client) InstanceID() string { return c.instanceID }

// RunOpts aliases sh.Opts for callers building docker invocations.
type RunOpts = sh.Opts

// Run executes a docker command, streaming combined stdout+stderr to out.
func (c *Client) Run(ctx context.Context, out io.Writer, opts RunOpts, args ...string) error {
	return sh.Stream(ctx, out, opts, c.bin, args...)
}

func (c *Client) capture(ctx context.Context, args ...string) (string, error) {
	return sh.Capture(ctx, c.bin, args...)
}

func (c *Client) captureCombined(ctx context.Context, args ...string) (string, error) {
	return sh.CaptureCombined(ctx, c.bin, args...)
}

// ContainerInfo is a subset of `docker ps` JSON output.
type ContainerInfo struct {
	ID     string `json:"ID"`
	Names  string `json:"Names"`
	State  string `json:"State"`
	Status string `json:"Status"`
	Image  string `json:"Image"`
}

// ListByEnv returns containers labeled for the given env id (running + stopped).
func (c *Client) ListByEnv(ctx context.Context, envID int64) ([]ContainerInfo, error) {
	out, err := c.capture(ctx, "ps", "-a",
		"--filter", fmt.Sprintf("label=%s=%d", LabelEnv, envID),
		"--format", "{{json .}}")
	if err != nil {
		return nil, err
	}
	var infos []ContainerInfo
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ci ContainerInfo
		if err := json.Unmarshal([]byte(line), &ci); err == nil {
			infos = append(infos, ci)
		}
	}
	return infos, nil
}

// ImageDigest returns the RepoDigests/Id of an image, best-effort.
func (c *Client) ImageDigest(ctx context.Context, image string) string {
	out, err := c.capture(ctx, "image", "inspect", image, "--format", "{{.Id}}")
	if err != nil {
		return ""
	}
	return out
}
