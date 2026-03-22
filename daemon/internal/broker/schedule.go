package broker

import (
	"context"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// initScheduler starts the start/stop cron instance and registers any schedules
// already stored on existing servers (survives daemon restarts).
func (b *Broker) initScheduler(ctx context.Context) {
	b.schedCron = cron.New()
	b.schedEntries = make(map[string][2]cron.EntryID)
	b.schedCron.Start()

	b.mu.RLock()
	type entry struct{ id, start, stop string }
	var toSchedule []entry
	for id, s := range b.servers {
		if s.StartSchedule != "" || s.StopSchedule != "" {
			toSchedule = append(toSchedule, entry{id, s.StartSchedule, s.StopSchedule})
		}
	}
	b.mu.RUnlock()

	for _, e := range toSchedule {
		b.scheduleStartStop(e.id, e.start, e.stop)
	}
	b.logger.Info("Start/stop scheduler initialized", zap.Int("registered", len(toSchedule)))
}

// scheduleStartStop registers (or re-registers) cron jobs for a server's
// start and stop schedules. An empty expression disables that half.
func (b *Broker) scheduleStartStop(id, startExpr, stopExpr string) {
	if b.schedCron == nil {
		return
	}
	b.unscheduleStartStop(id)

	var startID, stopID cron.EntryID

	if startExpr != "" {
		eid, err := b.schedCron.AddFunc(startExpr, func() {
			b.logger.Info("Scheduled start triggered", zap.String("id", id))
			if err := b.StartServer(context.Background(), id); err != nil {
				b.logger.Warn("Scheduled start failed", zap.String("id", id), zap.Error(err))
			}
		})
		if err != nil {
			b.logger.Warn("Failed to register start schedule",
				zap.String("id", id), zap.String("expr", startExpr), zap.Error(err))
		} else {
			startID = eid
			b.logger.Info("Start schedule registered", zap.String("id", id), zap.String("expr", startExpr))
		}
	}

	if stopExpr != "" {
		eid, err := b.schedCron.AddFunc(stopExpr, func() {
			b.logger.Info("Scheduled stop triggered", zap.String("id", id))
			if err := b.StopServer(context.Background(), id); err != nil {
				b.logger.Warn("Scheduled stop failed", zap.String("id", id), zap.Error(err))
			}
		})
		if err != nil {
			b.logger.Warn("Failed to register stop schedule",
				zap.String("id", id), zap.String("expr", stopExpr), zap.Error(err))
		} else {
			stopID = eid
			b.logger.Info("Stop schedule registered", zap.String("id", id), zap.String("expr", stopExpr))
		}
	}

	b.schedMu.Lock()
	b.schedEntries[id] = [2]cron.EntryID{startID, stopID}
	b.schedMu.Unlock()
}

// unscheduleStartStop removes any start/stop cron entries for the given server.
func (b *Broker) unscheduleStartStop(id string) {
	if b.schedCron == nil {
		return
	}
	b.schedMu.Lock()
	if ids, ok := b.schedEntries[id]; ok {
		if ids[0] != 0 {
			b.schedCron.Remove(ids[0])
		}
		if ids[1] != 0 {
			b.schedCron.Remove(ids[1])
		}
		delete(b.schedEntries, id)
	}
	b.schedMu.Unlock()
}
