package backup

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	return NewService(Config{
		DataDir:         t.TempDir(),
		DefaultSchedule: "0 3 * * *",
		RetainDays:      30,
		Compression:     "gzip",
	}, zap.NewNop())
}

func TestNewService_Defaults(t *testing.T) {
	svc := NewService(Config{}, zap.NewNop())
	if svc.cfg.DefaultSchedule != "0 3 * * *" {
		t.Errorf("default schedule = %q, want 0 3 * * *", svc.cfg.DefaultSchedule)
	}
	if svc.cfg.RetainDays != 30 {
		t.Errorf("retain days = %d, want 30", svc.cfg.RetainDays)
	}
	if svc.cfg.Compression != "gzip" {
		t.Errorf("compression = %q, want gzip", svc.cfg.Compression)
	}
}

func TestListRecords_Empty(t *testing.T) {
	svc := newTestService(t)
	records := svc.ListRecords("srv-1")
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestTriggerBackup_ReturnsJob(t *testing.T) {
	svc := newTestService(t)
	job, err := svc.TriggerBackup(context.Background(), "srv-1", []string{}, "local", "full")
	if err != nil {
		t.Fatalf("TriggerBackup error: %v", err)
	}
	if job == nil {
		t.Fatal("expected job, got nil")
	}
	if job.ServerID != "srv-1" {
		t.Errorf("job.ServerID = %q, want srv-1", job.ServerID)
	}
	if job.Type != "backup" {
		t.Errorf("job.Type = %q, want backup", job.Type)
	}
}

func TestGetJob_Found(t *testing.T) {
	svc := newTestService(t)
	job, _ := svc.TriggerBackup(context.Background(), "srv-1", []string{}, "local", "full")

	got, ok := svc.GetJob(job.ID)
	if !ok {
		t.Fatal("expected job to be found")
	}
	if got.ID != job.ID {
		t.Errorf("job ID mismatch: got %s, want %s", got.ID, job.ID)
	}
}

func TestGetJob_NotFound(t *testing.T) {
	svc := newTestService(t)
	_, ok := svc.GetJob("nonexistent-id")
	if ok {
		t.Error("expected ok=false for unknown job ID")
	}
}

func TestScheduleServer_InvalidCron(t *testing.T) {
	svc := newTestService(t)
	err := svc.ScheduleServer("srv-1", ServerBackupConfig{
		Enabled:  true,
		Schedule: "not-a-valid-cron",
		Paths:    []string{"/data"},
	})
	if err == nil {
		t.Error("expected error for invalid cron schedule, got nil")
	}
}

func TestScheduleServer_Disabled(t *testing.T) {
	svc := newTestService(t)
	// Disabled should succeed and not schedule anything
	err := svc.ScheduleServer("srv-1", ServerBackupConfig{Enabled: false})
	if err != nil {
		t.Errorf("unexpected error scheduling disabled server: %v", err)
	}
}

func TestUnscheduleServer_NoOp(t *testing.T) {
	svc := newTestService(t)
	// Should not panic if server was never scheduled
	svc.UnscheduleServer("nonexistent")
}

func TestScheduleServer_ThenUnschedule(t *testing.T) {
	svc := newTestService(t)
	err := svc.ScheduleServer("srv-1", ServerBackupConfig{
		Enabled:  true,
		Schedule: "0 0 4 * * *", // cron.WithSeconds requires 6 fields
		Paths:    []string{},
	})
	if err != nil {
		t.Fatalf("schedule error: %v", err)
	}
	svc.UnscheduleServer("srv-1")
	// Unscheduling again should be a no-op
	svc.UnscheduleServer("srv-1")
}

func TestRestore_BackupNotFound(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.Restore(context.Background(), "srv-1", "fake-backup-id")
	if err == nil {
		t.Error("expected error restoring nonexistent backup, got nil")
	}
}

func TestHumanizeBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1024 * 1024, "1.0 MB"},
		{1536 * 1024, "1.5 MB"},
	}
	for _, tc := range tests {
		got := humanizeBytes(tc.input)
		if got != tc.want {
			t.Errorf("humanizeBytes(%d) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestPruneOldBackups(t *testing.T) {
	svc := NewService(Config{
		DataDir:    t.TempDir(),
		RetainDays: 1,
	}, zap.NewNop())

	// Inject a record old enough to be pruned
	old := &Record{
		ID:        "old-record",
		ServerID:  "srv-1",
		Status:    "complete",
		CreatedAt: time.Now().AddDate(0, 0, -5),
	}
	fresh := &Record{
		ID:        "fresh-record",
		ServerID:  "srv-1",
		Status:    "complete",
		CreatedAt: time.Now(),
	}

	svc.mu.Lock()
	svc.records["srv-1"] = []*Record{old, fresh}
	svc.mu.Unlock()

	svc.pruneOldBackups("srv-1")

	records := svc.ListRecords("srv-1")
	if len(records) != 1 {
		t.Errorf("expected 1 record after pruning, got %d", len(records))
	}
	if records[0].ID != "fresh-record" {
		t.Errorf("expected fresh-record to survive, got %s", records[0].ID)
	}
}
