package release

import (
	"fmt"
	"strconv"
	"strings"
)

// BumpVersion computes the next version from a previous version tag.
// Supports both v-prefixed (v1.2.3) and bare (1.2.3) versions.
// The output preserves the prefix style of the input.
func BumpVersion(prev, kind string) (string, error) {
	if prev == "" {
		return "", fmt.Errorf("cannot bump: no previous version")
	}

	prefix := ""
	version := prev
	if strings.HasPrefix(prev, "v") {
		prefix = "v"
		version = prev[1:]
	}

	// Split on first hyphen to handle prerelease/revision suffixes
	// e.g., "2.20.4-v1" → base="2.20.4", suffix="-v1"
	base := version
	suffix := ""
	if idx := strings.IndexByte(version, '-'); idx >= 0 {
		base = version[:idx]
		suffix = version[idx:]
	}

	parts := strings.SplitN(base, ".", 3)
	if len(parts) != 3 {
		return "", fmt.Errorf("cannot bump %q: expected X.Y.Z format", prev)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", fmt.Errorf("cannot bump %q: invalid major %q", prev, parts[0])
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", fmt.Errorf("cannot bump %q: invalid minor %q", prev, parts[1])
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", fmt.Errorf("cannot bump %q: invalid patch %q", prev, parts[2])
	}

	switch kind {
	case "major":
		major++
		minor = 0
		patch = 0
		suffix = "" // major bump drops suffix
	case "minor":
		minor++
		patch = 0
		suffix = "" // minor bump drops suffix
	case "patch":
		patch++
		// patch preserves suffix structure for revision-based schemes
		// e.g., 2.20.4-v1 → 2.20.5-v1
	default:
		return "", fmt.Errorf("unknown bump kind %q (expected: major, minor, patch)", kind)
	}

	return fmt.Sprintf("%s%d.%d.%d%s", prefix, major, minor, patch, suffix), nil
}
