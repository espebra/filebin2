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
	dao      *dbl.DAO
	s3       *s3.S3AO
	interval time.Duration
}

func (l *Lurker) Init(interval int) (err error) {
	l.interval = time.Second * time.Duration(interval)
	if err != nil {
		return err
	}
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
			//case t := <-ticker.C:
			case _ = <-ticker.C:
				t0 := time.Now()
				l.CleanUpExpiredBins()
				fmt.Printf("Lurker completed run in %.3fs\n", time.Since(t0).Seconds())
			}
		}
	}()
	//ticker.Stop()
	//done <- true
	//fmt.Println("Lurker process stopped")
}

func (l *Lurker) CleanUpExpiredBins() {
	bins, err := l.dao.Bin().GetBinsPendingExpiration()
	if err != nil {
		fmt.Printf("Unable to GetBinsPendingExpiration(): %s\n", err.Error())
		return
	}
	for _, bin := range bins {
		fmt.Printf("> Bin %s is expired\n", bin.Id)
	}
	fmt.Printf("Found %d expired bins.\n", len(bins))
}
