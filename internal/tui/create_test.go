package tui

import (
	"strings"
	"testing"

	"nathanbeddoewebdev/vpsm/internal/domain"

	"github.com/charmbracelet/huh"
	"github.com/google/go-cmp/cmp"
)

type optionPair struct {
	Key   string
	Value string
}

func TestBuildLocationOptions_AddsCustom(t *testing.T) {
	locations := []domain.Location{
		{
			ID:      "1",
			Name:    "fsn1",
			City:    "Falkenstein",
			Country: "DE",
		},
	}

	options, labels := buildLocationOptions(locations, "custom-loc")

	expected := []optionPair{
		{Key: "fsn1 - Falkenstein, DE", Value: "fsn1"},
		{Key: "Custom: custom-loc", Value: "custom-loc"},
	}

	if diff := cmp.Diff(expected, optionsToPairs(options)); diff != "" {
		t.Errorf("unexpected location options (-want +got):\n%s", diff)
	}
	if _, ok := labels[""]; ok {
		t.Errorf("expected no auto label in map, but found one")
	}
}

func TestBuildServerTypeOptions_UsesNameAndPrice(t *testing.T) {
	serverTypes := []domain.ServerTypeSpec{
		{
			ID:           "1",
			Name:         "cpx11",
			Cores:        2,
			Memory:       2,
			Disk:         40,
			PriceMonthly: "4.50",
		},
	}

	_, labels := buildServerTypeOptions(serverTypes, "")

	expected := "cpx11 - 2 vCPU / 2 GB / 40 GB - 4.50/mo"
	if diff := cmp.Diff(expected, labels["cpx11"]); diff != "" {
		t.Errorf("unexpected server type label (-want +got):\n%s", diff)
	}
}

func TestFilterServerTypesByLocation(t *testing.T) {
	serverTypes := []domain.ServerTypeSpec{
		{Name: "cpx11", Locations: []string{"fsn1", "nbg1"}},
		{Name: "cax11", Locations: []string{"hel1"}},
		{Name: "cx21", Locations: []string{"fsn1"}},
	}

	filtered := filterServerTypesByLocation(serverTypes, "fsn1")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 server types after filtering, got %d", len(filtered))
	}
	if filtered[0].Name != "cpx11" {
		t.Errorf("expected first server type to be cpx11, got %q", filtered[0].Name)
	}
	if filtered[1].Name != "cx21" {
		t.Errorf("expected second server type to be cx21, got %q", filtered[1].Name)
	}
}

func TestFilterServerTypesByLocation_ExcludesUnavailable(t *testing.T) {
	serverTypes := []domain.ServerTypeSpec{
		{Name: "cpx11", Locations: []string{"fsn1"}},
		{Name: "cax11", Locations: []string{"hel1"}},
		{Name: "cx21", Locations: nil},
	}

	filtered := filterServerTypesByLocation(serverTypes, "sgp1")
	if len(filtered) != 0 {
		t.Errorf("expected 0 server types for sgp1, got %d", len(filtered))
	}
}

func TestFilterServerTypesByLocation_EmptyLocationReturnsAll(t *testing.T) {
	serverTypes := []domain.ServerTypeSpec{
		{Name: "cpx11", Locations: []string{"fsn1"}},
		{Name: "cax11", Locations: []string{"hel1"}},
	}

	filtered := filterServerTypesByLocation(serverTypes, "")
	if len(filtered) != len(serverTypes) {
		t.Errorf("expected all %d server types for empty location, got %d", len(serverTypes), len(filtered))
	}
}

func TestBuildImageOptions_FilterByArchitecture(t *testing.T) {
	images := []domain.ImageSpec{
		{ID: "1", Name: "ubuntu-24.04", Type: "system", Architecture: "x86"},
		{ID: "2", Name: "ubuntu-24.04-arm", Type: "system", Architecture: "arm"},
		{ID: "3", Name: "snapshot-1", Type: "snapshot", Architecture: "arm"},
	}

	options, _ := buildImageOptions(images, "arm", "")
	pairs := optionsToPairs(options)
	if len(pairs) != 1 {
		t.Fatalf("expected 1 image option, got %d", len(pairs))
	}
	if pairs[0].Key != "ubuntu-24.04-arm (arm)" {
		t.Errorf("unexpected image label: %q", pairs[0].Key)
	}

	options, _ = buildImageOptions(images, "riscv", "")
	if len(options) != 2 {
		t.Fatalf("expected fallback to 2 system images, got %d", len(options))
	}
}

