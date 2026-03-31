package config

import (
	"fmt"
	"net/url"
	"strings"
)

// SourcesConfig holds the build source definitions and mirror forge declarations.
// The primary source is the authoritative forge. Mirrors are strict downstream
// replicas synchronized outward from the primary.
// PublishOrigin declares where rendered artifacts are served from.
type SourcesConfig struct {
	Primary       SourceConfig   `yaml:"primary"`
	Mirrors       []MirrorConfig `yaml:"mirrors,omitempty"`
	PublishOrigin *PublishOrigin  `yaml:"publish_origin,omitempty"`
}

// SourceConfig defines the primary (authoritative) build source.
type SourceConfig struct {
	Kind          string `yaml:"kind"`           // source type (default: "git")
	Worktree      string `yaml:"worktree"`       // path to working tree (default: ".")
	URL           string `yaml:"url"`            // optional: enables deterministic link_base/raw_base derivation
	DefaultBranch string `yaml:"default_branch"` // optional: used with URL for raw_base derivation
}

// MirrorConfig declares a downstream forge replica that receives projections
// from the primary. A mirror is strict — it never originates state.
// Directionality is enforced: source → mirror only.
type MirrorConfig struct {
	ID          string     `yaml:"id"`          // unique identifier (e.g., "github")
	Provider    string     `yaml:"provider"`    // forge provider: github, gitlab, gitea
	URL         string     `yaml:"url"`         // forge base URL (e.g., "https://github.com")
	ProjectID   string     `yaml:"project_id"`  // owner/repo or numeric ID on the mirror forge
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

// PublishOrigin declares where rendered artifacts (badges, etc.) are served from.
// Three kinds:
//   - primary: derives raw content URL from sources.primary (the authoritative forge)
//   - mirror:  derives raw content URL from a sources.mirrors[] entry
//   - url:     explicit base URL (CDN, S3, RGW, any static hosting)
//
// Mirrors MUST track the primary branch. The branch used for raw URL construction
// always comes from sources.primary.default_branch — for both primary and mirror kinds.
type PublishOrigin struct {
	Kind string `yaml:"kind"`           // "primary", "mirror", or "url"
	Ref  string `yaml:"ref,omitempty"`  // mirror ID (kind: mirror only)
	Base string `yaml:"base,omitempty"` // explicit URL (kind: url only)
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

// ResolvePublishOrigin resolves the serving base URL for rendered artifacts.
// Hard fails if publish_origin is nil or misconfigured.
// Config values may contain {var:*} templates — these are resolved using cfg.Vars.
func ResolvePublishOrigin(cfg *Config) (string, error) {
	po := cfg.Sources.PublishOrigin
	if po == nil {
		return "", fmt.Errorf("sources.publish_origin is required")
	}
	branch := resolveVars(cfg.Sources.Primary.DefaultBranch, cfg.Vars)
	if branch == "" {
		return "", fmt.Errorf("sources.primary.default_branch is required when publish_origin is used")
	}
	switch po.Kind {
	case "primary":
		srcURL := resolveVars(cfg.Sources.Primary.URL, cfg.Vars)
		if srcURL == "" {
			return "", fmt.Errorf("sources.publish_origin (kind: primary): sources.primary.url is required")
		}
		provider, baseURL, projectID, err := ParseForgeURL(srcURL)
		if err != nil {
			return "", fmt.Errorf("sources.publish_origin (kind: primary): %w", err)
		}
		return ForgeRawBase(provider, baseURL, projectID, branch)
	case "mirror":
		if po.Ref == "" {
			return "", fmt.Errorf("sources.publish_origin (kind: mirror): ref is required")
		}
		mirror := FindMirrorByID(cfg.Sources.Mirrors, po.Ref)
		if mirror == nil {
			return "", fmt.Errorf("sources.publish_origin ref %q: not found in sources.mirrors", po.Ref)
		}
		mirrorURL := resolveVars(mirror.URL, cfg.Vars)
		projectID := resolveVars(mirror.ProjectID, cfg.Vars)
		return ForgeRawBase(mirror.Provider, mirrorURL, projectID, branch)
	case "url":
		base := resolveVars(po.Base, cfg.Vars)
		if base == "" {
			return "", fmt.Errorf("sources.publish_origin (kind: url): base is required")
		}
		return strings.TrimRight(base, "/"), nil
	default:
		return "", fmt.Errorf(
			"sources.publish_origin: unknown kind %q (expected primary, mirror, or url)", po.Kind)
	}
}

// resolveVars performs simple {var:name} template resolution.
// Avoids importing gitver into config package.
func resolveVars(s string, vars map[string]string) string {
	if len(vars) == 0 || !strings.Contains(s, "{var:") {
		return s
	}
	for k, v := range vars {
		s = strings.ReplaceAll(s, "{var:"+k+"}", v)
	}
	return s
}

// ResolveLinkBase resolves the page-link (blob) base URL from publish_origin.
// Same resolution path as ResolvePublishOrigin but returns blob URLs instead of raw URLs.
// Used for resolving relative link paths (e.g., LICENSE → full blob URL).
func ResolveLinkBase(cfg *Config) (string, error) {
	po := cfg.Sources.PublishOrigin
	if po == nil {
		return "", fmt.Errorf("sources.publish_origin is required")
	}
	branch := resolveVars(cfg.Sources.Primary.DefaultBranch, cfg.Vars)
	if branch == "" {
		return "", fmt.Errorf("sources.primary.default_branch is required when publish_origin is used")
	}
	switch po.Kind {
	case "primary":
		srcURL := resolveVars(cfg.Sources.Primary.URL, cfg.Vars)
		if srcURL == "" {
			return "", fmt.Errorf("sources.publish_origin (kind: primary): sources.primary.url is required")
		}
		provider, baseURL, projectID, err := ParseForgeURL(srcURL)
		if err != nil {
			return "", fmt.Errorf("sources.publish_origin (kind: primary): %w", err)
		}
		return ForgeLinkBase(provider, baseURL, projectID, branch)
	case "mirror":
		if po.Ref == "" {
			return "", fmt.Errorf("sources.publish_origin (kind: mirror): ref is required")
		}
		mirror := FindMirrorByID(cfg.Sources.Mirrors, po.Ref)
		if mirror == nil {
			return "", fmt.Errorf("sources.publish_origin ref %q: not found in sources.mirrors", po.Ref)
		}
		mirrorURL := resolveVars(mirror.URL, cfg.Vars)
		projectID := resolveVars(mirror.ProjectID, cfg.Vars)
		return ForgeLinkBase(mirror.Provider, mirrorURL, projectID, branch)
	case "url":
		// kind: url has no forge concept — no blob URL derivable.
		// Return empty — callers should handle relative links as-is.
		return "", nil
	default:
		return "", fmt.Errorf(
			"sources.publish_origin: unknown kind %q (expected primary, mirror, or url)", po.Kind)
	}
}

// FindMirrorByID returns the mirror with the given ID, or nil.
func FindMirrorByID(mirrors []MirrorConfig, id string) *MirrorConfig {
	for i := range mirrors {
		if mirrors[i].ID == id {
			return &mirrors[i]
		}
	}
	return nil
}

// ForgeRawBase constructs a raw content base URL from forge mirror fields.
// Handles GitLab subgroup paths (group/subgroup/project) correctly.
// All inputs are normalized to prevent double-slash artifacts.
func ForgeRawBase(provider, baseURL, projectID, branch string) (string, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	projectID = strings.Trim(projectID, "/")
	branch = strings.Trim(branch, "/")
	switch provider {
	case "github":
		host := strings.Replace(baseURL, "github.com", "raw.githubusercontent.com", 1)
		return fmt.Sprintf("%s/%s/%s", host, projectID, branch), nil
	case "gitlab":
		// Works for subgroups: gitlab.com/group/subgroup/repo/-/raw/main
		return fmt.Sprintf("%s/%s/-/raw/%s", baseURL, projectID, branch), nil
	case "gitea":
		return fmt.Sprintf("%s/%s/raw/branch/%s", baseURL, projectID, branch), nil
	default:
		return "", fmt.Errorf("unsupported forge provider %q for raw URL derivation", provider)
	}
}

// ForgeLinkBase constructs a page-link (blob) base URL from forge fields.
// Used for resolving relative links (e.g., LICENSE → blob URL).
func ForgeLinkBase(provider, baseURL, projectID, branch string) (string, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	projectID = strings.Trim(projectID, "/")
	branch = strings.Trim(branch, "/")
	switch provider {
	case "github":
		return fmt.Sprintf("%s/%s/blob/%s", baseURL, projectID, branch), nil
	case "gitlab":
		return fmt.Sprintf("%s/%s/-/blob/%s", baseURL, projectID, branch), nil
	case "gitea":
		return fmt.Sprintf("%s/%s/src/branch/%s", baseURL, projectID, branch), nil
	default:
		return "", fmt.Errorf("unsupported forge provider %q for link base derivation", provider)
	}
}

// ParseForgeURL detects forge provider from a full repository URL,
// extracts base URL and project ID.
// Examples:
//
//	"https://github.com/PrPlanIT/StageFreight"       → github, "https://github.com", "PrPlanIT/StageFreight"
//	"https://gitlab.prplanit.com/SoFMeRight/dungeon" → gitlab, "https://gitlab.prplanit.com", "SoFMeRight/dungeon"
func ParseForgeURL(rawURL string) (provider, baseURL, projectID string, err error) {
	u, err := url.Parse(strings.TrimRight(rawURL, "/"))
	if err != nil {
		return "", "", "", fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	path := strings.TrimPrefix(u.Path, "/")
	if path == "" {
		return "", "", "", fmt.Errorf("URL %q has no project path", rawURL)
	}
	base := u.Scheme + "://" + u.Host
	if strings.Contains(u.Host, "github.com") {
		return "github", base, path, nil
	}
	if strings.Contains(u.Host, "gitlab") {
		return "gitlab", base, path, nil
	}
	if strings.Contains(u.Host, "gitea") || strings.Contains(u.Host, "codeberg") {
		return "gitea", base, path, nil
	}
	return "", "", "", fmt.Errorf(
		"cannot detect forge provider from URL %q — use kind: mirror (with explicit provider) or kind: url instead",
		rawURL)
}
