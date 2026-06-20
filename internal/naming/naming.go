// Package naming centralizes how edp derives Docker object names from an
// environment, so the deploy engine and timed-hook runner always agree on the
// container name / compose project a script should target.
package naming

import (
	"fmt"
	"regexp"
	"strings"
)

var re = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)

// Sanitize lowercases and replaces invalid Docker-name characters with '-'.
func Sanitize(s string) string {
	s = re.ReplaceAllString(strings.ToLower(s), "-")
	return strings.Trim(s, "-._")
}

// ContainerName is the container created for a single-container environment.
func ContainerName(envName string) string { return "edp-" + Sanitize(envName) }

// ComposeProject is the compose project name for a stack environment.
func ComposeProject(envID int64) string { return fmt.Sprintf("edp-%d", envID) }
