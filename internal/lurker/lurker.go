package lurker

import (
	"log/slog"
	"time"

	"github.com/espebra/filebin2/internal/dbl"
	"github.com/espebra/filebin2/internal/ds"
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

	// Progress of the backfill of files in bins that were soft-deleted
	// before files were soft-deleted together with their bin
	backfillPause   time.Duration
	backfillStarted bool
	backfillDone    bool
	backfillNext    int64
	backfillMax     int64
	backfillCount   int64
}

const (
	// Number of file ids to process per backfill batch
	backfillBatchSize = 10000

	// Maximum time to spend on the backfill per lurker run, so that the
	// other lurker tasks are not delayed for long
	backfillRunBudget = time.Minute
)

// New creates a new Lurker instance
func New(dao *dbl.DAO, s3ao *s3.S3AO, wm *workspace.Manager) *Lurker {
	return &Lurker{
		dao:           dao,
		s3:            s3ao,
		workspace:     wm,
		backfillPause: 500 * time.Millisecond,
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
	l.BackfillDeletedFiles()
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
			// Mark bin as deleted, but only if it is still expired: a
			// concurrent upload may have revived the bin by extending its
			// expiration since GetPendingDelete ran, and the guarded update
			// leaves such bins alone instead of reverting the extension.
			// Orphaned content will be detected by DeletePendingContent
			// using COUNT(*).
			deleted, err := l.dao.Bin().MarkDeletedIfExpired(&bin)
			if err != nil {
				slog.Error("unable to mark bin as deleted", "bin", bin.Id, "error", err)
				continue
			}
			if !deleted {
				slog.Debug("skipping bin that was revived or already deleted", "bin", bin.Id)
				continue
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
			deleted := l.deleteContent(content)

			// Throttle to reduce load during bulk deletions
			if deleted && l.throttle > 0 {
				time.Sleep(l.throttle)
			}
		}
	}
}

// deleteContent claims and removes a single content object from S3. It
// returns true if the object was deleted. The per-content lock serializes
// the claim and the S3 delete against uploads of the same content, so an
// upload can never observe in_storage=true while a delete of the object is
// in flight.
func (l *Lurker) deleteContent(content ds.FileContent) bool {
	unlock, acquired, err := l.dao.FileContent().TryLockContent(content.SHA256)
	if err != nil {
		slog.Error("unable to lock content for deletion", "sha256", content.SHA256, "error", err)
		return false
	}
	if !acquired {
		// An upload of this content is in flight and will create an active
		// reference. Leave the content alone; if it really is orphaned, it is
		// picked up again on the next run.
		slog.Debug("skipping content locked by a concurrent upload", "sha256", content.SHA256)
		return false
	}
	defer unlock()

	// Atomically claim the content for deletion: this sets in_storage=false
	// only if there are still zero active file references. The claim is
	// committed before the S3 delete so that once the lock is released, a
	// deduplicated upload observes in_storage=false and re-uploads the object
	// rather than referencing a deleted one.
	claimed, err := l.dao.FileContent().ClaimForDeletion(content.SHA256)
	if err != nil {
		slog.Error("unable to claim content for deletion", "sha256", content.SHA256, "error", err)
		return false
	}
	if !claimed {
		// The content gained an active reference (a new upload) or was
		// already claimed since GetPendingDelete ran. Leave it alone.
		slog.Debug("skipping content that is referenced or already claimed", "sha256", content.SHA256)
		return false
	}

	// Delete from S3 now that the claim is committed.
	if err := l.s3.RemoveObjectByHash(content.SHA256); err != nil {
		slog.Error("failed to remove object from S3", "sha256", content.SHA256, "error", err)
		// Roll back the claim so the still-present object is retried on
		// a later run rather than being leaked with in_storage=false.
		if rbErr := l.dao.FileContent().SetInStorage(content.SHA256, true); rbErr != nil {
			slog.Error("unable to roll back deletion claim", "sha256", content.SHA256, "error", rbErr)
		}
		return false
	}

	return true
}

// BackfillDeletedFiles soft-deletes files that belong to bins that were
// soft-deleted before files were soft-deleted together with their bin. It
// walks the file table in id ranges with a pause between batches, and spends
// at most backfillRunBudget per lurker run. Once a pass is complete, or the
// initial check finds nothing to do, it does nothing for the rest of the
// process lifetime.
func (l *Lurker) BackfillDeletedFiles() {
	if l.backfillDone {
		return
	}

	if !l.backfillStarted {
		pending, err := l.dao.File().HasActiveInDeletedBins()
		if err != nil {
			slog.Error("unable to check for files in deleted bins", "error", err)
			return
		}
		if !pending {
			l.backfillDone = true
			return
		}
		maxID, err := l.dao.File().MaxID()
		if err != nil {
			slog.Error("unable to get the highest file id", "error", err)
			return
		}
		l.backfillMax = maxID
		l.backfillStarted = true
		slog.Info("starting backfill of files in deleted bins", "max_file_id", maxID)
	}

	t0 := time.Now()
	for l.backfillNext <= l.backfillMax && time.Since(t0) < backfillRunBudget {
		from := l.backfillNext
		to := from + backfillBatchSize
		count, err := l.dao.File().MarkDeletedInDeletedBins(from, to)
		if err != nil {
			slog.Error("unable to backfill files in deleted bins", "from_id", from, "to_id", to, "error", err)
			return
		}
		l.backfillCount += count
		l.backfillNext = to
		time.Sleep(l.backfillPause)
	}

	if l.backfillNext > l.backfillMax {
		l.backfillDone = true
		slog.Info("completed backfill of files in deleted bins", "count", l.backfillCount)
	} else {
		slog.Info("backfill of files in deleted bins in progress", "next_file_id", l.backfillNext, "max_file_id", l.backfillMax, "count", l.backfillCount)
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
