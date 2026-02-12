package tui

import (
	"strings"
	"testing"

	"nathanbeddoewebdev/vpsm/internal/domain"

	"github.com/google/go-cmp/cmp"
)

func TestBuildServerOptions_FormatsCorrectly(t *testing.T) {
	servers := []domain.Server{
		{
			ID:         "42",
			Name:       "web-server",
			Status:     "running",
			ServerType: "cpx11",
			PublicIPv4: "1.2.3.4",
			Region:     "fsn1",
		},
		{
			ID:         "99",
			Name:       "db-server",
			Status:     "stopped",
			ServerType: "cpx22",
			PublicIPv4: "5.6.7.8",
			Region:     "nbg1",
		},
	}

	options := buildServerOptions(servers)

	if len(options) != 2 {
		t.Fatalf("expected 2 options, got %d", len(options))
	}

	pairs := optionsToPairs(options)

	want := []optionPair{
		{Key: "web-server - running - cpx11 - 1.2.3.4 - fsn1", Value: "42"},
		{Key: "db-server - stopped - cpx22 - 5.6.7.8 - nbg1", Value: "99"},
	}

	if diff := cmp.Diff(want, pairs); diff != "" {
		t.Errorf("unexpected server options (-want +got):\n%s", diff)
	}
}

func TestBuildServerOptions_MinimalFields(t *testing.T) {
	servers := []domain.Server{
		{
			ID:   "1",
			Name: "bare-server",
		},
	}

	options := buildServerOptions(servers)
	pairs := optionsToPairs(options)

	if len(pairs) != 1 {
		t.Fatalf("expected 1 option, got %d", len(pairs))
	}
	if pairs[0].Key != "bare-server" {
		t.Errorf("expected label 'bare-server', got %q", pairs[0].Key)
	}
	if pairs[0].Value != "1" {
		t.Errorf("expected value '1', got %q", pairs[0].Value)
	}
}

func TestServerOptionLabel_AllFields(t *testing.T) {
	s := domain.Server{
		Name:       "web-1",
		Status:     "running",
		ServerType: "cpx11",
		PublicIPv4: "1.2.3.4",
		Region:     "fsn1",
	}

	label := serverOptionLabel(s)
	want := "web-1 - running - cpx11 - 1.2.3.4 - fsn1"
	if label != want {
		t.Errorf("serverOptionLabel() = %q, want %q", label, want)
	}
}

func TestServerOptionLabel_NameOnly(t *testing.T) {
	s := domain.Server{Name: "minimal"}
	label := serverOptionLabel(s)
	if label != "minimal" {
		t.Errorf("serverOptionLabel() = %q, want %q", label, "minimal")
	}
}

func TestBuildDeleteSummary_AllFields(t *testing.T) {
	s := domain.Server{
		ID:         "42",
		Name:       "web-server",
		Status:     "running",
		ServerType: "cpx11",
		Image:      "ubuntu-24.04",
		Region:     "fsn1",
		PublicIPv4: "1.2.3.4",
		PublicIPv6: "2001:db8::",
	}

	summary := buildDeleteSummary(s)

	expected := []string{
		"ID: 42",
		"Name: web-server",
		"Status: running",
		"Type: cpx11",
		"Image: ubuntu-24.04",
		"Region: fsn1",
		"IPv4: 1.2.3.4",
		"IPv6: 2001:db8::",
	}

	for _, want := range expected {
		if !strings.Contains(summary, want) {
			t.Errorf("expected summary to include %q, got:\n%s", want, summary)
		}
	}
}

func TestBuildDeleteSummary_MinimalFields(t *testing.T) {
	s := domain.Server{
		ID:     "1",
		Name:   "bare",
		Status: "running",
	}

	summary := buildDeleteSummary(s)

	if !strings.Contains(summary, "ID: 1") {
		t.Errorf("expected 'ID: 1' in summary, got:\n%s", summary)
	}
	if !strings.Contains(summary, "Name: bare") {
		t.Errorf("expected 'Name: bare' in summary, got:\n%s", summary)
	}
	// Optional fields should not appear.
	if strings.Contains(summary, "Type:") {
		t.Errorf("expected no 'Type:' line in summary, got:\n%s", summary)
	}
	if strings.Contains(summary, "IPv4:") {
		t.Errorf("expected no 'IPv4:' line in summary, got:\n%s", summary)
	}
}
