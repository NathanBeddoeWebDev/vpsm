package providers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"

	"nathanbeddoewebdev/vpsm/internal/domain"
)

// --- ListLocations tests ---

func TestListLocations_HappyPath(t *testing.T) {
	response := map[string]interface{}{
		"locations": []interface{}{
			testLocationJSON(1, "fsn1", "DE", "Falkenstein"),
			testLocationJSON(2, "nbg1", "DE", "Nuremberg"),
			testLocationJSON(3, "ash", "US", "Ashburn"),
		},
	}

	srv := newTestAPI(t, response)
	provider := newTestHetznerProvider(srv.URL, "test-token")

	locations, err := provider.ListLocations()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(locations) != 3 {
		t.Fatalf("expected 3 locations, got %d", len(locations))
	}

	want := []domain.Location{
		{ID: "1", Name: "fsn1", Description: "fsn1", Country: "DE", City: "Falkenstein"},
		{ID: "2", Name: "nbg1", Description: "nbg1", Country: "DE", City: "Nuremberg"},
		{ID: "3", Name: "ash", Description: "ash", Country: "US", City: "Ashburn"},
	}

	if diff := cmp.Diff(want, locations); diff != "" {
		t.Errorf("locations mismatch (-want +got):\n%s", diff)
	}
}

func TestListLocations_EmptyList(t *testing.T) {
	srv := newTestAPI(t, map[string]interface{}{
		"locations": []interface{}{},
	})
	provider := newTestHetznerProvider(srv.URL, "test-token")

	locations, err := provider.ListLocations()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(locations) != 0 {
		t.Errorf("expected 0 locations, got %d", len(locations))
	}
}

func TestListLocations_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "unauthorized",
				"message": "unable to authenticate",
			},
		})
	}))
	t.Cleanup(srv.Close)

	provider := newTestHetznerProvider(srv.URL, "bad-token")
	_, err := provider.ListLocations()
	if err == nil {
		t.Fatal("expected error for 401 response, got nil")
	}
}

// --- ListServerTypes tests ---

func TestListServerTypes_HappyPath(t *testing.T) {
	st1 := testServerTypeJSON(1, "cpx11", "x86")
	st1["cores"] = 2
	st1["memory"] = 2.0
	st1["disk"] = 40
	st1["locations"] = []interface{}{
		testServerTypeLocationJSON(1, "fsn1", nil),
		testServerTypeLocationJSON(2, "nbg1", nil),
	}
	st1["prices"] = []interface{}{
		map[string]interface{}{
			"location":      "fsn1",
			"price_hourly":  map[string]interface{}{"net": "0.0054", "gross": "0.0064"},
			"price_monthly": map[string]interface{}{"net": "3.29", "gross": "3.92"},
		},
		map[string]interface{}{
			"location":      "nbg1",
			"price_hourly":  map[string]interface{}{"net": "0.0054", "gross": "0.0064"},
			"price_monthly": map[string]interface{}{"net": "3.29", "gross": "3.92"},
		},
	}

	st2 := testServerTypeJSON(2, "cax11", "arm")
	st2["cores"] = 2
	st2["memory"] = 4.0
	st2["disk"] = 40
	st2["locations"] = []interface{}{
		testServerTypeLocationJSON(1, "fsn1", nil),
	}
	st2["prices"] = []interface{}{
		map[string]interface{}{
			"location":      "fsn1",
			"price_hourly":  map[string]interface{}{"net": "0.0046", "gross": "0.0055"},
			"price_monthly": map[string]interface{}{"net": "2.69", "gross": "3.29"},
		},
	}

	response := map[string]interface{}{
		"server_types": []interface{}{st1, st2},
	}

	srv := newTestAPI(t, response)
	provider := newTestHetznerProvider(srv.URL, "test-token")

	serverTypes, err := provider.ListServerTypes()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(serverTypes) != 2 {
		t.Fatalf("expected 2 server types, got %d", len(serverTypes))
	}

	want := []domain.ServerTypeSpec{
		{
			ID: "1", Name: "cpx11", Description: "cpx11",
			Cores: 2, Memory: 2.0, Disk: 40, Architecture: "x86",
			PriceMonthly: "3.92", PriceHourly: "0.0064",
			Locations: []string{"fsn1", "nbg1"},
		},
		{
			ID: "2", Name: "cax11", Description: "cax11",
			Cores: 2, Memory: 4.0, Disk: 40, Architecture: "arm",
			PriceMonthly: "3.29", PriceHourly: "0.0055",
			Locations: []string{"fsn1"},
		},
	}

	if diff := cmp.Diff(want, serverTypes); diff != "" {
		t.Errorf("server types mismatch (-want +got):\n%s", diff)
	}
}

