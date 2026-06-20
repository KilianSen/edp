// Package sh runs external commands, streaming their combined output line by
// line to a writer. It is the single choke point for shelling out (docker, git).
package sh

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

type Opts struct {
	Dir   string
	Env   []string // additional KEY=VAL appended to the process environment
	Stdin string
	// QuietEcho suppresses printing the "$ cmd args" banner (use when args carry secrets).
	QuietEcho bool
	// EchoArgs, if set, is printed instead of the real args in the banner.
	EchoArgs []string
}

// Stream runs bin+args, copying merged stdout/stderr to out as it arrives.
func Stream(ctx context.Context, out io.Writer, opts Opts, bin string, args ...string) error {
	cmd := exec.CommandContext(ctx, bin, args...)
	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}
	if len(opts.Env) > 0 {
		cmd.Env = append(cmd.Environ(), opts.Env...)
	}
	if opts.Stdin != "" {
		cmd.Stdin = strings.NewReader(opts.Stdin)
	}
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if out != nil && !opts.QuietEcho {
		shown := args
		if opts.EchoArgs != nil {
			shown = opts.EchoArgs
		}
		fmt.Fprintf(out, "$ %s %s\n", bin, strings.Join(shown, " "))
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	sc := bufio.NewScanner(pipe)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		if out != nil {
			fmt.Fprintln(out, sc.Text())
		}
	}
	return cmd.Wait()
}

// Capture runs a command and returns its trimmed stdout.
func Capture(ctx context.Context, bin string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, bin, args...).Output()
	return strings.TrimSpace(string(out)), err
}

// CaptureCombined runs a command and returns trimmed stdout+stderr together,
// useful when the interesting detail (e.g. "already exists") is on stderr.
func CaptureCombined(ctx context.Context, bin string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, bin, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
