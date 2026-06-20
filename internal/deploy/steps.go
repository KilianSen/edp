package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"edp/internal/docker"
	"edp/internal/naming"
	"edp/internal/sh"
	"edp/internal/source"
	"edp/internal/store"
)

// deploy runs the full pipeline for an env and returns (commitSHA, imageDigest,
// readyMs). readyMs is the time from start until the health check passed (0 if
// no health check is configured).
func (e *Engine) deploy(ctx context.Context, lw *logWriter, env *store.Environment, start time.Time) (string, string, int64, error) {
	repoDir := filepath.Join(e.workspace, fmt.Sprintf("env-%d", env.ID))

	hookEnv := []string{
		"EDP_ENV_ID=" + fmt.Sprint(env.ID),
		"EDP_ENV_NAME=" + env.Name,
		"EDP_DEPLOY_TYPE=" + env.DeployType,
		"EDP_SOURCE_TYPE=" + env.SourceType,
		"EDP_REPO_DIR=" + repoDir,
	}
	// the env's own variables are available to setup/cleanup hooks too
	hookEnv = append(hookEnv, splitLines(env.RunEnv)...)

	// 1. setup hook
	if err := e.runHook(ctx, lw, "setup", env.SetupScript, hookEnv); err != nil {
		return "", "", 0, fmt.Errorf("setup hook: %w", err)
	}

	// 2. acquire source
	var commit string
	if env.SourceType == store.SourceGit || env.DeployType == store.DeployCompose {
		if env.RepoURL == "" {
			return "", "", 0, fmt.Errorf("git repo URL is required")
		}
		lw.Printf("\n== Fetching source ==\n")
		sha, err := source.Ensure(ctx, lw, repoDir, env.RepoURL, env.GitRef, env.GitToken)
		if err != nil {
			return "", "", 0, err
		}
		commit = sha
	}

	var digest string
	var err error
	if env.DeployType == store.DeployCompose {
		err = e.deployCompose(ctx, lw, env, repoDir)
	} else {
		digest, err = e.deployContainer(ctx, lw, env, repoDir)
	}
	if err != nil {
		return commit, digest, 0, err
	}

	// cleanup hook
	if herr := e.runHook(ctx, lw, "cleanup", env.CleanupScript, hookEnv); herr != nil {
		lw.Printf("warning: cleanup hook failed: %v\n", herr)
	}

	// 7. health check — defines "ready" and the start duration
	readyMs, err := e.runHealthCheck(ctx, lw, env, start)
	if err != nil {
		return commit, digest, readyMs, err
	}

	// prune
	if env.PruneImages {
		lw.Printf("\n== Pruning dangling images ==\n")
		_ = e.dk.PruneDanglingImages(ctx, lw)
	}
	return commit, digest, readyMs, nil
}

