package broker

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// TriggerServerUpdate stops the server if running, re-deploys it to pick up any
// available updates, then restarts it if it was previously running.
// SteamCMD's app_update and docker pull are both idempotent — if the server is
// already up-to-date, this is a no-op (no files are re-downloaded).
func (b *Broker) TriggerServerUpdate(ctx context.Context, id string) (*Job, error) {
	b.mu.RLock()
	s, ok := b.servers[id]
	b.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("server not found: %s", id)
	}
	if s.DeployMethod == "" {
		return nil, fmt.Errorf("server has no deploy method configured — deploy it before updating")
	}
	job := b.newJob("update", id)
	go b.doUpdate(context.Background(), id, job)
	return job, nil
}

func (b *Broker) doUpdate(ctx context.Context, id string, job *Job) {
	b.logger.Info("Starting server update", zap.String("id", id))

	b.mu.RLock()
	s, ok := b.servers[id]
	if !ok {
		b.mu.RUnlock()
		b.updateJob(job.ID, "failed", 0, "server not found")
		return
	}
	wasRunning := s.State == StateRunning
	deployMethod := s.DeployMethod
	b.mu.RUnlock()

	// Step 1: stop if running.
	if wasRunning {
		b.updateJob(job.ID, "running", 10, "Stopping server before update...")
		b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"system","msg":"[update] Stopping server before update...","ts":%d}`, time.Now().Unix()))
		if err := b.StopServer(ctx, id); err != nil {
			b.updateJob(job.ID, "failed", 0, "failed to stop server: "+err.Error())
			return
		}
		for i := 0; i < 30; i++ {
			b.mu.RLock()
			state := b.servers[id].State
			b.mu.RUnlock()
			if state == StateStopped || state == StateIdle {
				break
			}
			time.Sleep(1 * time.Second)
		}
	}

	// Step 2: re-deploy (downloads only changed content).
	b.updateJob(job.ID, "running", 30, "Downloading update...")
	b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"system","msg":"[update] Downloading update...","ts":%d}`, time.Now().Unix()))
	deployJob, err := b.DeployServer(ctx, id, DeployRequest{Method: deployMethod, Force: true})
	if err != nil {
		b.updateJob(job.ID, "failed", 0, "failed to start deploy: "+err.Error())
		return
	}
	// Override DeployServer's StateDeploying with StateUpdating so the UI
	// can distinguish a first-time install from a game-files update.
	b.mu.Lock()
	if sv, ok := b.servers[id]; ok {
		sv.State = StateUpdating
	}
	b.mu.Unlock()

	// Poll until deploy finishes (max 10 min).
	for i := 0; i < 600; i++ {
		b.mu.RLock()
		dj, exists := b.jobs[deployJob.ID]
		b.mu.RUnlock()
		if !exists {
			break
		}
		if dj.Status == "success" {
			break
		}
		if dj.Status == "failed" {
			b.updateJob(job.ID, "failed", 0, "deploy failed: "+dj.Message)
			return
		}
		time.Sleep(1 * time.Second)
	}

	// Record update timestamp.
	now := time.Now()
	b.mu.Lock()
	if sv, ok := b.servers[id]; ok {
		sv.LastUpdateCheck = &now
	}
	b.saveServersLocked()
	b.mu.Unlock()

	b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"system","msg":"[update] Update complete","ts":%d}`, time.Now().Unix()))

	// Step 3: restart if the server was running before the update.
	if wasRunning {
		b.updateJob(job.ID, "running", 90, "Restarting server...")
		b.sendConsoleMessage(id, fmt.Sprintf(`{"type":"system","msg":"[update] Restarting server...","ts":%d}`, time.Now().Unix()))
		if err := b.StartServer(ctx, id); err != nil {
			b.updateJob(job.ID, "failed", 90, "update complete but failed to restart: "+err.Error())
			return
		}
	}

	b.updateJob(job.ID, "success", 100, "Update complete")
	b.logger.Info("Server update complete", zap.String("id", id))
}

// scheduleAutoUpdate registers a cron job for this server's auto-update schedule.
// Replaces any existing entry. Uses b.updateMu (not b.mu) to avoid lock ordering issues.
func (b *Broker) scheduleAutoUpdate(id, schedule string) {
	if b.updateCron == nil {
		return
	}
	b.unscheduleAutoUpdate(id)
	if schedule == "" {
		schedule = "0 4 * * *" // default: 4 AM daily
	}
	entryID, err := b.updateCron.AddFunc(schedule, func() {
		b.mu.RLock()
		s, ok := b.servers[id]
		b.mu.RUnlock()
		if !ok || !s.AutoUpdate {
			return
		}
		b.logger.Info("Auto-update triggered by schedule", zap.String("id", id))
		if _, err := b.TriggerServerUpdate(context.Background(), id); err != nil {
			b.logger.Warn("Auto-update failed", zap.String("id", id), zap.Error(err))
		}
	})
	if err != nil {
		b.logger.Warn("Failed to schedule auto-update", zap.String("id", id), zap.Error(err))
		return
	}
	b.updateMu.Lock()
	b.updateEntries[id] = entryID
	b.updateMu.Unlock()
	b.logger.Info("Auto-update scheduled", zap.String("id", id), zap.String("schedule", schedule))
}

// unscheduleAutoUpdate removes any cron entry for the given server.
func (b *Broker) unscheduleAutoUpdate(id string) {
	if b.updateCron == nil {
		return
	}
	b.updateMu.Lock()
	if entryID, ok := b.updateEntries[id]; ok {
		b.updateCron.Remove(entryID)
		delete(b.updateEntries, id)
	}
	b.updateMu.Unlock()
}

// initAutoUpdateScheduler starts the cron scheduler and registers schedules
// for all servers that already have auto-update enabled.
func (b *Broker) initAutoUpdateScheduler(ctx context.Context) {
	b.updateCron = cron.New()
	b.updateEntries = make(map[string]cron.EntryID)
	b.updateCron.Start()

	b.mu.RLock()
	type entry struct{ id, schedule string }
	var toSchedule []entry
	for id, s := range b.servers {
		if s.AutoUpdate {
			toSchedule = append(toSchedule, entry{id, s.AutoUpdateSchedule})
		}
	}
	b.mu.RUnlock()

	for _, e := range toSchedule {
		b.scheduleAutoUpdate(e.id, e.schedule)
	}

	go func() {
		<-ctx.Done()
		b.updateCron.Stop()
	}()
}
