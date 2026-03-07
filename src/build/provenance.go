package build

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ProvenanceStatement follows the in-toto Statement v1 / SLSA Provenance v1
// structure. Not full SLSA compliance, but a useful provenance document that
// can evolve into DSSE envelopes, cosign attestations, or OCI referrer artifacts.
type ProvenanceStatement struct {
	Type          string              `json:"_type"`
	PredicateType string              `json:"predicateType"`
	Subject       []ProvenanceSubject `json:"subject"`
	Predicate     ProvenancePredicate `json:"predicate"`
}

// ProvenanceSubject identifies what was built.
type ProvenanceSubject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest,omitempty"`
}

// ProvenancePredicate describes how it was built.
type ProvenancePredicate struct {
	BuildType    string                 `json:"buildType"`
	Builder      ProvenanceBuilder      `json:"builder"`
	Invocation   ProvenanceInvocation   `json:"invocation"`
	Metadata     ProvenanceMetadata     `json:"metadata"`
	Materials    []ProvenanceMaterial   `json:"materials,omitempty"`
	StageFreight map[string]any         `json:"stagefreight,omitempty"`
}

// ProvenanceBuilder identifies the build system.
type ProvenanceBuilder struct {
	ID string `json:"id"`
}

// ProvenanceInvocation captures the build parameters and environment.
type ProvenanceInvocation struct {
	ConfigSource map[string]any `json:"configSource,omitempty"`
	Parameters   map[string]any `json:"parameters,omitempty"`
	Environment  map[string]any `json:"environment,omitempty"`
}

// ProvenanceMetadata captures timing and completeness.
type ProvenanceMetadata struct {
	BuildStartedOn  string          `json:"buildStartedOn,omitempty"`
	BuildFinishedOn string          `json:"buildFinishedOn,omitempty"`
	Completeness    map[string]bool `json:"completeness,omitempty"`
	Reproducible    bool            `json:"reproducible"`
}

// ProvenanceMaterial represents an input to the build.
type ProvenanceMaterial struct {
	URI    string            `json:"uri"`
	Digest map[string]string `json:"digest,omitempty"`
}

// WriteProvenance writes a provenance statement as indented JSON.
func WriteProvenance(path string, stmt ProvenanceStatement) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating provenance dir: %w", err)
	}
	data, err := json.MarshalIndent(stmt, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling provenance: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
