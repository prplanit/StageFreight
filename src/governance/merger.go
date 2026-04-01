package governance

// MergeConfigs deep-merges managed and local configs.
// Local always wins on conflict.
// managedLayer is the base layer number for managed entries (preset layers come before).
// Returns merged config + trace showing source of every value.
func MergeConfigs(managed, local map[string]any, managedLayer int) (map[string]any, MergeTrace) {
	var trace MergeTrace
	localLayer := managedLayer + 1

	if managed == nil && local == nil {
		return map[string]any{}, trace
	}
	if managed == nil {
		traceAll(&trace, local, "local", localLayer)
		return copyMap(local), trace
	}
	if local == nil {
		traceAll(&trace, managed, "managed", managedLayer)
		return copyMap(managed), trace
	}

	merged := DeepMerge(managed, local)

	// Build trace: walk managed, record what survived and what was overridden.
	traceMerge(&trace, managed, local, merged, "", managedLayer, localLayer)

	return merged, trace
}

// DeepMerge merges two maps. Override (second arg) wins on conflict.
// Objects: deep merge. Scalars: override replaces. Lists: override replaces.
func DeepMerge(base, override map[string]any) map[string]any {
	result := make(map[string]any, len(base)+len(override))

	// Start with all base keys.
	for k, v := range base {
		result[k] = v
	}

	// Apply overrides.
	for k, overrideVal := range override {
		baseVal, exists := result[k]
		if !exists {
			result[k] = overrideVal
			continue
		}

		// Both exist — check if deep merge applies.
		baseMap, baseIsMap := baseVal.(map[string]any)
		overrideMap, overrideIsMap := overrideVal.(map[string]any)

		if baseIsMap && overrideIsMap {
			// Deep merge objects.
			result[k] = DeepMerge(baseMap, overrideMap)
		} else {
			// Scalar or list: override replaces.
			result[k] = overrideVal
		}
	}

	return result
}

// traceMerge builds a merge trace by comparing managed, local, and merged.
func traceMerge(trace *MergeTrace, managed, local, merged map[string]any, prefix string, managedLayer, localLayer int) {
	for k, mergedVal := range merged {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}

		managedVal, inManaged := managed[k]
		_, inLocal := local[k]

		switch {
		case inManaged && inLocal:
			// Both sources — check if local overrode.
			managedSub, managedIsMap := managedVal.(map[string]any)
			localSub, localIsMap := local[k].(map[string]any)
			mergedSub, mergedIsMap := mergedVal.(map[string]any)

			if managedIsMap && localIsMap && mergedIsMap {
				traceMerge(trace, managedSub, localSub, mergedSub, path, managedLayer, localLayer)
			} else {
				// Scalar/list conflict — local won.
				// Record the managed value as overridden.
				trace.Entries = append(trace.Entries, MergeEntry{
					Path:         path,
					Source:       "managed",
					Layer:        managedLayer,
					Operation:    "set",
					Value:        managedVal,
					Overridden:   true,
					OverriddenBy: "local",
				})
				// Record the local value as the winner.
				op := "override"
				if _, isList := mergedVal.([]any); isList {
					op = "replace"
				}
				trace.Entries = append(trace.Entries, MergeEntry{
					Path:      path,
					Source:    "local",
					Layer:     localLayer,
					Operation: op,
					Value:     mergedVal,
				})
			}

		case inLocal:
			trace.Entries = append(trace.Entries, MergeEntry{
				Path:      path,
				Source:    "local",
				Layer:     localLayer,
				Operation: "set",
				Value:     mergedVal,
			})

		case inManaged:
			trace.Entries = append(trace.Entries, MergeEntry{
				Path:      path,
				Source:    "managed",
				Layer:     managedLayer,
				Operation: "set",
				Value:     mergedVal,
			})
		}
	}
}

// traceAll records all keys from a single source.
func traceAll(trace *MergeTrace, m map[string]any, source string, layer int) {
	for k, v := range m {
		trace.Entries = append(trace.Entries, MergeEntry{
			Path:      k,
			Source:    source,
			Layer:     layer,
			Operation: "set",
			Value:     v,
		})
	}
}

// copyMap returns a shallow copy.
func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
