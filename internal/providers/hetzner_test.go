package providers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"nathanbeddoewebdev/vpsm/internal/domain"
	"nathanbeddoewebdev/vpsm/internal/services/auth"
)

// newTestHetznerProvider creates a HetznerProvider pointed at the given base URL.
func newTestHetznerProvider(baseURL string, token string) *HetznerProvider {
	return &HetznerProvider{
		client:  &http.Client{Timeout: 5 * time.Second},
		baseURL: baseURL,
		token:   token,
	}
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

func TestListServers_HappyPath(t *testing.T) {
	created := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	response := hetznerListServersResponse{
		Servers: []hetznerServer{
			{
				ID:         42,
				Name:       "web-server",
				Status:     "running",
				Created:    created,
				ServerType: hetznerServerType{Name: "cpx11", Architecture: "x86"},
				Image:      &hetznerImage{Name: "ubuntu-24.04"},
				Location:   hetznerLocation{Name: "fsn1"},
				Datacenter: hetznerLocation{Name: "fsn1-dc14"},
				PublicNet: hetznerPublicNet{
					IPv4: &hetznerIPAddress{IP: "1.2.3.4"},
					IPv6: &hetznerIPAddress{IP: "2001:db8::/64"},
				},
				PrivateNet: []hetznerIPAddress{{IP: "10.0.0.2"}},
			},
			{
				ID:         99,
				Name:       "db-server",
				Status:     "stopped",
				Created:    created,
				ServerType: hetznerServerType{Name: "cpx22", Architecture: "arm"},
				Image:      &hetznerImage{Name: "debian-12"},
				Location:   hetznerLocation{Name: "nbg1"},
				Datacenter: hetznerLocation{Name: "nbg1-dc3"},
				PublicNet: hetznerPublicNet{
					IPv4: &hetznerIPAddress{IP: "5.6.7.8"},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/servers" {
			t.Errorf("expected path /servers, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Authorization header 'Bearer test-token', got %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got %q", r.Header.Get("Content-Type"))
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
		PublicIPv6:  "2001:db8::/64",
		PrivateIPv4: "10.0.0.2",
		Region:      "fsn1",
		ServerType:  "cpx11",
		Image:       "ubuntu-24.04",
		Provider:    "hetzner",
		Metadata: map[string]interface{}{
			"hetzner_id":   int64(42),
			"datacenter":   "fsn1-dc14",
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
			"datacenter":   "nbg1-dc3",
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
	srv := newTestAPI(t, hetznerListServersResponse{Servers: []hetznerServer{}})

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
	response := hetznerListServersResponse{
		Servers: []hetznerServer{
			{
				ID:         1,
				Name:       "bare-server",
				Status:     "running",
				ServerType: hetznerServerType{Name: "cx11", Architecture: "x86"},
				Image:      nil,
				Location:   hetznerLocation{Name: "hel1"},
				Datacenter: hetznerLocation{Name: "hel1-dc2"},
				// PublicNet and PrivateNet left as zero values
			},
		},
	}

	srv := newTestAPI(t, response)

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
		w.WriteHeader(http.StatusUnauthorized)
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer registry-token" {
			t.Errorf("expected Authorization 'Bearer registry-token', got %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(hetznerListServersResponse{
			Servers: []hetznerServer{
				{
					ID:         7,
					Name:       "registry-server",
					Status:     "running",
					ServerType: hetznerServerType{Name: "cpx31", Architecture: "x86"},
					Location:   hetznerLocation{Name: "ash"},
					Datacenter: hetznerLocation{Name: "ash-dc1"},
				},
			},
		})
	}))
	t.Cleanup(srv.Close)

	Reset()
	t.Cleanup(func() { Reset() })

	Register("test-hetzner", func(store auth.Store) (domain.Provider, error) {
		token, err := store.GetToken("test-hetzner")
		if err != nil {
			return nil, err
		}
		return &HetznerProvider{
			client:  &http.Client{Timeout: 5 * time.Second},
			baseURL: srv.URL,
			token:   token,
		}, nil
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
		return &HetznerProvider{
			client:  &http.Client{},
			baseURL: "http://unused",
			token:   token,
		}, nil
	})

	store := auth.NewMockStore() // no token set

	_, err := Get("test-hetzner", store)
	if err == nil {
		t.Fatal("expected error for missing token, got nil")
	}
}
