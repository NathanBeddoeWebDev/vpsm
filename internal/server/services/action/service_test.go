package action

import (
	"context"
	"errors"
	"testing"
	"time"

	"nathanbeddoewebdev/vpsm/internal/actionstore"
	"nathanbeddoewebdev/vpsm/internal/server/domain"
)

// mockRepository implements actionstore.ActionRepository for testing.
type mockRepository struct {
	records            []actionstore.ActionRecord
	saveErr            error
	listPendingErr     error
	listRecentErr      error
	deleteOlderThanErr error
	deletedCount       int64
}

func (m *mockRepository) Save(record *actionstore.ActionRecord) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	if record.ID == 0 {
		record.ID = int64(len(m.records) + 1)
		record.CreatedAt = time.Now().UTC()
	}
	record.UpdatedAt = time.Now().UTC()
	m.records = append(m.records, *record)
	return nil
}

func (m *mockRepository) Get(id int64) (*actionstore.ActionRecord, error) {
	for _, r := range m.records {
		if r.ID == id {
			return &r, nil
		}
	}
	return nil, nil
}

func (m *mockRepository) ListPending() ([]actionstore.ActionRecord, error) {
	if m.listPendingErr != nil {
		return nil, m.listPendingErr
	}
	var pending []actionstore.ActionRecord
	for _, r := range m.records {
		if r.Status == "running" {
			pending = append(pending, r)
		}
	}
	return pending, nil
}

func (m *mockRepository) ListRecent(n int) ([]actionstore.ActionRecord, error) {
	if m.listRecentErr != nil {
		return nil, m.listRecentErr
	}
	if len(m.records) < n {
		return m.records, nil
	}
	return m.records[:n], nil
}

func (m *mockRepository) DeleteOlderThan(d time.Duration) (int64, error) {
	if m.deleteOlderThanErr != nil {
		return 0, m.deleteOlderThanErr
	}
	return m.deletedCount, nil
}

func (m *mockRepository) Close() error {
	return nil
}

// mockProvider implements domain.Provider for testing.
type mockProvider struct {
	getServerErr error
	server       *domain.Server
}

func (m *mockProvider) GetDisplayName() string {
	return "mock"
}

func (m *mockProvider) GetServer(ctx context.Context, id string) (*domain.Server, error) {
	if m.getServerErr != nil {
		return nil, m.getServerErr
	}
	return m.server, nil
}

func (m *mockProvider) ListServers(ctx context.Context) ([]domain.Server, error) {
	return nil, nil
}

func (m *mockProvider) CreateServer(ctx context.Context, opts domain.CreateServerOpts) (*domain.Server, error) {
	return nil, nil
}

func (m *mockProvider) DeleteServer(ctx context.Context, id string) error {
	return nil
}

func (m *mockProvider) StartServer(ctx context.Context, id string) (*domain.ActionStatus, error) {
	return nil, nil
}

func (m *mockProvider) StopServer(ctx context.Context, id string) (*domain.ActionStatus, error) {
	return nil, nil
}

func TestNewService(t *testing.T) {
	repo := &mockRepository{}
	provider := &mockProvider{}
	svc := NewService(provider, "test", repo)

	if svc.repo != repo {
		t.Error("expected repository to be set")
	}
	if svc.provider != provider {
		t.Error("expected provider to be set")
	}
	if svc.providerName != "test" {
		t.Errorf("expected providerName 'test', got %q", svc.providerName)
	}
}

