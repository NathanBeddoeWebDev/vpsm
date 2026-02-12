package providers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"

	"nathanbeddoewebdev/vpsm/internal/domain"
	"nathanbeddoewebdev/vpsm/internal/services/auth"
)

// --- Test helpers ---

// newTestHetznerProvider creates a HetznerProvider whose SDK client
// is pointed at the given httptest server URL.
func newTestHetznerProvider(serverURL string, token string) *HetznerProvider {
	return NewHetznerProvider(
		hcloud.WithEndpoint(serverURL),
		hcloud.WithToken(token),
	)
}

// newTestAPI spins up an httptest.Server that returns the given response as JSON.
// The server is automatically closed when the test finishes.
func newTestAPI(t *testing.T, response interface{}) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("failed to encode test response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// --- JSON builder helpers for Hetzner API-shaped responses ---

// testLocationJSON builds a Hetzner API location object.
func testLocationJSON(id int, name, country, city string) map[string]interface{} {
	return map[string]interface{}{
		"id": id, "name": name, "description": name,
		"country": country, "city": city,
		"latitude": 50.0, "longitude": 12.0,
		"network_zone": "eu-central",
	}
}

// testServerTypeJSON builds a Hetzner API server_type object.
func testServerTypeJSON(id int, name, arch string) map[string]interface{} {
	return map[string]interface{}{
		"id": id, "name": name, "description": name,
		"cores": 2, "memory": 2.0, "disk": 40,
		"architecture": arch,
		"storage_type": "local",
		"cpu_type":     "shared",
		"prices":       []interface{}{},
	}
}

// testImageJSON builds a Hetzner API image object.
func testImageJSON(id int, name, osFlavor, osVersion, arch string) map[string]interface{} {
	return map[string]interface{}{
		"id": id, "name": name, "description": name,
		"type": "system", "status": "available",
		"os_flavor": osFlavor, "os_version": osVersion,
		"architecture": arch,
	}
}

// testServerJSON builds a minimal Hetzner API server object with sensible defaults.
// The returned map can be modified before being used in a response.
func testServerJSON(id int, name, status, created string, loc map[string]interface{}, st map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"id":      id,
		"name":    name,
		"status":  status,
		"created": created,
		"public_net": map[string]interface{}{
			"floating_ips": []interface{}{},
			"firewalls":    []interface{}{},
		},
		"private_net":    []interface{}{},
		"server_type":    st,
		"image":          nil,
		"location":       loc,
		"labels":         map[string]interface{}{},
		"volumes":        []interface{}{},
		"load_balancers": []interface{}{},
	}
}

// --- ListServers tests ---

func TestListServers_HappyPath(t *testing.T) {
	const createdStr = "2024-06-15T12:00:00+00:00"
	// Parse the expected time identically to how the SDK will parse it from JSON,
	// avoiding any timezone representation mismatches with time.Date().
	created, _ := time.Parse(time.RFC3339, createdStr)

	fsn1 := testLocationJSON(1, "fsn1", "DE", "Falkenstein")
	nbg1 := testLocationJSON(2, "nbg1", "DE", "Nuremberg")

	server1 := testServerJSON(42, "web-server", "running", createdStr, fsn1, testServerTypeJSON(1, "cpx11", "x86"))
	server1["public_net"] = map[string]interface{}{
		"ipv4":         map[string]interface{}{"ip": "1.2.3.4", "blocked": false},
		"ipv6":         map[string]interface{}{"ip": "2001:db8::/64", "blocked": false},
		"floating_ips": []interface{}{},
		"firewalls":    []interface{}{},
	}
	server1["private_net"] = []interface{}{
		map[string]interface{}{"ip": "10.0.0.2", "alias_ips": []interface{}{}, "network": 1, "mac_address": ""},
	}
	server1["image"] = testImageJSON(1, "ubuntu-24.04", "ubuntu", "24.04", "x86")

	server2 := testServerJSON(99, "db-server", "stopped", createdStr, nbg1, testServerTypeJSON(2, "cpx22", "arm"))
	server2["public_net"] = map[string]interface{}{
		"ipv4":         map[string]interface{}{"ip": "5.6.7.8", "blocked": false},
		"floating_ips": []interface{}{},
		"firewalls":    []interface{}{},
	}
	server2["image"] = testImageJSON(2, "debian-12", "debian", "12", "x86")

	response := map[string]interface{}{
		"servers": []interface{}{server1, server2},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/servers" {
			t.Errorf("expected path /servers, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Authorization header 'Bearer test-token', got %q", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(srv.Close)

	provider := newTestHetznerProvider(srv.URL, "test-token")
	servers, err := provider.ListServers()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}

	wantFirst := domain.Server{
		ID:          "42",
		Name:        "web-server",
		Status:      "running",
		CreatedAt:   created,
		PublicIPv4:  "1.2.3.4",
		PublicIPv6:  "2001:db8::",
		PrivateIPv4: "10.0.0.2",
		Region:      "fsn1",
		ServerType:  "cpx11",
		Image:       "ubuntu-24.04",
		Provider:    "hetzner",
		Metadata: map[string]interface{}{
			"hetzner_id":   int64(42),
			"architecture": "x86",
		},
	}

	wantSecond := domain.Server{
		ID:         "99",
		Name:       "db-server",
		Status:     "stopped",
		CreatedAt:  created,
		PublicIPv4: "5.6.7.8",
		Region:     "nbg1",
		ServerType: "cpx22",
		Image:      "debian-12",
		Provider:   "hetzner",
		Metadata: map[string]interface{}{
			"hetzner_id":   int64(99),
			"architecture": "arm",
		},
	}

	if diff := cmp.Diff(wantFirst, servers[0]); diff != "" {
		t.Errorf("server[0] mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(wantSecond, servers[1]); diff != "" {
		t.Errorf("server[1] mismatch (-want +got):\n%s", diff)
	}
}

func TestListServers_EmptyList(t *testing.T) {
	srv := newTestAPI(t, map[string]interface{}{
		"servers": []interface{}{},
	})

	provider := newTestHetznerProvider(srv.URL, "test-token")
	servers, err := provider.ListServers()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(servers))
	}
}