// deployContainer builds-or-pulls a single image and (re)creates one container.
func (e *Engine) deployContainer(ctx context.Context, lw *logWriter, env *store.Environment, repoDir string) (string, error) {
	image := env.ImageName
	switch env.SourceType {
	case store.SourceRegistry:
		image = env.RegistryImage
		if image == "" {
			return "", fmt.Errorf("registry image is required")
		}
		if env.RegistryUsername != "" {
			lw.Printf("\n== Registry login ==\n")
			if err := e.dk.Login(ctx, lw, registryHost(image), env.RegistryUsername, env.RegistryPassword); err != nil {
				return "", fmt.Errorf("registry login: %w", err)
			}
		}
		lw.Printf("\n== Pulling image ==\n")
		if err := e.dk.Run(ctx, lw, docker.RunOpts{}, "pull", image); err != nil {
			return "", fmt.Errorf("docker pull: %w", err)
		}
	case store.SourceDockerfile:
		// Build from an inline Dockerfile with no repo. The Dockerfile must be
		// self-contained (embed any scripts via base64), since the build context
		// is just the generated Dockerfile.
		if image == "" {
			image = "edp/" + sanitize(env.Name) + ":latest"
		}
		if strings.TrimSpace(env.DockerfileContent) == "" {
			return "", fmt.Errorf("dockerfile source requires dockerfile_content")
		}
		if err := os.MkdirAll(repoDir, 0o755); err != nil {
			return "", fmt.Errorf("create build dir: %w", err)
		}
		dfPath := filepath.Join(repoDir, "Dockerfile")
		if err := os.WriteFile(dfPath, []byte(env.DockerfileContent), 0o644); err != nil {
			return "", fmt.Errorf("write Dockerfile: %w", err)
		}
		lw.Printf("\n== Building %s from inline Dockerfile (%d bytes) ==\n", image, len(env.DockerfileContent))
		if err := e.dk.Run(ctx, lw, docker.RunOpts{Dir: repoDir}, "build", "-t", image, repoDir); err != nil {
			return "", fmt.Errorf("docker build: %w", err)
		}
	default: // git build
		if image == "" {
			image = "edp/" + sanitize(env.Name) + ":latest"
		}
		lw.Printf("\n== Building image %s ==\n", image)
		buildArgs := []string{"build", "-t", image}
		dockerfile := env.DockerfilePath
		// A custom inline Dockerfile overrides whatever the repo ships.
		if strings.TrimSpace(env.DockerfileContent) != "" {
			path := filepath.Join(repoDir, ".edp.Dockerfile")
			if err := os.WriteFile(path, []byte(env.DockerfileContent), 0o644); err != nil {
				return "", fmt.Errorf("write custom Dockerfile: %w", err)
			}
			lw.Printf("using custom Dockerfile (%d bytes)\n", len(env.DockerfileContent))
			dockerfile = path
		}
		if dockerfile != "" {
			if !filepath.IsAbs(dockerfile) {
				dockerfile = filepath.Join(repoDir, dockerfile)
			}
			buildArgs = append(buildArgs, "-f", dockerfile)
		}
		bctx := repoDir
		if env.BuildContext != "" && env.BuildContext != "." {
			bctx = filepath.Join(repoDir, env.BuildContext)
		}
		buildArgs = append(buildArgs, bctx)
		if err := e.dk.Run(ctx, lw, docker.RunOpts{Dir: repoDir}, buildArgs...); err != nil {
			return "", fmt.Errorf("docker build: %w", err)
		}
	}

	// remove old containers for this env
	lw.Printf("\n== Replacing container ==\n")
	if err := e.dk.RemoveContainersByEnv(ctx, lw, env.ID); err != nil {
		lw.Printf("warning: removing old containers: %v\n", err)
	}

	// volume sweep (after removal so named volumes are free)
	e.sweepVolumes(ctx, lw, env)

	// create + start new container
	runArgs := e.containerRunArgs(env, image)
	if err := e.dk.Run(ctx, lw, docker.RunOpts{}, runArgs...); err != nil {
		return "", fmt.Errorf("docker run: %w", err)
	}

	// connect any additional networks beyond the first
	nets := splitList(env.RunNetworks)
	for _, n := range nets[min(1, len(nets)):] {
		_ = e.dk.Run(ctx, lw, docker.RunOpts{}, "network", "connect", n, containerName(env))
	}

	// attach to the shared network so edp's reverse proxy can reach it by name
	if env.ProxyPort != "" {
		if err := e.dk.EnsureNetwork(ctx); err == nil {
			if err := e.dk.ConnectShared(ctx, containerName(env)); err != nil {
				lw.Printf("warning: could not attach to proxy network: %v\n", err)
			} else {
				lw.Printf("attached to shared network %q for proxying\n", docker.SharedNetwork)
			}
		}
	}

	return e.dk.ImageDigest(ctx, image), nil
}

// containerRunArgs assembles the `docker run` invocation from the env config.
func (e *Engine) containerRunArgs(env *store.Environment, image string) []string {
	name := containerName(env)
	args := []string{"run", "-d", "--name", name,
		"--label", docker.LabelManaged + "=true",
		"--label", fmt.Sprintf("%s=%d", docker.LabelEnv, env.ID)}
	if env.RestartPolicy != "" {
		args = append(args, "--restart", env.RestartPolicy)
	}
	// An inline entrypoint script runs as `<interpreter> -c <script>`; the
	// Entrypoint field chooses the interpreter (default /bin/sh).
	script := strings.TrimSpace(env.EntrypointScript)
	entrypoint := strings.TrimSpace(env.Entrypoint)
	if script != "" && entrypoint == "" {
		entrypoint = "/bin/sh"
	}
	if entrypoint != "" {
		args = append(args, "--entrypoint", entrypoint)
	}
	for _, p := range splitList(env.RunPorts) {
		args = append(args, "-p", p)
	}
	for _, kv := range splitLines(env.RunEnv) {
		args = append(args, "-e", kv)
	}
	for _, v := range splitLines(env.RunVolumes) {
		args = append(args, "-v", v)
	}
	if nets := splitList(env.RunNetworks); len(nets) > 0 {
		args = append(args, "--network", nets[0])
	}
	args = append(args, image)
	if script != "" {
		args = append(args, "-c", env.EntrypointScript) // the script body, verbatim
	} else {
		// a command override becomes the container's args (overriding the image's CMD)
		args = append(args, splitArgs(env.Command)...)
	}
	return args
}

