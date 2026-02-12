package providers

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"nathanbeddoewebdev/vpsm/internal/domain"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

// --- CatalogProvider implementation ---

// ListLocations retrieves all available locations from the Hetzner Cloud API.
func (h *HetznerProvider) ListLocations() ([]domain.Location, error) {
	ctx := context.Background()

	hzLocations, err := h.client.Location.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list locations: %w", err)
	}

	locations := make([]domain.Location, 0, len(hzLocations))
	for _, loc := range hzLocations {
		locations = append(locations, toDomainLocation(loc))
	}

	return locations, nil
}

// ListServerTypes retrieves all available server types from the Hetzner Cloud API.
func (h *HetznerProvider) ListServerTypes() ([]domain.ServerTypeSpec, error) {
	ctx := context.Background()

	hzServerTypes, err := h.client.ServerType.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list server types: %w", err)
	}

	serverTypes := make([]domain.ServerTypeSpec, 0, len(hzServerTypes))
	for _, st := range hzServerTypes {
		serverTypes = append(serverTypes, toDomainServerType(st))
	}

	return serverTypes, nil
}

// ListImages retrieves all available images from the Hetzner Cloud API.
func (h *HetznerProvider) ListImages() ([]domain.ImageSpec, error) {
	ctx := context.Background()

	hzImages, err := h.client.Image.AllWithOpts(ctx, hcloud.ImageListOpts{
		Status: []hcloud.ImageStatus{hcloud.ImageStatusAvailable},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}

	images := make([]domain.ImageSpec, 0, len(hzImages))
	for _, img := range hzImages {
		images = append(images, toDomainImage(img))
	}

	return images, nil
}

// ListSSHKeys retrieves all SSH keys from the Hetzner Cloud API.
func (h *HetznerProvider) ListSSHKeys() ([]domain.SSHKeySpec, error) {
	ctx := context.Background()

	hzKeys, err := h.client.SSHKey.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list SSH keys: %w", err)
	}

	keys := make([]domain.SSHKeySpec, 0, len(hzKeys))
	for _, k := range hzKeys {
		keys = append(keys, toDomainSSHKey(k))
	}

	return keys, nil
}

// --- Domain mapping helpers ---

func toDomainLocation(loc *hcloud.Location) domain.Location {
	return domain.Location{
		ID:          strconv.FormatInt(loc.ID, 10),
		Name:        loc.Name,
		Description: loc.Description,
		Country:     loc.Country,
		City:        loc.City,
	}
}

func toDomainServerType(st *hcloud.ServerType) domain.ServerTypeSpec {
	spec := domain.ServerTypeSpec{
		ID:           strconv.FormatInt(st.ID, 10),
		Name:         st.Name,
		Description:  st.Description,
		Cores:        st.Cores,
		Memory:       float64(st.Memory),
		Disk:         st.Disk,
		Architecture: string(st.Architecture),
	}

	// Extract available locations, excluding any that are deprecated and
	// past their UnavailableAfter date. The Locations field carries
	// per-location deprecation info and is the preferred source (the
	// Pricings-based approach does not account for deprecation).
	now := time.Now()
	spec.Locations = availableLocations(st.Locations, now)

	// Fall back to the prices array if Locations was empty (older API
	// responses may omit it).
	if len(spec.Locations) == 0 {
		locations := make([]string, 0, len(st.Pricings))
		for _, pricing := range st.Pricings {
			if pricing.Location != nil && pricing.Location.Name != "" {
				locations = append(locations, pricing.Location.Name)
			}
		}
		if len(locations) > 0 {
			spec.Locations = uniqueStrings(locations)
		}
	}

	// Use the first available price entry as the representative price.
	if len(st.Pricings) > 0 {
		spec.PriceMonthly = st.Pricings[0].Monthly.Gross
		spec.PriceHourly = st.Pricings[0].Hourly.Gross
	}

	return spec
}

// availableLocations returns location names from the server type's Locations
// field, excluding any that are deprecated with an UnavailableAfter date in
// the past. A location without deprecation info, or with a future
// UnavailableAfter date, is considered available.
func availableLocations(stLocations []hcloud.ServerTypeLocation, now time.Time) []string {
	names := make([]string, 0, len(stLocations))
	for _, stl := range stLocations {
		if stl.Location == nil || stl.Location.Name == "" {
			continue
		}
		if stl.IsDeprecated() && now.After(stl.UnavailableAfter()) {
			continue
		}
		names = append(names, stl.Location.Name)
	}
	if len(names) == 0 {
		return nil
	}
	return uniqueStrings(names)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func toDomainImage(img *hcloud.Image) domain.ImageSpec {
	return domain.ImageSpec{
		ID:           strconv.FormatInt(img.ID, 10),
		Name:         img.Name,
		Description:  img.Description,
		Type:         string(img.Type),
		OSFlavor:     img.OSFlavor,
		Architecture: string(img.Architecture),
	}
}

func toDomainSSHKey(k *hcloud.SSHKey) domain.SSHKeySpec {
	return domain.SSHKeySpec{
		ID:          strconv.FormatInt(k.ID, 10),
		Name:        k.Name,
		Fingerprint: k.Fingerprint,
	}
}
