package reuse

import (
	"regexp"
	"strings"
)

var (
	nonAlphaNumericRegex = regexp.MustCompile(`[^a-z0-9]+`)
	multipleDashRegex    = regexp.MustCompile(`-+`)
)

// Slugify converts an arbitrary string into a clean, URL-friendly slug
func Slugify(text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	withDashes := nonAlphaNumericRegex.ReplaceAllString(lower, "-")
	cleanDashes := multipleDashRegex.ReplaceAllLiteralString(withDashes, "-")
	trimmed := strings.Trim(cleanDashes, "-")
	if trimmed == "" {
		return "item"
	}
	return trimmed
}