func TestListServerTypes_ExcludesDeprecatedLocations(t *testing.T) {
	st := testServerTypeJSON(1, "cpx11", "x86")
	st["locations"] = []interface{}{
		testServerTypeLocationJSON(1, "fsn1", map[string]interface{}{
			"announced":         "2024-01-01T00:00:00+00:00",
			"unavailable_after": "2024-06-01T00:00:00+00:00",
		}),
		testServerTypeLocationJSON(2, "nbg1", nil),
	}
	st["prices"] = []interface{}{
		map[string]interface{}{
			"location":      "fsn1",
			"price_hourly":  map[string]interface{}{"net": "0.0054", "gross": "0.0064"},
			"price_monthly": map[string]interface{}{"net": "3.29", "gross": "3.92"},
		},
		map[string]interface{}{
			"location":      "nbg1",
			"price_hourly":  map[string]interface{}{"net": "0.0054", "gross": "0.0064"},
			"price_monthly": map[string]interface{}{"net": "3.29", "gross": "3.92"},
		},
	}

	response := map[string]interface{}{
		"server_types": []interface{}{st},
	}

	srv := newTestAPI(t, response)
	provider := newTestHetznerProvider(srv.URL, "test-token")

	serverTypes, err := provider.ListServerTypes()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(serverTypes) != 1 {
		t.Fatalf("expected 1 server type, got %d", len(serverTypes))
	}

	// fsn1 is deprecated (unavailable_after is in the past), only nbg1 should remain.
	want := []string{"nbg1"}
	if diff := cmp.Diff(want, serverTypes[0].Locations); diff != "" {
		t.Errorf("locations mismatch (-want +got):\n%s", diff)
	}
}

func TestListServerTypes_FallsBackToPricesWhenNoLocations(t *testing.T) {
	st := testServerTypeJSON(1, "cpx11", "x86")
	st["locations"] = []interface{}{} // empty locations array
	st["prices"] = []interface{}{
		map[string]interface{}{
			"location":      "fsn1",
			"price_hourly":  map[string]interface{}{"net": "0.0054", "gross": "0.0064"},
			"price_monthly": map[string]interface{}{"net": "3.29", "gross": "3.92"},
		},
	}

	response := map[string]interface{}{
		"server_types": []interface{}{st},
	}

	srv := newTestAPI(t, response)
	provider := newTestHetznerProvider(srv.URL, "test-token")

	serverTypes, err := provider.ListServerTypes()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(serverTypes) != 1 {
		t.Fatalf("expected 1 server type, got %d", len(serverTypes))
	}

	want := []string{"fsn1"}
	if diff := cmp.Diff(want, serverTypes[0].Locations); diff != "" {
		t.Errorf("locations mismatch (-want +got):\n%s", diff)
	}
}

func TestListServerTypes_NoPrices(t *testing.T) {
	st := testServerTypeJSON(1, "cpx11", "x86")
	// prices is already an empty array from the helper

	response := map[string]interface{}{
		"server_types": []interface{}{st},
	}

	srv := newTestAPI(t, response)
	provider := newTestHetznerProvider(srv.URL, "test-token")

	serverTypes, err := provider.ListServerTypes()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(serverTypes) != 1 {
		t.Fatalf("expected 1 server type, got %d", len(serverTypes))
	}

	if serverTypes[0].PriceMonthly != "" {
		t.Errorf("PriceMonthly = %q, want empty", serverTypes[0].PriceMonthly)
	}
	if serverTypes[0].PriceHourly != "" {
		t.Errorf("PriceHourly = %q, want empty", serverTypes[0].PriceHourly)
	}
}

func TestListServerTypes_EmptyList(t *testing.T) {
	srv := newTestAPI(t, map[string]interface{}{
		"server_types": []interface{}{},
	})
	provider := newTestHetznerProvider(srv.URL, "test-token")

	serverTypes, err := provider.ListServerTypes()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(serverTypes) != 0 {
		t.Errorf("expected 0 server types, got %d", len(serverTypes))
	}
}

func TestListServerTypes_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "server_error",
				"message": "internal error",
			},
		})
	}))
	t.Cleanup(srv.Close)

	provider := newTestHetznerProvider(srv.URL, "test-token")
	_, err := provider.ListServerTypes()
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

// --- ListImages tests ---

func TestListImages_HappyPath(t *testing.T) {
	response := map[string]interface{}{
		"images": []interface{}{
			testImageJSON(114690387, "ubuntu-24.04", "ubuntu", "24.04", "x86"),
			testImageJSON(114690389, "debian-12", "debian", "12", "x86"),
		},
	}

	srv := newTestAPI(t, response)
	provider := newTestHetznerProvider(srv.URL, "test-token")

	images, err := provider.ListImages()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(images) != 2 {
		t.Fatalf("expected 2 images, got %d", len(images))
	}

	want := []domain.ImageSpec{
		{ID: "114690387", Name: "ubuntu-24.04", Description: "ubuntu-24.04", Type: "system", OSFlavor: "ubuntu", Architecture: "x86"},
		{ID: "114690389", Name: "debian-12", Description: "debian-12", Type: "system", OSFlavor: "debian", Architecture: "x86"},
	}

	if diff := cmp.Diff(want, images); diff != "" {
		t.Errorf("images mismatch (-want +got):\n%s", diff)
	}
}

