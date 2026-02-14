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
			// Safety check: verify no files reference this content
			count, err := l.dao.File().CountBySHA256(content.SHA256)
			if err != nil {
				slog.Error("unable to count files for SHA256", "sha256", content.SHA256, "error", err)
				continue
			}
			if count > 0 {
				slog.Debug("skipping content with active file references", "sha256", content.SHA256, "references", count)
				continue
			}

			// Delete from S3
			if err := l.s3.RemoveObjectByHash(content.SHA256); err != nil {
				slog.Error("failed to remove object from S3", "sha256", content.SHA256, "error", err)
				return
			}

			// Mark as not in storage (or delete the record)
			content.InStorage = false
			if err := l.dao.FileContent().Update(&content); err != nil {
				slog.Error("unable to update file_content", "sha256", content.SHA256, "error", err)
				return
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
