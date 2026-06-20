// Package bootstrap seeds the instance registry from a declarative YAML file on
// startup. It is idempotent — instances whose label already exists are left
// untouched — so the file can stay mounted for infra-as-code provisioning while
// the API still allows dynamic add/remove at runtime.
package bootstrap

import (
	"context"
	"fmt"
	"log"
	"os"

	"edp-manager/internal/store"

	"gopkg.in/yaml.v3"
)

type fileSpec struct {
	Instances []instanceSpec `yaml:"instances"`
}

type instanceSpec struct {
	Label    string `yaml:"label"`
	BaseURL  string `yaml:"base_url"`
	Token    string `yaml:"token"`     // inline token (discouraged; prefer token_env)
	TokenEnv string `yaml:"token_env"` // name of an env var holding the token
}

// SeedFromFile loads path and creates any instances whose label isn't already
// registered. A missing file is not an error (nothing to seed).
func SeedFromFile(ctx context.Context, st *store.Store, path string) error {
	if path == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("config: %s not found; skipping seed", path)
			return nil
		}
		return err
	}
	var spec fileSpec
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	created := 0
	for _, in := range spec.Instances {
		if in.Label == "" || in.BaseURL == "" {
			log.Printf("config: skipping instance with empty label/base_url")
			continue
		}
		existing, err := st.InstanceByLabel(ctx, in.Label)
		if err != nil {
			return err
		}
		if existing != nil {
			continue // idempotent: leave configured instances untouched
		}
		token := in.Token
		if token == "" && in.TokenEnv != "" {
			token = os.Getenv(in.TokenEnv)
			if token == "" {
				log.Printf("config: instance %q: env %s is empty", in.Label, in.TokenEnv)
			}
		}
		inst := &store.Instance{Label: in.Label, BaseURL: in.BaseURL, APIToken: token}
		if err := st.CreateInstance(ctx, inst); err != nil {
			return fmt.Errorf("seed instance %q: %w", in.Label, err)
		}
		created++
	}
	if created > 0 {
		log.Printf("config: seeded %d instance(s) from %s", created, path)
	}
	return nil
}
