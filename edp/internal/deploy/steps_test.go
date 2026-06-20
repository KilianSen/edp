package deploy

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"edp/internal/docker"
	"edp/internal/store"
)

func TestSplitArgs(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"npm start", []string{"npm", "start"}},
		{"  spaced   out  ", []string{"spaced", "out"}},
		{`-c "echo hi there"`, []string{"-c", "echo hi there"}},
		{`sh -c 'a b c'`, []string{"sh", "-c", "a b c"}},
		{`--flag="quoted value" plain`, []string{"--flag=quoted value", "plain"}},
		{`"leading" mid 'trail'`, []string{"leading", "mid", "trail"}},
	}
	for _, c := range cases {
		if got := splitArgs(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitArgs(%q) = %#v, want %#v", c.in, got, c.want)
		}
	}
}

// containsArg reports whether args contains the exact value v.
func containsArg(args []string, v string) bool { return slices.Contains(args, v) }

// argAfter returns the argument following the first occurrence of flag, or "".
func argAfter(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func TestContainerRunArgs(t *testing.T) {
	e := &Engine{dk: docker.New("docker", "inst-123")}

	// command override: appended after the image, tokenized
	a := e.containerRunArgs(&store.Environment{Name: "x", Command: `-c "echo hi"`}, "img")
	if a[len(a)-3] != "img" || a[len(a)-2] != "-c" || a[len(a)-1] != "echo hi" {
		t.Errorf("command override tail wrong: %#v", a[len(a)-3:])
	}

	// the instance label is stamped so teardown can find this container
	if got := argAfter(a, "--label"); !containsArg(a, docker.LabelInstance+"=inst-123") {
		t.Errorf("missing instance label; first label=%q args=%#v", got, a)
	}

	// entrypoint script: --entrypoint defaults to /bin/sh, script passed via -c verbatim
	script := "echo a\necho b"
	s := e.containerRunArgs(&store.Environment{Name: "x", EntrypointScript: script}, "img")
	if got := argAfter(s, "--entrypoint"); got != "/bin/sh" {
		t.Errorf("default interpreter = %q, want /bin/sh", got)
	}
	if s[len(s)-3] != "img" || s[len(s)-2] != "-c" || s[len(s)-1] != script {
		t.Errorf("script tail wrong: %#v", s[len(s)-3:])
	}

	// custom interpreter via the Entrypoint field
	b := e.containerRunArgs(&store.Environment{Name: "x", Entrypoint: "/bin/bash", EntrypointScript: "echo hi"}, "img")
	if got := argAfter(b, "--entrypoint"); got != "/bin/bash" {
		t.Errorf("interpreter = %q, want /bin/bash", got)
	}
	if strings.Join(b, " ") == "" {
		t.Fatal("unreachable")
	}
}
