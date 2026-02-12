package util

import (
	"testing"
)

func TestValidateServerName_Valid(t *testing.T) {
	valid := []string{
		"web-1",
		"my.server",
		"a1",
		"web-server-01",
		"prod.web.01",
		"Ab",
		"UPPERCASE",
		"MiXeD123",
		"123numeric",
		"a-b.c-d",
	}
	for _, name := range valid {
		t.Run(name, func(t *testing.T) {
			if err := ValidateServerName(name); err != nil {
				t.Errorf("expected %q to be valid, got error: %v", name, err)
			}
		})
	}
}

func TestValidateServerName_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		wantMsg string
	}{
		{"", "at least 2 characters"},
		{"a", "at least 2 characters"},
		{"this is a test", "invalid characters"},
		{"web server", "invalid characters"},
		{"-web", "must start with an alphanumeric"},
		{".web", "must start with an alphanumeric"},
		{"web-", "must not end with a hyphen"},
		{"web.", "must not end with a hyphen or period"},
		{"hello world!", "invalid characters"},
		{"web@server", "invalid characters"},
		{"name_with_underscores", "invalid characters"},
		{"web\tserver", "invalid characters"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateServerName(tt.name)
			if err == nil {
				t.Errorf("expected %q to be invalid, got nil", tt.name)
				return
			}
			if got := err.Error(); !contains(got, tt.wantMsg) {
				t.Errorf("expected error containing %q, got %q", tt.wantMsg, got)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
