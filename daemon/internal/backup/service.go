package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// Target backend type
const (
	TargetLocal = "local"
	TargetNFS   = "nfs"
	TargetS3    = "s3"
)

// Config holds backup service configuration
type Config struct {
	DefaultSchedule string `yaml:"default_schedule"`
	RetainDays      int    `yaml:"retain_days"`
	Compression     string `yaml:"compression"` // gzip|zstd
	DefaultTarget   string `yaml:"default_target"`
	// DataDir is the base directory for local backup archives.
	// Defaults to /var/lib/games-dashboard/backups.
	DataDir         string `yaml:"data_dir"`
}

// Record represents a completed backup
type Record struct {
	ID        string    `json:"id"`
	ServerID  string    `json:"server_id"`
	Type      string    `json:"type"` // full|incremental
	Target    string    `json:"target"`
	SizeBytes int64     `json:"size_bytes"`
	Checksum  string    `json:"checksum"`
	Paths     []string  `json:"paths"`
	CreatedAt time.Time `json:"created_at"`
	Status    string    `json:"status"` // pending|running|complete|failed
	Error     string    `json:"error,omitempty"`
}

// Job represents a running backup or restore operation
type Job struct {
	ID        string    `json:"id"`
	ServerID  string    `json:"server_id"`
	Type      string    `json:"type"` // backup|restore
	Status    string    `json:"status"`
	Progress  int       `json:"progress"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ServerBackupConfig holds per-server backup configuration
type ServerBackupConfig struct {
	Enabled    bool     `json:"enabled"`
	Target     string   `json:"target"`
	Schedule   string   `json:"schedule"`
	RetainDays int      `json:"retain_days"`
	Paths      []string `json:"paths"`
}

// Service manages backup scheduling, execution, and retention
type Service struct {
	cfg     Config
	logger  *zap.Logger
	cron    *cron.Cron
	mu      sync.RWMutex
	records map[string][]*Record // serverID -> []Record
	jobs    map[string]*Job
	// schedules maps serverID -> cron entry ID
	schedules map[string]cron.EntryID
}

// NewService creates a new backup service
func NewService(cfg Config, logger *zap.Logger) *Service {
	if cfg.DefaultSchedule == "" {
		cfg.DefaultSchedule = "0 3 * * *"
	}
	if cfg.RetainDays == 0 {
		cfg.RetainDays = 30
	}
	if cfg.Compression == "" {
		cfg.Compression = "gzip"
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "/var/lib/games-dashboard/backups"
	}

	return &Service{
		cfg:       cfg,
		logger:    logger,
		cron:      cron.New(cron.WithSeconds()),
		records:   make(map[string][]*Record),
		jobs:      make(map[string]*Job),
		schedules: make(map[string]cron.EntryID),
	}
}

// Start begins the cron scheduler
func (s *Service) Start(ctx context.Context) {
	s.cron.Start()
	s.logger.Info("Backup service started")

	<-ctx.Done()
	s.logger.Info("Backup service stopping")
	s.cron.Stop()
}

// ScheduleServer registers a cron job for a server's backups
func (s *Service) ScheduleServer(serverID string, cfg ServerBackupConfig) error {
	if !cfg.Enabled {
		s.UnscheduleServer(serverID)
		return nil
	}

	schedule := cfg.Schedule
	if schedule == "" {
		schedule = s.cfg.DefaultSchedule
	}

	// Remove existing schedule
	s.UnscheduleServer(serverID)

	entryID, err := s.cron.AddFunc(schedule, func() {
		s.logger.Info("Running scheduled backup", zap.String("server", serverID))
		job := s.newJob(serverID, "backup")
		go s.executeBackup(context.Background(), serverID, cfg.Paths, cfg.Target, "full", job)
	})
	if err != nil {
		return fmt.Errorf("invalid cron schedule %q: %w", schedule, err)
	}

	s.mu.Lock()
	s.schedules[serverID] = entryID
	s.mu.Unlock()

	s.logger.Info("Backup scheduled",
		zap.String("server", serverID),
		zap.String("schedule", schedule))
	return nil
}

// UnscheduleServer removes a server's backup schedule
func (s *Service) UnscheduleServer(serverID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.schedules[serverID]; ok {
		s.cron.Remove(id)
		delete(s.schedules, serverID)
	}
}

// TriggerBackup starts an immediate backup for a server
func (s *Service) TriggerBackup(ctx context.Context, serverID string, paths []string, target, backupType string) (*Job, error) {
	job := s.newJob(serverID, "backup")
	go s.executeBackup(context.Background(), serverID, paths, target, backupType, job)
	return job, nil
}

// Restore starts a restore from a backup record
func (s *Service) Restore(ctx context.Context, serverID, backupID string) (*Job, error) {
	s.mu.RLock()
	var record *Record
	for _, r := range s.records[serverID] {
		if r.ID == backupID {
			record = r
			break
		}
	}
	s.mu.RUnlock()

	if record == nil {
		return nil, fmt.Errorf("backup %s not found for server %s", backupID, serverID)
	}

	job := s.newJob(serverID, "restore")
	go s.executeRestore(context.Background(), serverID, record, job)
	return job, nil
}

// ListRecords returns all backup records for a server
func (s *Service) ListRecords(serverID string) []*Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Record, len(s.records[serverID]))
	copy(result, s.records[serverID])
	return result
}

// GetJob returns a job by ID
func (s *Service) GetJob(jobID string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[jobID]
	return j, ok
}

func (s *Service) executeBackup(ctx context.Context, serverID string, paths []string, target, backupType string, job *Job) {
	s.updateJob(job.ID, "running", 10, "Preparing backup...")
	s.logger.Info("Starting backup",
		zap.String("server", serverID),
		zap.String("type", backupType),
		zap.String("target", target))

	record := &Record{
		ID:        uuid.New().String(),
		ServerID:  serverID,
		Type:      backupType,
		Target:    target,
		Paths:     paths,
		CreatedAt: time.Now(),
		Status:    "running",
	}

	s.mu.Lock()
	s.records[serverID] = append(s.records[serverID], record)
	s.mu.Unlock()

	s.updateJob(job.ID, "running", 30, "Archiving files...")

	var totalSize int64
	h := sha256.New()

	for i, path := range paths {
		if err := ctx.Err(); err != nil {
			s.failRecord(record, "backup cancelled")
			s.updateJob(job.ID, "failed", 0, "Backup cancelled")
			return
		}

		size, checksum, err := s.archivePath(path, target, record.ID)
		if err != nil {
			s.logger.Warn("Failed to archive path",
				zap.String("path", path),
				zap.Error(err))
			// Non-fatal: continue with other paths
		}
		totalSize += size
		io.WriteString(h, checksum)

		progress := 30 + int(float64(i+1)/float64(len(paths))*60)
		s.updateJob(job.ID, "running", progress, fmt.Sprintf("Archived %s", filepath.Base(path)))
	}

	record.SizeBytes = totalSize
	record.Checksum = fmt.Sprintf("sha256:%x", h.Sum(nil))
	record.Status = "complete"

	s.mu.Lock()
	s.updateRecordStatus(serverID, record.ID, "complete")
	s.mu.Unlock()

	s.updateJob(job.ID, "success", 100, fmt.Sprintf("Backup complete (%s)", humanizeBytes(totalSize)))
	s.logger.Info("Backup complete",
		zap.String("server", serverID),
		zap.String("id", record.ID),
		zap.Int64("size_bytes", totalSize))

	// Prune old backups
	s.pruneOldBackups(serverID)
}

func (s *Service) executeRestore(ctx context.Context, serverID string, record *Record, job *Job) {
	s.updateJob(job.ID, "running", 10, "Preparing restore...")
	s.logger.Info("Starting restore",
		zap.String("server", serverID),
		zap.String("backup_id", record.ID))

	s.updateJob(job.ID, "running", 50, "Restoring files...")

	for i, path := range record.Paths {
		if err := ctx.Err(); err != nil {
			s.updateJob(job.ID, "failed", 0, "Restore cancelled")
			return
		}

		if err := s.restorePath(record.ID, record.Target, path); err != nil {
			s.logger.Warn("Failed to restore path",
				zap.String("path", path),
				zap.Error(err))
		}

		progress := 50 + int(float64(i+1)/float64(len(record.Paths))*45)
		s.updateJob(job.ID, "running", progress, fmt.Sprintf("Restored %s", filepath.Base(path)))
	}

	s.updateJob(job.ID, "success", 100, "Restore complete")
	s.logger.Info("Restore complete", zap.String("server", serverID), zap.String("backup_id", record.ID))
}

// localArchiveDir returns the directory where backup archives are stored.
// If target is an absolute path it is used as the base; otherwise DataDir is used.
func (s *Service) localArchiveDir(target, backupID string) string {
	base := s.cfg.DataDir
	if strings.HasPrefix(target, "/") {
		base = target
	}
	return filepath.Join(base, backupID)
}

func (s *Service) archivePath(sourcePath, target, backupID string) (size int64, checksum string, err error) {
	archiveDir := s.localArchiveDir(target, backupID)
	if mkErr := os.MkdirAll(archiveDir, 0o750); mkErr != nil {
		return 0, "", fmt.Errorf("create archive dir %s: %w", archiveDir, mkErr)
	}

	archiveFile := filepath.Join(archiveDir, filepath.Base(sourcePath)+".tar.gz")

	f, createErr := os.Create(archiveFile)
	if createErr != nil {
		return 0, "", fmt.Errorf("create archive file: %w", createErr)
	}
	defer f.Close()

	// Write simultaneously to file and SHA-256 hasher
	h := sha256.New()
	mw := io.MultiWriter(f, h)

	gz := gzip.NewWriter(mw)
	tw := tar.NewWriter(gz)

	walkErr := filepath.Walk(sourcePath, func(p string, fi os.FileInfo, walkE error) error {
		if walkE != nil {
			return nil // skip unreadable entries
		}
		rel, relErr := filepath.Rel(filepath.Dir(sourcePath), p)
		if relErr != nil {
			return nil
		}

		hdr, hdrErr := tar.FileInfoHeader(fi, "")
		if hdrErr != nil {
			return nil
		}
		hdr.Name = rel

		if whErr := tw.WriteHeader(hdr); whErr != nil {
			return whErr
		}
		if fi.IsDir() || !fi.Mode().IsRegular() {
			return nil
		}

		src, openErr := os.Open(p)
		if openErr != nil {
			return nil // skip unreadable files
		}
		defer src.Close()
		_, copyErr := io.Copy(tw, src)
		return copyErr
	})
	if walkErr != nil {
		return 0, "", fmt.Errorf("archiving %s: %w", sourcePath, walkErr)
	}

	if closeErr := tw.Close(); closeErr != nil {
		return 0, "", fmt.Errorf("tar close: %w", closeErr)
	}
	if closeErr := gz.Close(); closeErr != nil {
		return 0, "", fmt.Errorf("gzip close: %w", closeErr)
	}

	stat, statErr := f.Stat()
	if statErr == nil {
		size = stat.Size()
	}
	return size, fmt.Sprintf("sha256:%x", h.Sum(nil)), nil
}

func (s *Service) restorePath(backupID, target, destPath string) error {
	archiveDir := s.localArchiveDir(target, backupID)
	archiveFile := filepath.Join(archiveDir, filepath.Base(destPath)+".tar.gz")

	f, openErr := os.Open(archiveFile)
	if openErr != nil {
		return fmt.Errorf("open archive %s: %w", archiveFile, openErr)
	}
	defer f.Close()

	gr, gzErr := gzip.NewReader(f)
	if gzErr != nil {
		return fmt.Errorf("gzip reader: %w", gzErr)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	restoreRoot := filepath.Dir(destPath)
	cleanRoot := filepath.Clean(restoreRoot)

	for {
		hdr, nextErr := tr.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return fmt.Errorf("tar read: %w", nextErr)
		}

		outPath := filepath.Join(restoreRoot, filepath.Clean(hdr.Name))
		// Prevent path traversal: ensure the target stays within restoreRoot.
		if !strings.HasPrefix(filepath.Clean(outPath), cleanRoot+string(os.PathSeparator)) &&
			filepath.Clean(outPath) != cleanRoot {
			s.logger.Warn("Skipping archive entry outside restore root",
				zap.String("entry", hdr.Name))
			continue
		}

		if hdr.FileInfo().IsDir() {
			os.MkdirAll(outPath, hdr.FileInfo().Mode())
			continue
		}

		if mkErr := os.MkdirAll(filepath.Dir(outPath), 0o750); mkErr != nil {
			continue
		}
		out, createErr := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode())
		if createErr != nil {
			s.logger.Warn("Failed to create restore file", zap.String("path", outPath), zap.Error(createErr))
			continue
		}
		_, copyErr := io.Copy(out, tr)
		out.Close()
		if copyErr != nil {
			s.logger.Warn("Failed to write restore file", zap.String("path", outPath), zap.Error(copyErr))
		}
	}

	s.logger.Debug("Restore complete", zap.String("backup_id", backupID), zap.String("dest", destPath))
	return nil
}

func (s *Service) pruneOldBackups(serverID string) {
	retainDays := s.cfg.RetainDays
	if retainDays <= 0 {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -retainDays)

	s.mu.Lock()
	defer s.mu.Unlock()

	records := s.records[serverID]
	kept := records[:0]
	pruned := 0
	for _, r := range records {
		if r.CreatedAt.After(cutoff) {
			kept = append(kept, r)
		} else {
			pruned++
		}
	}

	s.records[serverID] = kept
	if pruned > 0 {
		s.logger.Info("Pruned old backups",
			zap.String("server", serverID),
			zap.Int("pruned", pruned))
	}
}

func (s *Service) newJob(serverID, jobType string) *Job {
	job := &Job{
		ID:        uuid.New().String(),
		ServerID:  serverID,
		Type:      jobType,
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.mu.Lock()
	s.jobs[job.ID] = job
	s.mu.Unlock()
	return job
}

func (s *Service) updateJob(jobID, status string, progress int, message string) {
	s.mu.Lock()
	if j, ok := s.jobs[jobID]; ok {
		j.Status = status
		j.Progress = progress
		j.Message = message
		j.UpdatedAt = time.Now()
	}
	s.mu.Unlock()
}

func (s *Service) failRecord(record *Record, msg string) {
	s.mu.Lock()
	record.Status = "failed"
	record.Error = msg
	s.mu.Unlock()
}

func (s *Service) updateRecordStatus(serverID, recordID, status string) {
	for _, r := range s.records[serverID] {
		if r.ID == recordID {
			r.Status = status
			return
		}
	}
}

func humanizeBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
