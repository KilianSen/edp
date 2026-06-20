package naming

import "testing"

func TestSanitize(t *testing.T) {
	cases := map[string]string{
		"My App":          "my-app",
		"feature/JIRA-12": "feature-jira-12",
		"  spaces  ":      "spaces",
		"Café_Über":       "caf-_-ber",
		"already-ok.1":    "already-ok.1",
		"---weird---":     "weird",
	}
	for in, want := range cases {
		if got := Sanitize(in); got != want {
			t.Errorf("Sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestContainerName(t *testing.T) {
	if got := ContainerName("My App"); got != "edp-my-app" {
		t.Errorf("got %q", got)
	}
}

func TestComposeProject(t *testing.T) {
	if got := ComposeProject(7); got != "edp-7" {
		t.Errorf("got %q", got)
	}
}
