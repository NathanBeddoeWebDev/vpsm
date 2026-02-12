package util

import "strings"

// NormalizeKey lowercases and trims a string for use as a consistent lookup key.
func NormalizeKey(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
