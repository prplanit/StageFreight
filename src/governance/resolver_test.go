package governance

import (
	"fmt"
	"testing"
)

// fakeLoader is an in-memory preset loader for testing.
type fakeLoader map[string]string

func (f fakeLoader) Load(path string) ([]byte, error) {
	v, ok := f[path]
	if !ok {
		return nil, fmt.Errorf("not found: %s", path)
	}
	return []byte(v), nil
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func requireError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func requireEqual(t *testing.T, expected, actual any, msg string) {
	t.Helper()
	if fmt.Sprintf("%v", expected) != fmt.Sprintf("%v", actual) {
		t.Fatalf("%s: expected %v, got %v", msg, expected, actual)
	}
}

func requireTrue(t *testing.T, val bool, msg string) {
	t.Helper()
	if !val {
		t.Fatal(msg)
	}
}

// --- Resolver Tests ---

func TestResolvePresets_SimplePreset(t *testing.T) {
	loader := fakeLoader{
		"preset/security.yml": `
security:
  enabled: true
  sbom: true
`,
	}

	raw := map[string]any{
		"security": map[string]any{
			"preset": "preset/security.yml",
		},
	}

	resolved, _, err := ResolvePresets(raw, loader, "test@v1", "inline", 0, nil)
	requireNoError(t, err)

	sec := resolved["security"].(map[string]any)
	requireEqual(t, true, sec["enabled"], "security.enabled")
	requireEqual(t, true, sec["sbom"], "security.sbom")
}

func TestResolvePresets_ScalarOverride(t *testing.T) {
	loader := fakeLoader{
		"preset/security.yml": `
security:
  enabled: true
  sbom: true
`,
	}

	raw := map[string]any{
		"security": map[string]any{
			"preset": "preset/security.yml",
			"sbom":   false,
		},
	}

	resolved, entries, err := ResolvePresets(raw, loader, "test@v1", "inline", 0, nil)
	requireNoError(t, err)

	sec := resolved["security"].(map[string]any)
	requireEqual(t, false, sec["sbom"], "local override should win")

	// Check that the preset entry for sbom was marked as overridden.
	foundOverride := false
	for _, e := range entries {
		if e.Path == "security.sbom" && e.Overridden {
			foundOverride = true
		}
	}
	requireTrue(t, foundOverride, "expected override trace for security.sbom")
}

func TestResolvePresets_ListReplacement(t *testing.T) {
	loader := fakeLoader{
		"preset/targets.yml": `
targets:
  items:
    - id: base
`,
	}

	raw := map[string]any{
		"targets": map[string]any{
			"preset": "preset/targets.yml",
			"items":  []any{"override"},
		},
	}

	_, entries, err := ResolvePresets(raw, loader, "test@v1", "inline", 0, nil)
	requireNoError(t, err)

	foundReplace := false
	for _, e := range entries {
		if e.Path == "targets.items" && e.Operation == "replace" {
			foundReplace = true
		}
	}
	requireTrue(t, foundReplace, "expected list replacement to be recorded as 'replace'")
}

func TestResolvePresets_CycleDetection(t *testing.T) {
	loader := fakeLoader{
		"a.yml": `
targets:
  preset: "b.yml"
`,
		"b.yml": `
targets:
  preset: "a.yml"
`,
	}

	raw := map[string]any{
		"targets": map[string]any{
			"preset": "a.yml",
		},
	}

	_, _, err := ResolvePresets(raw, loader, "test@v1", "inline", 0, nil)
	requireError(t, err)
}

func TestResolvePresets_SingleKeyValidation(t *testing.T) {
	loader := fakeLoader{
		"bad.yml": `
targets:
  items: []
badges:
  items: []
`,
	}

	raw := map[string]any{
		"targets": map[string]any{
			"preset": "bad.yml",
		},
	}

	_, _, err := ResolvePresets(raw, loader, "test@v1", "inline", 0, nil)
	requireError(t, err) // must reject multi-key presets
}

func TestResolvePresets_KeyMismatch(t *testing.T) {
	loader := fakeLoader{
		"wrong.yml": `
badges:
  items: []
`,
	}

	raw := map[string]any{
		"targets": map[string]any{
			"preset": "wrong.yml",
		},
	}

	_, _, err := ResolvePresets(raw, loader, "test@v1", "inline", 0, nil)
	requireError(t, err) // preset declares "badges" but imported into "targets"
}

func TestResolvePresets_SourceRefNotConcatenated(t *testing.T) {
	loader := fakeLoader{
		"preset/security.yml": `
security:
  enabled: true
`,
	}

	raw := map[string]any{
		"security": map[string]any{
			"preset": "preset/security.yml",
		},
	}

	_, entries, err := ResolvePresets(raw, loader, "PrPlanIT/MaintenancePolicy@v1.0.0", "inline", 0, nil)
	requireNoError(t, err)

	for _, e := range entries {
		if e.Source == "preset:preset/security.yml" {
			requireEqual(t, "PrPlanIT/MaintenancePolicy@v1.0.0", e.SourceRef,
				"sourceRef should be repo identity, not concatenated path")
		}
	}
}

// --- Merger Tests ---

func TestMergeConfigs_LocalWins(t *testing.T) {
	managed := map[string]any{
		"security": map[string]any{
			"sbom": true,
		},
	}

	local := map[string]any{
		"security": map[string]any{
			"sbom": false,
		},
	}

	merged, trace := MergeConfigs(managed, local, 0)

	sbom := merged["security"].(map[string]any)["sbom"]
	requireEqual(t, false, sbom, "local should override managed")

	foundOverride := false
	for _, e := range trace.Entries {
		if e.Path == "security.sbom" && e.Overridden {
			foundOverride = true
		}
	}
	requireTrue(t, foundOverride, "expected override trace for security.sbom")
}

func TestMergeConfigs_LayerNumbers(t *testing.T) {
	managed := map[string]any{
		"security": map[string]any{
			"sbom": true,
		},
	}

	local := map[string]any{
		"security": map[string]any{
			"sbom": false,
		},
	}

	_, trace := MergeConfigs(managed, local, 5) // preset layers 0-4, managed=5, local=6

	for _, e := range trace.Entries {
		if e.Path == "security.sbom" && e.Source == "managed" {
			requireEqual(t, 5, e.Layer, "managed layer")
		}
		if e.Path == "security.sbom" && e.Source == "local" {
			requireEqual(t, 6, e.Layer, "local layer")
		}
	}
}

func TestMergeConfigs_DeepMerge(t *testing.T) {
	managed := map[string]any{
		"security": map[string]any{
			"sbom":           true,
			"release_detail": "full",
		},
	}

	local := map[string]any{
		"security": map[string]any{
			"sbom": false,
		},
	}

	merged, _ := MergeConfigs(managed, local, 0)

	sec := merged["security"].(map[string]any)
	requireEqual(t, false, sec["sbom"], "local sbom should win")
	requireEqual(t, "full", sec["release_detail"], "managed release_detail should survive")
}

// --- Renderer Tests ---

func TestRenderManagedConfig_Deterministic(t *testing.T) {
	config := map[string]any{
		"security": map[string]any{"sbom": true},
		"targets":  []any{"a"},
	}

	managed := ManagedConfig{
		Source:      "test",
		Ref:         "v1",
		ClusterID:   "test-cluster",
		GeneratedAt: "2026-01-01T00:00:00Z",
		Config:      config,
	}

	a, err := RenderManagedConfig(managed)
	requireNoError(t, err)

	b, err := RenderManagedConfig(managed)
	requireNoError(t, err)

	requireEqual(t, string(a), string(b), "render must be deterministic")
}

func TestRenderManagedConfig_CanonicalOrder(t *testing.T) {
	config := map[string]any{
		"release":  map[string]any{"enabled": true},
		"security": map[string]any{"sbom": true},
		"targets":  []any{"a"},
	}

	managed := ManagedConfig{
		Source:      "test",
		Ref:         "v1",
		ClusterID:   "test-cluster",
		GeneratedAt: "2026-01-01T00:00:00Z",
		Config:      config,
	}

	out, err := RenderManagedConfig(managed)
	requireNoError(t, err)

	s := string(out)
	// targets should come before security, security before release (canonical order)
	targetsIdx := indexOf(s, "targets:")
	securityIdx := indexOf(s, "security:")
	releaseIdx := indexOf(s, "release:")

	requireTrue(t, targetsIdx < securityIdx, "targets should come before security")
	requireTrue(t, securityIdx < releaseIdx, "security should come before release")
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
