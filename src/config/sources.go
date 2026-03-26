package config

// SourcesConfig holds the build source definitions and accessory forge declarations.
// The primary source is the authoritative forge. Accessories are derived mirrors
// synchronized outward from the primary.
type SourcesConfig struct {
	Primary     SourceConfig      `yaml:"primary"`
	Accessories []AccessoryConfig `yaml:"accessories,omitempty"`
}

// SourceConfig defines the primary (authoritative) build source.
type SourceConfig struct {
	Kind          string `yaml:"kind"`           // source type (default: "git")
	Worktree      string `yaml:"worktree"`       // path to working tree (default: ".")
	URL           string `yaml:"url"`            // optional: enables deterministic link_base/raw_base derivation
	DefaultBranch string `yaml:"default_branch"` // optional: used with URL for raw_base derivation
}

// AccessoryConfig declares a derived forge that receives projections from the primary.
// An accessory is a strict mirror — it never originates state. Directionality is
// enforced: source → accessory only.
type AccessoryConfig struct {
	ID          string     `yaml:"id"`          // unique identifier (e.g., "github")
	Provider    string     `yaml:"provider"`    // forge provider: github, gitlab, gitea
	URL         string     `yaml:"url"`         // forge base URL (e.g., "https://github.com")
	ProjectID   string     `yaml:"project_id"`  // owner/repo or numeric ID on the accessory forge
	Credentials string     `yaml:"credentials"` // env var prefix for token resolution
	Sync        SyncConfig `yaml:"sync"`        // which domains to synchronize
}

// SyncConfig declares which sync domains an accessory receives.
// Git mirror is the foundation; artifact projection is subordinate and
// gated on mirror success for the same accessory.
type SyncConfig struct {
	// Git enables authoritative mirror replication via git push --mirror.
	// All refs, branches, tags, deletions, force updates. This is the
	// foundation — artifact sync runs only after mirror succeeds.
	Git bool `yaml:"git,omitempty"`

	// Releases enables forge-native release projection (notes, assets, links).
	// Runs after git mirror succeeds. Tag is the identity key.
	Releases bool `yaml:"releases,omitempty"`

	// Docs enables README/doc file projection via forge commit API.
	// Mutually exclusive with Git (docs arrive through git mirror).
	// Only valid when Git is false.
	Docs bool `yaml:"docs,omitempty"`
}

// DefaultSourcesConfig returns sensible defaults for source configuration.
func DefaultSourcesConfig() SourcesConfig {
	return SourcesConfig{
		Primary: SourceConfig{
			Kind:     "git",
			Worktree: ".",
		},
	}
}
