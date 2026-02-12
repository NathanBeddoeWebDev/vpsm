package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nathanbeddoewebdev/vpsm/internal/domain"
)

// testActionJSON builds a minimal Hetzner API action object.
func testActionJSON(id int) map[string]interface{} {
	return map[string]interface{}{
		"id":       id,
		"status":   "running",
		"command":  "create_server",
		"progress": 0,
		"started":  "2024-06-15T12:00:00+00:00",
		"finished": nil,
		"resources": []interface{}{
			map[string]interface{}{"id": 1, "type": "server"},
		},
		"error": map[string]interface{}{
			"code":    "",
			"message": "",
		},
	}
}

// newCreateTestServer spins up a flexible httptest.Server that routes
// based on method + path. The handlers map is keyed by "METHOD /path".
func newCreateTestServer(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		if h, ok := handlers[key]; ok {
			h(w, r)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "not_found",
				"message": "not found",
			},
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCreateServer_HappyPath(t *testing.T) {
	loc := testLocationJSON(1, "fsn1", "DE", "Falkenstein")
	st := testServerTypeJSON(1, "cpx11", "x86")
	img := testImageJSON(1, "ubuntu-24.04", "ubuntu", "24.04", "x86")

	createdServer := testServerJSON(42, "my-server", "initializing", "2024-06-15T12:00:00+00:00", loc, st)
	createdServer["image"] = img
	createdServer["public_net"] = map[string]interface{}{
		"ipv4":         map[string]interface{}{"ip": "1.2.3.4", "blocked": false},
		"ipv6":         map[string]interface{}{"ip": "2001:db8::/64", "blocked": false},
		"floating_ips": []interface{}{},
		"firewalls":    []interface{}{},
	}

	srv := newCreateTestServer(t, map[string]http.HandlerFunc{
		"POST /servers": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			bodyStr := string(body)

			if !strings.Contains(bodyStr, `"name":"my-server"`) {
				t.Errorf("expected name 'my-server' in body, got: %s", bodyStr)
			}
			if !strings.Contains(bodyStr, `"image":"ubuntu-24.04"`) {
				t.Errorf("expected image 'ubuntu-24.04' in body, got: %s", bodyStr)
			}
			if !strings.Contains(bodyStr, `"server_type":"cpx11"`) {
				t.Errorf("expected server_type 'cpx11' in body, got: %s", bodyStr)
			}
			if !strings.Contains(bodyStr, `"location":"fsn1"`) {
				t.Errorf("expected location 'fsn1' in body, got: %s", bodyStr)
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"server":       createdServer,
				"action":       testActionJSON(1),
				"next_actions": []interface{}{},
			})
		},
	})

	ctx := context.Background()
	provider := newTestHetznerProvider(t, srv.URL, "test-token")

	server, err := provider.CreateServer(ctx, domain.CreateServerOpts{
		Name:       "my-server",
		Image:      "ubuntu-24.04",
		ServerType: "cpx11",
		Location:   "fsn1",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if server.ID != "42" {
		t.Errorf("ID = %q, want %q", server.ID, "42")
	}
	if server.Name != "my-server" {
		t.Errorf("Name = %q, want %q", server.Name, "my-server")
	}
	if server.Status != "initializing" {
		t.Errorf("Status = %q, want %q", server.Status, "initializing")
	}
	if server.PublicIPv4 != "1.2.3.4" {
		t.Errorf("PublicIPv4 = %q, want %q", server.PublicIPv4, "1.2.3.4")
	}
	if server.Image != "ubuntu-24.04" {
		t.Errorf("Image = %q, want %q", server.Image, "ubuntu-24.04")
	}
	if server.Region != "fsn1" {
		t.Errorf("Region = %q, want %q", server.Region, "fsn1")
	}

	// No root password expected when not returned by API
	if _, ok := server.Metadata["root_password"]; ok {
		t.Errorf("expected no root_password in metadata")
	}
}

func TestCreateServer_WithSSHKeys(t *testing.T) {
	loc := testLocationJSON(1, "fsn1", "DE", "Falkenstein")
	st := testServerTypeJSON(1, "cpx11", "x86")
	img := testImageJSON(1, "ubuntu-24.04", "ubuntu", "24.04", "x86")

	createdServer := testServerJSON(50, "ssh-server", "initializing", "2024-06-15T12:00:00+00:00", loc, st)
	createdServer["image"] = img

	srv := newCreateTestServer(t, map[string]http.HandlerFunc{
		"GET /ssh_keys": func(w http.ResponseWriter, r *http.Request) {
			name := r.URL.Query().Get("name")
			w.Header().Set("Content-Type", "application/json")

			switch name {
			case "my-key":
				json.NewEncoder(w).Encode(map[string]interface{}{
					"ssh_keys": []interface{}{
						testSSHKeyJSON(10, "my-key", "aa:bb:cc", "ssh-rsa AAAA..."),
					},
				})
			case "deploy-key":
				json.NewEncoder(w).Encode(map[string]interface{}{
					"ssh_keys": []interface{}{
						testSSHKeyJSON(20, "deploy-key", "dd:ee:ff", "ssh-ed25519 AAAA..."),
					},
				})
			default:
				json.NewEncoder(w).Encode(map[string]interface{}{
					"ssh_keys": []interface{}{},
				})
			}
		},
		"POST /servers": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			bodyStr := string(body)

			// Verify SSH key IDs are in the request body
			if !strings.Contains(bodyStr, "10") || !strings.Contains(bodyStr, "20") {
				t.Errorf("expected SSH key IDs 10 and 20 in body, got: %s", bodyStr)
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"server":       createdServer,
				"action":       testActionJSON(2),
				"next_actions": []interface{}{},
			})
		},
	})

	ctx := context.Background()
	provider := newTestHetznerProvider(t, srv.URL, "test-token")

	server, err := provider.CreateServer(ctx, domain.CreateServerOpts{
		Name:       "ssh-server",
		Image:      "ubuntu-24.04",
		ServerType: "cpx11",
		SSHKeys:    []string{"my-key", "deploy-key"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if server.ID != "50" {
		t.Errorf("ID = %q, want %q", server.ID, "50")
	}
}

func TestCreateServer_RootPassword(t *testing.T) {
	loc := testLocationJSON(1, "fsn1", "DE", "Falkenstein")
	st := testServerTypeJSON(1, "cpx11", "x86")
	img := testImageJSON(1, "ubuntu-24.04", "ubuntu", "24.04", "x86")

	createdServer := testServerJSON(60, "nokey-server", "initializing", "2024-06-15T12:00:00+00:00", loc, st)
	createdServer["image"] = img

	srv := newCreateTestServer(t, map[string]http.HandlerFunc{
		"POST /servers": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"server":        createdServer,
				"action":        testActionJSON(3),
				"next_actions":  []interface{}{},
				"root_password": "YItygb1v4bSn",
			})
		},
	})

	ctx := context.Background()
	provider := newTestHetznerProvider(t, srv.URL, "test-token")

	server, err := provider.CreateServer(ctx, domain.CreateServerOpts{
		Name:       "nokey-server",
		Image:      "ubuntu-24.04",
		ServerType: "cpx11",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	pw, ok := server.Metadata["root_password"].(string)
	if !ok || pw == "" {
		t.Fatal("expected root_password in metadata, got none")
	}
	if pw != "YItygb1v4bSn" {
		t.Errorf("root_password = %q, want %q", pw, "YItygb1v4bSn")
	}
}

func TestCreateServer_SSHKeyNotFound(t *testing.T) {
	srv := newCreateTestServer(t, map[string]http.HandlerFunc{
		"GET /ssh_keys": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ssh_keys": []interface{}{},
			})
		},
	})

	ctx := context.Background()
	provider := newTestHetznerProvider(t, srv.URL, "test-token")

	_, err := provider.CreateServer(ctx, domain.CreateServerOpts{
		Name:       "fail-server",
		Image:      "ubuntu-24.04",
		ServerType: "cpx11",
		SSHKeys:    []string{"nonexistent-key"},
	})
	if err == nil {
		t.Fatal("expected error for missing SSH key, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent-key") {
		t.Errorf("expected error to mention 'nonexistent-key', got: %v", err)
	}
}