// splitArgs tokenizes a command string into argv, honoring single and double
// quotes so values with spaces survive (e.g. `sh -c "echo hi there"`).
func splitArgs(s string) []string {
	var args []string
	var cur strings.Builder
	var quote rune
	inToken := false
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			inToken = true
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if inToken {
				args = append(args, cur.String())
				cur.Reset()
				inToken = false
			}
		default:
			cur.WriteRune(r)
			inToken = true
		}
	}
	if inToken {
		args = append(args, cur.String())
	}
	return args
}

// deployCompose brings up a compose stack for the env.
func (e *Engine) deployCompose(ctx context.Context, lw *logWriter, env *store.Environment, repoDir string) error {
	project := composeProject(env)
	file := filepath.Join(repoDir, firstNonEmpty(env.ComposePath, "docker-compose.yml"))

	// compose file selection: base file, plus an optional inline override merged
	// on top (docker compose merges left-to-right via repeated -f).
	files := []string{"-f", file}
	if strings.TrimSpace(env.ComposeOverride) != "" {
		ovr := filepath.Join(repoDir, ".edp.compose.override.yml")
		if err := os.WriteFile(ovr, []byte(env.ComposeOverride), 0o644); err != nil {
			return fmt.Errorf("write compose override: %w", err)
		}
		lw.Printf("merging compose override (%d bytes)\n", len(env.ComposeOverride))
		files = append(files, "-f", ovr)
	}

	compose := func(extra ...string) []string {
		c := append([]string{"compose", "-p", project}, files...)
		return append(c, extra...)
	}
	// The env's variables are put in the compose process environment so `${VAR}`
	// interpolation and services that reference them resolve correctly.
	opts := docker.RunOpts{Dir: repoDir, Env: splitLines(env.RunEnv)}

	if env.VolumeSweep != store.SweepNone {
		lw.Printf("\n== Tearing down stack (volume sweep=%s) ==\n", env.VolumeSweep)
		_ = e.dk.Run(ctx, lw, opts, compose("down", "-v", "--remove-orphans")...)
	}

	lw.Printf("\n== Bringing up stack ==\n")
	if err := e.dk.Run(ctx, lw, opts, compose("up", "-d", "--build", "--remove-orphans")...); err != nil {
		return fmt.Errorf("compose up: %w", err)
	}
	return nil
}

// sweepVolumes removes the env's volumes according to its policy (container mode).
func (e *Engine) sweepVolumes(ctx context.Context, lw *logWriter, env *store.Environment) {
	if env.VolumeSweep == store.SweepNone {
		return
	}
	lw.Printf("\n== Volume sweep (%s) ==\n", env.VolumeSweep)
	// named volumes referenced in the run config (skip bind mounts)
	var named []string
	for _, v := range splitLines(env.RunVolumes) {
		src := strings.SplitN(v, ":", 2)[0]
		if src != "" && !strings.ContainsAny(src, "/\\.") {
			named = append(named, src)
		}
	}
	e.dk.RemoveNamedVolumes(ctx, lw, named)
	if env.VolumeSweep == store.SweepAll {
		_ = e.dk.RemoveVolumesByLabel(ctx, lw, fmt.Sprintf("%s=%d", docker.LabelEnv, env.ID))
	}
}

// runHook executes a user lifecycle script as Python (`python3 -c`) with the
// EDP_* variables injected into os.environ. Python keeps hooks cross-platform
// (Linux container and Windows dev host) and gives users a real scripting
// language for setup/cleanup logic.
func (e *Engine) runHook(ctx context.Context, lw *logWriter, name, script string, env []string) error {
	script = strings.TrimSpace(script)
	if script == "" {
		return nil
	}
	lw.Printf("\n== %s hook ==\n", name)
	return sh.Stream(ctx, lw, sh.Opts{Env: env, EchoArgs: []string{name + " hook (python)"}}, e.pythonBin, "-c", script)
}

// ---- helpers ----

func sanitize(s string) string { return naming.Sanitize(s) }

func containerName(env *store.Environment) string { return naming.ContainerName(env.Name) }

func composeProject(env *store.Environment) string { return naming.ComposeProject(env.ID) }

func registryHost(image string) string {
	parts := strings.SplitN(image, "/", 2)
	if len(parts) == 2 && strings.ContainsAny(parts[0], ".:") {
		return parts[0]
	}
	return "" // Docker Hub
}

func splitList(s string) []string {
	var out []string
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' }) {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

func splitLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
