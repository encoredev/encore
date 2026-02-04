package urlutil

import "strings"

// JoinURL joins a base URL and a relative path, ensuring exactly one slash.
// It guards against accidental full URLs in relPath.
func JoinURL(base, relPath string) string {
	// Guard: If relPath is actually a full URL, return it as-is to prevent mangling.
	if strings.HasPrefix(relPath, "http://") || strings.HasPrefix(relPath, "https://") {
		return relPath
	}
	// If base is empty, return cleaned relative path to avoid leading slash being interpreted as root
	if strings.TrimSpace(base) == "" {
		return strings.TrimLeft(relPath, "/")
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(relPath, "/")
}