func TestCreateServer_APIError(t *testing.T) {
	srv := newCreateTestServer(t, map[string]http.HandlerFunc{
		"POST /servers": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"code":    "uniqueness_error",
					"message": "server name is already used",
				},
			})
		},
	})

	ctx := context.Background()
	provider := newTestHetznerProvider(t, srv.URL, "test-token")

	_, err := provider.CreateServer(ctx, domain.CreateServerOpts{
		Name:       "duplicate-server",
		Image:      "ubuntu-24.04",
		ServerType: "cpx11",
	})
	if err == nil {
		t.Fatal("expected error for API conflict, got nil")
	}
	if !strings.Contains(err.Error(), "conflict") {
		t.Errorf("expected error to mention 'conflict', got: %v", err)
	}
}

func TestCreateServer_WithLabelsAndUserData(t *testing.T) {
	loc := testLocationJSON(1, "fsn1", "DE", "Falkenstein")
	st := testServerTypeJSON(1, "cpx11", "x86")
	img := testImageJSON(1, "ubuntu-24.04", "ubuntu", "24.04", "x86")

	createdServer := testServerJSON(70, "labeled-server", "initializing", "2024-06-15T12:00:00+00:00", loc, st)
	createdServer["image"] = img

	srv := newCreateTestServer(t, map[string]http.HandlerFunc{
		"POST /servers": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			bodyStr := string(body)

			if !strings.Contains(bodyStr, `"env"`) || !strings.Contains(bodyStr, `"prod"`) {
				t.Errorf("expected label env=prod in body, got: %s", bodyStr)
			}
			if !strings.Contains(bodyStr, `"user_data"`) || !strings.Contains(bodyStr, "#!/bin/bash") {
				t.Errorf("expected user_data in body, got: %s", bodyStr)
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"server":       createdServer,
				"action":       testActionJSON(4),
				"next_actions": []interface{}{},
			})
		},
	})

	ctx := context.Background()
	provider := newTestHetznerProvider(t, srv.URL, "test-token")

	server, err := provider.CreateServer(ctx, domain.CreateServerOpts{
		Name:       "labeled-server",
		Image:      "ubuntu-24.04",
		ServerType: "cpx11",
		Labels:     map[string]string{"env": "prod"},
		UserData:   "#!/bin/bash\necho hello",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if server.ID != "70" {
		t.Errorf("ID = %q, want %q", server.ID, "70")
	}
}
