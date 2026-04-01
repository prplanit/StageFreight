package governance

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// RenderManagedConfig produces the managed YAML file content for a repo.
// Uses canonical key ordering for deterministic, diff-stable output.
// Stamps metadata header with source provenance.
func RenderManagedConfig(managed ManagedConfig) ([]byte, error) {
	var b strings.Builder

	// Metadata header.
	b.WriteString("# MANAGED BY STAGEFREIGHT GOVERNANCE — DO NOT EDIT\n")
	b.WriteString(fmt.Sprintf("# Source: %s %s\n", managed.Source, managed.Ref))
	b.WriteString(fmt.Sprintf("# Cluster: %s\n", managed.ClusterID))
	b.WriteString(fmt.Sprintf("# Generated: %s\n", managed.GeneratedAt))
	b.WriteString("\n")

	// Render config in canonical key order.
	body, err := renderCanonical(managed.Config)
	if err != nil {
		return nil, fmt.Errorf("rendering managed config: %w", err)
	}
	b.Write(body)

	return []byte(b.String()), nil
}

// renderCanonical serializes a config map with fixed top-level key order.
// Keys not in CanonicalKeyOrder are appended alphabetically at the end.
func renderCanonical(config map[string]any) ([]byte, error) {
	// Build ordered node.
	node := &yaml.Node{
		Kind: yaml.MappingNode,
	}

	// First: canonical order.
	added := make(map[string]bool)
	for _, key := range CanonicalKeyOrder {
		val, ok := config[key]
		if !ok {
			continue
		}
		added[key] = true
		if err := appendKeyValue(node, key, val); err != nil {
			return nil, err
		}
	}

	// Then: any remaining keys (alphabetical).
	remaining := make([]string, 0)
	for k := range config {
		if !added[k] {
			remaining = append(remaining, k)
		}
	}
	sortStrings(remaining)
	for _, key := range remaining {
		if err := appendKeyValue(node, key, config[key]); err != nil {
			return nil, err
		}
	}

	// Marshal the ordered document.
	doc := &yaml.Node{
		Kind:    yaml.DocumentNode,
		Content: []*yaml.Node{node},
	}

	var b strings.Builder
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	enc.Close()

	return []byte(b.String()), nil
}

// appendKeyValue adds a key-value pair to a yaml mapping node.
func appendKeyValue(node *yaml.Node, key string, val any) error {
	keyNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: key,
	}

	valNode := &yaml.Node{}
	valBytes, err := yaml.Marshal(val)
	if err != nil {
		return fmt.Errorf("marshaling %s: %w", key, err)
	}
	if err := yaml.Unmarshal(valBytes, valNode); err != nil {
		return fmt.Errorf("unmarshaling %s node: %w", key, err)
	}

	// yaml.Unmarshal wraps in a document node — unwrap.
	if valNode.Kind == yaml.DocumentNode && len(valNode.Content) > 0 {
		valNode = valNode.Content[0]
	}

	node.Content = append(node.Content, keyNode, valNode)
	return nil
}

// sortStrings sorts a string slice in place (simple insertion sort for small slices).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
