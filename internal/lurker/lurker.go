package lurker

import (
	"fmt"
	"time"

	"github.com/espebra/filebin2/internal/dbl"
	"github.com/espebra/filebin2/internal/s3"
)

type Lurker struct {
	dao       *dbl.DAO
	s3        *s3.S3AO
	interval  time.Duration
	throttle  time.Duration
	retention uint64
}

// New creates a new Lurker instance
func New(dao *dbl.DAO, s3ao *s3.S3AO) *Lurker {
	return &Lurker{
		dao: dao,
		s3:  s3ao,
	}
}

func (l *Lurker) Init(interval int, throttle int, retention uint64) {
	l.interval = time.Second * time.Duration(interval)
	l.throttle = time.Millisecond * time.Duration(throttle)
	l.retention = retention
}

func (l *Lurker) Run() {
	fmt.Printf("Starting Lurker process (interval: %s)\n", l.interval.String())
	ticker := time.NewTicker(l.interval)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Lurker recovered from panic: %v\n", r)
				// Restart the lurker after a brief delay
				time.Sleep(10 * time.Second)
				l.Run()
			}
		}()
		for range ticker.C {
			t0 := time.Now()
			l.DeletePendingFiles()
			l.DeletePendingBins()
			l.DeletePendingContent()
			l.CleanTransactions()
			l.CleanClients()
			fmt.Printf("Lurker completed run in %.3fs\n", time.Since(t0).Seconds())
		}
	}()
}

func (l *Lurker) DeletePendingFiles() {
	files, err := l.dao.File().GetPendingDelete()
	if err != nil {
		fmt.Printf("Unable to GetPendingDelete(): %s\n", err.Error())
		return
	}
	if len(files) > 0 {
		fmt.Printf("Found %d files pending removal (already marked deleted).\n", len(files))
		// Files are already marked as deleted (deleted_at is set)
		// Orphaned content will be detected by DeletePendingContent using COUNT(*)
	}
}

func (l *Lurker) DeletePendingBins() {
	bins, err := l.dao.Bin().GetPendingDelete()
	if err != nil {
		fmt.Printf("Unable to GetPendingDelete(): %s\n", err.Error())
		return
	}
	if len(bins) > 0 {
		fmt.Printf("Found %d bins pending removal.\n", len(bins))
		for _, bin := range bins {
			// Mark bin as deleted
			bin.DeletedAt.Scan(time.Now().UTC())
			// Bin deletion cascades to files (sets deleted_at)
			// Orphaned content will be detected by DeletePendingContent using COUNT(*)
			if err := l.dao.Bin().Update(&bin); err != nil {
				fmt.Printf("Unable to update bin %s: %s\n", bin.Id, err.Error())
				return
			}
			fmt.Printf("Marked bin %s as deleted\n", bin.Id)
		}
	}
}

func (l *Lurker) DeletePendingContent() {
	contents, err := l.dao.FileContent().GetPendingDelete()
	if err != nil {
		fmt.Printf("Unable to get pending content deletions: %s\n", err.Error())
		return
	}
	if len(contents) > 0 {
		fmt.Printf("Found %d content objects pending removal.\n", len(contents))
		for _, content := range contents {
			// Safety check: verify no files reference this content
			count, err := l.dao.File().CountBySHA256(content.SHA256)
			if err != nil {
				fmt.Printf("Unable to count files for SHA256 %s: %s\n", content.SHA256, err.Error())
				continue
			}
			if count > 0 {
				fmt.Printf("Skipping %s: still has %d file references\n", content.SHA256, count)
				continue
			}

			// Delete from S3
			if err := l.s3.RemoveObjectByHash(content.SHA256); err != nil {
				fmt.Printf("Failed to remove %s from S3: %s\n", content.SHA256, err.Error())
				return
			}

			// Mark as not in storage (or delete the record)
			content.InStorage = false
			if err := l.dao.FileContent().Update(&content); err != nil {
				fmt.Printf("Unable to update file_content for SHA256 %s: %s\n", content.SHA256, err.Error())
				return
			}
			//fmt.Printf("Removed orphaned content %s from S3\n", content.SHA256)

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
		fmt.Printf("Unable to Transactions().Cleanup(): %s\n", err.Error())
		return
	}
	if count > 0 {
		fmt.Printf("Removed %d log transactions.\n", count)
	}
}

func (l *Lurker) CleanClients() {
	count, err := l.dao.Client().Cleanup(l.retention)
	if err != nil {
		fmt.Printf("Unable to Client().Cleanup(): %s\n", err.Error())
		return
	}
	if count > 0 {
		fmt.Printf("Removed %d client entries.\n", count)
	}
}