func TestListServers_NilOptionalFields(t *testing.T) {
	loc := testLocationJSON(3, "hel1", "FI", "Helsinki")

	server := testServerJSON(1, "bare-server", "running", "2024-06-15T12:00:00+00:00", loc, testServerTypeJSON(1, "cx11", "x86"))
	// image is already nil from testServerJSON
	// public_net has no ipv4/ipv6 entries, private_net is empty

	srv := newTestAPI(t, map[string]interface{}{
		"servers": []interface{}{server},
	})

	provider := newTestHetznerProvider(srv.URL, "test-token")
	servers, err := provider.ListServers()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}

	s := servers[0]
	if s.PublicIPv4 != "" {
		t.Errorf("PublicIPv4 = %q, want empty", s.PublicIPv4)
	}
	if s.PublicIPv6 != "" {
		t.Errorf("PublicIPv6 = %q, want empty", s.PublicIPv6)
	}
	if s.PrivateIPv4 != "" {
		t.Errorf("PrivateIPv4 = %q, want empty", s.PrivateIPv4)
	}
	if s.Image != "" {
		t.Errorf("Image = %q, want empty", s.Image)
	}
}

func TestListServers_Non200StatusCode(t *testing.T) {
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
	_, err := provider.ListServers()
	if err == nil {
		t.Fatal("expected error for 401 response, got nil")
	}
}

func TestListServers_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{invalid json`))
	}))
	t.Cleanup(srv.Close)

	provider := newTestHetznerProvider(srv.URL, "test-token")
	_, err := provider.ListServers()
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestListServers_FactoryViaRegistry(t *testing.T) {
	loc := testLocationJSON(4, "ash", "US", "Ashburn")

	server := testServerJSON(7, "registry-server", "running", "2024-06-15T12:00:00+00:00", loc, testServerTypeJSON(3, "cpx31", "x86"))

	response := map[string]interface{}{
		"servers": []interface{}{server},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer registry-token" {
			t.Errorf("expected Authorization 'Bearer registry-token', got %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(srv.Close)

	Reset()
	t.Cleanup(func() { Reset() })

	Register("test-hetzner", func(store auth.Store) (domain.Provider, error) {
		token, err := store.GetToken("test-hetzner")
		if err != nil {
			return nil, err
		}
		return NewHetznerProvider(
			hcloud.WithToken(token),
			hcloud.WithEndpoint(srv.URL),
		), nil
	})

	store := auth.NewMockStore()
	store.SetToken("test-hetzner", "registry-token")

	provider, err := Get("test-hetzner", store)
	if err != nil {
		t.Fatalf("expected no error from Get, got %v", err)
	}

	servers, err := provider.ListServers()
	if err != nil {
		t.Fatalf("expected no error from ListServers, got %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0].Name != "registry-server" {
		t.Errorf("server.Name = %q, want %q", servers[0].Name, "registry-server")
	}
	if servers[0].Provider != "hetzner" {
		t.Errorf("server.Provider = %q, want %q", servers[0].Provider, "hetzner")
	}
}

func TestListServers_FactoryMissingToken(t *testing.T) {
	Reset()
	t.Cleanup(func() { Reset() })

	Register("test-hetzner", func(store auth.Store) (domain.Provider, error) {
		token, err := store.GetToken("test-hetzner")
		if err != nil {
			return nil, err
		}
		return NewHetznerProvider(hcloud.WithToken(token)), nil
	})

	store := auth.NewMockStore() // no token set

	_, err := Get("test-hetzner", store)
	if err == nil {
		t.Fatal("expected error for missing token, got nil")
	}
}