func TestListImages_EmptyList(t *testing.T) {
	srv := newTestAPI(t, map[string]interface{}{
		"images": []interface{}{},
	})
	provider := newTestHetznerProvider(srv.URL, "test-token")

	images, err := provider.ListImages()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(images) != 0 {
		t.Errorf("expected 0 images, got %d", len(images))
	}
}

func TestListImages_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "forbidden",
				"message": "insufficient permissions",
			},
		})
	}))
	t.Cleanup(srv.Close)

	provider := newTestHetznerProvider(srv.URL, "test-token")
	_, err := provider.ListImages()
	if err == nil {
		t.Fatal("expected error for 403 response, got nil")
	}
}

// --- ListSSHKeys tests ---

func testSSHKeyJSON(id int, name, fingerprint, publicKey string) map[string]interface{} {
	return map[string]interface{}{
		"id":          id,
		"name":        name,
		"fingerprint": fingerprint,
		"public_key":  publicKey,
		"labels":      map[string]interface{}{},
		"created":     "2024-01-01T00:00:00+00:00",
	}
}

func TestListSSHKeys_HappyPath(t *testing.T) {
	response := map[string]interface{}{
		"ssh_keys": []interface{}{
			testSSHKeyJSON(1, "my-key", "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:2f", "ssh-rsa AAAA..."),
			testSSHKeyJSON(2, "deploy-key", "a1:b2:c3:d4:e5:f6:00:11:22:33:44:55:66:77:88:99", "ssh-ed25519 AAAA..."),
		},
	}

	srv := newTestAPI(t, response)
	provider := newTestHetznerProvider(srv.URL, "test-token")

	keys, err := provider.ListSSHKeys()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(keys) != 2 {
		t.Fatalf("expected 2 SSH keys, got %d", len(keys))
	}

	want := []domain.SSHKeySpec{
		{ID: "1", Name: "my-key", Fingerprint: "b7:2f:30:a0:2f:6c:58:6c:21:04:58:61:ba:06:3b:2f"},
		{ID: "2", Name: "deploy-key", Fingerprint: "a1:b2:c3:d4:e5:f6:00:11:22:33:44:55:66:77:88:99"},
	}

	if diff := cmp.Diff(want, keys); diff != "" {
		t.Errorf("SSH keys mismatch (-want +got):\n%s", diff)
	}
}

func TestListSSHKeys_EmptyList(t *testing.T) {
	srv := newTestAPI(t, map[string]interface{}{
		"ssh_keys": []interface{}{},
	})
	provider := newTestHetznerProvider(srv.URL, "test-token")

	keys, err := provider.ListSSHKeys()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 SSH keys, got %d", len(keys))
	}
}

func TestListSSHKeys_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "unauthorized",
				"message": "unable to authenticate",
			},
		})
	}))
	t.Cleanup(srv.Close)

	provider := newTestHetznerProvider(srv.URL, "bad-token")
	_, err := provider.ListSSHKeys()
	if err == nil {
		t.Fatal("expected error for 401 response, got nil")
	}
}

// --- Verify correct endpoints ---

func TestCatalogMethods_RequestPaths(t *testing.T) {
	tests := []struct {
		name     string
		call     func(p *HetznerProvider) error
		wantPath string
	}{
		{
			name:     "ListLocations",
			call:     func(p *HetznerProvider) error { _, err := p.ListLocations(); return err },
			wantPath: "/locations",
		},
		{
			name:     "ListServerTypes",
			call:     func(p *HetznerProvider) error { _, err := p.ListServerTypes(); return err },
			wantPath: "/server_types",
		},
		{
			name:     "ListImages",
			call:     func(p *HetznerProvider) error { _, err := p.ListImages(); return err },
			wantPath: "/images",
		},
		{
			name:     "ListSSHKeys",
			call:     func(p *HetznerProvider) error { _, err := p.ListSSHKeys(); return err },
			wantPath: "/ssh_keys",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.wantPath {
					t.Errorf("expected path %s, got %s", tc.wantPath, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer catalog-token" {
					t.Errorf("expected Authorization 'Bearer catalog-token', got %q", r.Header.Get("Authorization"))
				}

				// Return a valid empty response for each endpoint.
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"locations":    []interface{}{},
					"server_types": []interface{}{},
					"images":       []interface{}{},
					"ssh_keys":     []interface{}{},
				})
			}))
			t.Cleanup(srv.Close)

			provider := newTestHetznerProvider(srv.URL, "catalog-token")
			if err := tc.call(provider); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
