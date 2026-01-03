package main

import (
	"fmt"
	//"net/http"
	//"os"
	//"path"
	//"strings"
	//"encoding/json"
	"github.com/espebra/filebin2/dbl"
	"github.com/espebra/filebin2/s3"
	"time"
)

type Lurker struct {
	dao       *dbl.DAO
	s3        *s3.S3AO
	interval  time.Duration
	retention uint64
}

func (l *Lurker) Init(interval int, retention uint64) (err error) {
	l.interval = time.Second * time.Duration(interval)
	l.retention = retention
	return nil
}

func (l *Lurker) Run() {
	fmt.Printf("Starting Lurker process (interval: %s)\n", l.interval.String())
	ticker := time.NewTicker(l.interval)
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				return
			case _ = <-ticker.C:
				t0 := time.Now()
				l.DeletePendingFiles()
				l.DeletePendingBins()
				l.DeletePendingContent()
				l.CleanTransactions()
				l.CleanClients()
				fmt.Printf("Lurker completed run in %.3fs\n", time.Since(t0).Seconds())
			}
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
		fmt.Printf("Found %d files pending removal.\n", len(files))
		for _, file := range files {
			// Decrement reference count for the file content
			newCount, err := l.dao.FileContent().DecrementRefCount(file.SHA256)
			if err != nil {
				fmt.Printf("Unable to decrement ref count for %s: %s\n", file.SHA256, err.Error())
			} else {
				fmt.Printf("Decremented ref count for %s to %d\n", file.SHA256, newCount)
			}
			// No need to update file record - in_storage tracking is now purely in file_content
		}
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
			//fmt.Printf(" > Bin %s\n", bin.Id)
			files, err := l.dao.File().GetByBin(bin.Id, true)
			if err != nil {
				fmt.Printf("Unable to GetByBin: %s\n", err.Error())
				return
			}
			for _, file := range files {
				// Decrement reference count for the file content
				newCount, err := l.dao.FileContent().DecrementRefCount(file.SHA256)
				if err != nil {
					fmt.Printf("Unable to decrement ref count for %s: %s\n", file.SHA256, err.Error())
				} else {
					fmt.Printf("Decremented ref count for %s to %d\n", file.SHA256, newCount)
				}
				fmt.Printf("Processed file %s from bin %s\n", file.Filename, bin.Id)
				// No need to update file record - in_storage tracking is now purely in file_content
			}
			if err := l.dao.Bin().Update(&bin); err != nil {
				fmt.Printf("Unable to update bin %s: %s\n", bin.Id, err.Error())
				return
			}
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
			fmt.Printf("Removed orphaned content %s from S3\n", content.SHA256)
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
