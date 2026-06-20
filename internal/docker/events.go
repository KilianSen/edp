package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"os/exec"
)

// Event is the subset of a `docker events` record we care about.
type Event struct {
	Type   string `json:"Type"`
	Action string `json:"Action"`
	Actor  struct {
		Attributes map[string]string `json:"Attributes"`
	} `json:"Actor"`
}

// EnvID returns the edp.env label carried on the event's actor, or "" if none.
func (e Event) EnvID() string { return e.Actor.Attributes[LabelEnv] }

// StreamEvents runs `docker events` filtered to edp-managed objects and calls
// handle for each decoded event until ctx is cancelled or the stream ends. The
// caller is responsible for any reconnect loop.
func (c *Client) StreamEvents(ctx context.Context, handle func(Event)) error {
	cmd := exec.CommandContext(ctx, c.bin, "events",
		"--filter", "label="+LabelManaged+"=true",
		"--filter", "type=container",
		"--format", "{{json .}}")
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	sc := bufio.NewScanner(pipe)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var ev Event
		if err := json.Unmarshal(sc.Bytes(), &ev); err == nil {
			handle(ev)
		}
	}
	return cmd.Wait()
}