func TestService_Close(t *testing.T) {
	t.Run("WithRepository", func(t *testing.T) {
		repo := &mockRepository{}
		svc := NewService(nil, "", repo)
		if err := svc.Close(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("WithoutRepository", func(t *testing.T) {
		svc := NewService(nil, "", nil)
		if err := svc.Close(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestService_SaveRecord(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		repo := &mockRepository{}
		svc := NewService(nil, "", repo)

		record := &actionstore.ActionRecord{
			Provider: "test",
			ServerID: "123",
			Status:   "running",
		}

		if err := svc.SaveRecord(record); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if record.ID == 0 {
			t.Error("expected ID to be assigned")
		}
	})

	t.Run("NilRepository", func(t *testing.T) {
		svc := NewService(nil, "", nil)
		record := &actionstore.ActionRecord{}
		if err := svc.SaveRecord(record); err != nil {
			t.Errorf("expected nil error, got: %v", err)
		}
	})

	t.Run("RepositoryError", func(t *testing.T) {
		repo := &mockRepository{saveErr: errors.New("save failed")}
		svc := NewService(nil, "", repo)
		record := &actionstore.ActionRecord{}
		if err := svc.SaveRecord(record); err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestService_TrackAction(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		repo := &mockRepository{}
		svc := NewService(nil, "test", repo)

		action := &domain.ActionStatus{
			ID:       "act-1",
			Status:   "running",
			Progress: 50,
		}

		record := svc.TrackAction("server-1", "web-1", action, "start_server", "running")
		if record == nil {
			t.Fatal("expected record, got nil")
		}

		if record.ActionID != "act-1" {
			t.Errorf("expected ActionID 'act-1', got %q", record.ActionID)
		}
		if record.ServerID != "server-1" {
			t.Errorf("expected ServerID 'server-1', got %q", record.ServerID)
		}
		if record.ServerName != "web-1" {
			t.Errorf("expected ServerName 'web-1', got %q", record.ServerName)
		}
		if record.Command != "start_server" {
			t.Errorf("expected Command 'start_server', got %q", record.Command)
		}
		if record.TargetStatus != "running" {
			t.Errorf("expected TargetStatus 'running', got %q", record.TargetStatus)
		}
	})

	t.Run("NilRepository", func(t *testing.T) {
		svc := NewService(nil, "test", nil)
		action := &domain.ActionStatus{ID: "act-1"}
		record := svc.TrackAction("server-1", "", action, "start_server", "running")
		if record != nil {
			t.Error("expected nil record when repository is nil")
		}
	})

	t.Run("NilAction", func(t *testing.T) {
		repo := &mockRepository{}
		svc := NewService(nil, "test", repo)
		record := svc.TrackAction("server-1", "", nil, "start_server", "running")
		if record != nil {
			t.Error("expected nil record when action is nil")
		}
	})
}

func TestService_FinalizeAction(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		repo := &mockRepository{}
		svc := NewService(nil, "test", repo)

		record := &actionstore.ActionRecord{
			Provider: "test",
			ServerID: "123",
			Status:   "running",
		}
		repo.Save(record)

		svc.FinalizeAction(record, domain.ActionStatusSuccess, "")

		if record.Status != domain.ActionStatusSuccess {
			t.Errorf("expected status %q, got %q", domain.ActionStatusSuccess, record.Status)
		}
		if record.Progress != 100 {
			t.Errorf("expected progress 100, got %d", record.Progress)
		}
		if record.ErrorMessage != "" {
			t.Errorf("expected empty error message, got %q", record.ErrorMessage)
		}
	})

	t.Run("Error", func(t *testing.T) {
		repo := &mockRepository{}
		svc := NewService(nil, "test", repo)

		record := &actionstore.ActionRecord{
			Provider: "test",
			ServerID: "123",
			Status:   "running",
		}
		repo.Save(record)

		svc.FinalizeAction(record, domain.ActionStatusError, "test error")

		if record.Status != domain.ActionStatusError {
			t.Errorf("expected status %q, got %q", domain.ActionStatusError, record.Status)
		}
		if record.ErrorMessage != "test error" {
			t.Errorf("expected error message 'test error', got %q", record.ErrorMessage)
		}
	})

	t.Run("NilRepository", func(t *testing.T) {
		svc := NewService(nil, "test", nil)
		record := &actionstore.ActionRecord{}
		// Should not panic
		svc.FinalizeAction(record, domain.ActionStatusSuccess, "")
	})

	t.Run("NilRecord", func(t *testing.T) {
		repo := &mockRepository{}
		svc := NewService(nil, "test", repo)
		// Should not panic
		svc.FinalizeAction(nil, domain.ActionStatusSuccess, "")
	})
}

func TestService_ListPending(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		repo := &mockRepository{
			records: []actionstore.ActionRecord{
				{ID: 1, Status: "running"},
				{ID: 2, Status: "success"},
				{ID: 3, Status: "running"},
			},
		}
		svc := NewService(nil, "test", repo)

		pending, err := svc.ListPending()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(pending) != 2 {
			t.Errorf("expected 2 pending actions, got %d", len(pending))
		}
	})

	t.Run("NilRepository", func(t *testing.T) {
		svc := NewService(nil, "test", nil)
		_, err := svc.ListPending()
		if err == nil {
			t.Error("expected error when repository is nil")
		}
	})
}

func TestService_ListRecent(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		repo := &mockRepository{
			records: []actionstore.ActionRecord{
				{ID: 1, Status: "running"},
				{ID: 2, Status: "success"},
				{ID: 3, Status: "error"},
			},
		}
		svc := NewService(nil, "test", repo)

		recent, err := svc.ListRecent(2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(recent) != 2 {
			t.Errorf("expected 2 recent actions, got %d", len(recent))
		}
	})

	t.Run("NilRepository", func(t *testing.T) {
		svc := NewService(nil, "test", nil)
		_, err := svc.ListRecent(10)
		if err == nil {
			t.Error("expected error when repository is nil")
		}
	})
}

func TestService_Cleanup(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		repo := &mockRepository{deletedCount: 5}
		svc := NewService(nil, "test", repo)

		count, err := svc.Cleanup(24 * time.Hour)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if count != 5 {
			t.Errorf("expected 5 deleted, got %d", count)
		}
	})

	t.Run("NilRepository", func(t *testing.T) {
		svc := NewService(nil, "test", nil)
		_, err := svc.Cleanup(24 * time.Hour)
		if err == nil {
			t.Error("expected error when repository is nil")
		}
	})
}
