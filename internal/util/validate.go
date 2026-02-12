package util

import (
	"fmt"
	"regexp"
)

// validNameChars matches only alphanumeric characters, hyphens, and periods.
var validNameChars = regexp.MustCompile(`^[a-zA-Z0-9.\-]+$`)

// ValidateServerName checks that a server name conforms to RFC 1123 hostname
// rules as required by Hetzner Cloud:
//   - At least 2 characters
//   - Only alphanumeric characters (a-z, A-Z, 0-9), hyphens (-), and periods (.)
//   - First character must be alphanumeric
//   - Last character must not be a hyphen or period
func ValidateServerName(name string) error {
	if len(name) < 2 {
		return fmt.Errorf("server name must be at least 2 characters, got %d", len(name))
	}

	if !validNameChars.MatchString(name) {
		return fmt.Errorf("server name %q contains invalid characters (only a-z, A-Z, 0-9, hyphens, and periods are allowed)", name)
	}

	first := name[0]
	if !isAlphanumeric(first) {
		return fmt.Errorf("server name must start with an alphanumeric character, got %q", string(first))
	}

	last := name[len(name)-1]
	if last == '-' || last == '.' {
		return fmt.Errorf("server name must not end with a hyphen or period, got %q", string(last))
	}

	return nil
}

func isAlphanumeric(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