func TestBuildSSHKeyOptions_AddsCustom(t *testing.T) {
	keys := []domain.SSHKeySpec{
		{ID: "1", Name: "default", Fingerprint: "aa:bb"},
	}

	options, labels := buildSSHKeyOptions(keys, []string{"default", "missing"})
	pairs := optionsToPairs(options)

	if len(pairs) != 2 {
		t.Fatalf("expected 2 SSH key options, got %d", len(pairs))
	}
	if pairs[0].Key != "default (aa:bb)" {
		t.Errorf("unexpected first SSH key label: %q", pairs[0].Key)
	}
	if labels["missing"] != "Custom: missing" {
		t.Errorf("expected custom SSH key label, got %q", labels["missing"])
	}
}

func TestBuildSummary_IncludesOptionalFields(t *testing.T) {
	start := true
	opts := domain.CreateServerOpts{
		Name:             "web-1",
		Location:         "fsn1",
		ServerType:       "cpx11",
		Image:            "ubuntu-24.04",
		SSHKeys:          []string{"key-1"},
		Labels:           map[string]string{"env": "prod", "role": "web"},
		UserData:         "#!/bin/bash\necho hello",
		StartAfterCreate: &start,
	}

	summary := buildSummary(
		opts,
		map[string]string{"fsn1": "fsn1 - Falkenstein, DE"},
		map[string]string{"cpx11": "cpx11 - 2 vCPU / 2 GB / 40 GB"},
		map[string]string{"ubuntu-24.04": "ubuntu-24.04 (x86)"},
		map[string]string{"key-1": "key-1 (aa:bb)"},
	)

	expected := []string{
		"Name: web-1",
		"Location: fsn1 - Falkenstein, DE",
		"Server type: cpx11 - 2 vCPU / 2 GB / 40 GB",
		"Image: ubuntu-24.04 (x86)",
		"SSH keys: key-1 (aa:bb)",
		"Labels: env=prod, role=web",
		"User data: 22 bytes",
		"Start after create: true",
	}

	for _, want := range expected {
		if !strings.Contains(summary, want) {
			t.Errorf("expected summary to include %q, got:\n%s", want, summary)
		}
	}
}

func TestHasServerType(t *testing.T) {
	types := []domain.ServerTypeSpec{
		{ID: "1", Name: "cpx11"},
		{ID: "2", Name: "cax11"},
	}

	if !hasServerType(types, "cpx11") {
		t.Error("expected hasServerType to find cpx11 by name")
	}
	if !hasServerType(types, "2") {
		t.Error("expected hasServerType to find cax11 by ID")
	}
	if hasServerType(types, "nonexistent") {
		t.Error("expected hasServerType to return false for nonexistent type")
	}
}

func TestSelectHeight(t *testing.T) {
	if got := selectHeight(3, 10); got != 3 {
		t.Errorf("expected selectHeight(3, 10) = 3, got %d", got)
	}
	if got := selectHeight(15, 10); got != 10 {
		t.Errorf("expected selectHeight(15, 10) = 10, got %d", got)
	}
	if got := selectHeight(10, 10); got != 10 {
		t.Errorf("expected selectHeight(10, 10) = 10, got %d", got)
	}
}

func TestFilterImages(t *testing.T) {
	images := []domain.ImageSpec{
		{ID: "1", Name: "ubuntu-24.04", Type: "system", Architecture: "x86"},
		{ID: "2", Name: "ubuntu-24.04-arm", Type: "system", Architecture: "arm"},
		{ID: "3", Name: "snapshot-1", Type: "snapshot", Architecture: "x86"},
	}

	// Filters to system images matching architecture.
	filtered := filterImages(images, "x86")
	if len(filtered) != 1 || filtered[0].Name != "ubuntu-24.04" {
		t.Errorf("expected 1 x86 system image, got %d", len(filtered))
	}

	// Falls back to all system images when arch has no match.
	filtered = filterImages(images, "riscv")
	if len(filtered) != 2 {
		t.Errorf("expected 2 system images as fallback, got %d", len(filtered))
	}

	// Returns nil for empty input.
	filtered = filterImages(nil, "x86")
	if filtered != nil {
		t.Errorf("expected nil for empty images, got %v", filtered)
	}
}

func TestFormatLabels(t *testing.T) {
	labels := map[string]string{"env": "prod", "role": "web"}
	result := formatLabels(labels)
	if result != "env=prod, role=web" {
		t.Errorf("unexpected label format: %q", result)
	}

	if formatLabels(nil) != "" {
		t.Error("expected empty string for nil labels")
	}
}

func optionsToPairs(options []huh.Option[string]) []optionPair {
	pairs := make([]optionPair, 0, len(options))
	for _, option := range options {
		pairs = append(pairs, optionPair{Key: option.Key, Value: option.Value})
	}
	return pairs
}
