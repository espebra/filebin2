package lurker

import (
	"log/slog"
	"time"

	"github.com/espebra/filebin2/internal/dbl"
	"github.com/espebra/filebin2/internal/s3"
	"github.com/espebra/filebin2/internal/workspace"
)

type Lurker struct {
	dao       *dbl.DAO
	s3        *s3.S3AO
	workspace *workspace.Manager
	interval  time.Duration
	throttle  time.Duration
	retention uint64
	stopChan  chan struct{}
}

// New creates a new Lurker instance
func New(dao *dbl.DAO, s3ao *s3.S3AO, wm *workspace.Manager) *Lurker {
	return &Lurker{
		dao:       dao,
		s3:        s3ao,
		workspace: wm,
	}
}

func (l *Lurker) Init(interval int, throttle int, retention uint64) {
	l.interval = time.Second * time.Duration(interval)
	l.throttle = time.Millisecond * time.Duration(throttle)
	l.retention = retention
}

func (l *Lurker) Run() {
	slog.Info("starting lurker process", "interval_seconds", l.interval.Seconds())
	l.stopChan = make(chan struct{})
	go func() {
		for {
			l.runOnce()
			select {
			case <-time.After(l.interval):
				// continue to next iteration
			case <-l.stopChan:
				slog.Info("lurker stopped")
				return
			}
		}
	}()
}

func (l *Lurker) runOnce() {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("lurker recovered from panic", "panic", r)
		}
	}()
	t0 := time.Now()
	l.DeletePendingBins()
	l.DeletePendingContent()
	l.CleanTransactions()
	l.CleanClients()
	l.CleanWorkspaceFiles()
	slog.Debug("lurker completed run", "duration_seconds", time.Since(t0).Seconds())
}

func (l *Lurker) Stop() {
	if l.stopChan != nil {
		close(l.stopChan)
	}
}

func (l *Lurker) DeletePendingBins() {
	bins, err := l.dao.Bin().GetPendingDelete()
	if err != nil {
		slog.Error("unable to get pending bin deletions", "error", err)
		return
	}
	if len(bins) > 0 {
		slog.Info("found bins pending removal", "count", len(bins))
		for _, bin := range bins {
			// Mark bin as deleted
			_ = bin.DeletedAt.Scan(time.Now().UTC())
			// Bin deletion cascades to files (sets deleted_at)
			// Orphaned content will be detected by DeletePendingContent using COUNT(*)
			if err := l.dao.Bin().Update(&bin); err != nil {
				slog.Error("unable to update bin", "bin", bin.Id, "error", err)
				return
			}
			slog.Info("marked bin as deleted", "bin", bin.Id)
		}
	}
}

func (l *Lurker) DeletePendingContent() {
	contents, err := l.dao.FileContent().GetPendingDelete()
	if err != nil {
		slog.Error("unable to get pending content deletions", "error", err)
		return
	}
	if len(contents) > 0 {
		slog.Info("found content objects pending removal", "count", len(contents))
		for _, content := range contents {
			// Atomically claim the content for deletion: this sets
			// in_storage=false only if there are still zero active file
			// references. Committing this before the S3 delete ensures a
			// concurrent deduplicated upload observes in_storage=false and
			// re-uploads the object rather than skipping the upload and
			// leaving a reference to a deleted object (data loss).
			claimed, err := l.dao.FileContent().ClaimForDeletion(content.SHA256)
			if err != nil {
				slog.Error("unable to claim content for deletion", "sha256", content.SHA256, "error", err)
				continue
			}
			if !claimed {
				// The content gained an active reference (a new upload) or was
				// already claimed since GetPendingDelete ran. Leave it alone.
				slog.Debug("skipping content that is referenced or already claimed", "sha256", content.SHA256)
				continue
			}

			// Delete from S3 now that the claim is committed.
			if err := l.s3.RemoveObjectByHash(content.SHA256); err != nil {
				slog.Error("failed to remove object from S3", "sha256", content.SHA256, "error", err)
				// Roll back the claim so the still-present object is retried on
				// a later run rather than being leaked with in_storage=false.
				if rbErr := l.dao.FileContent().SetInStorage(content.SHA256, true); rbErr != nil {
					slog.Error("unable to roll back deletion claim", "sha256", content.SHA256, "error", rbErr)
				}
				continue
			}

			// Throttle to reduce load during bulk deletions
			if l.throttle > 0 {
				time.Sleep(l.throttle)
			}
		}
	}
}

func (l *Lurker) CleanTransactions() {
	count, err := l.dao.Transaction().Cleanup(l.retention)
	if err != nil {
		slog.Error("unable to cleanup transactions", "error", err)
		return
	}
	if count > 0 {
		slog.Info("removed log transactions", "count", count)
	}
}

func (l *Lurker) CleanWorkspaceFiles() {
	if l.workspace == nil {
		return
	}
	l.workspace.CleanStaleFiles(24 * time.Hour)
}

func (l *Lurker) CleanClients() {
	count, err := l.dao.Client().Cleanup(l.retention)
	if err != nil {
		slog.Error("unable to cleanup clients", "error", err)
		return
	}
	if count > 0 {
		slog.Info("removed client entries", "count", count)
	}
}
