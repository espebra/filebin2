// Package audit writes an append-only audit log of the transfers and
// lifecycle events of bins and files, intended for answering requests from
// law enforcement long after the content itself has been deleted.
//
// Events are written as JSON lines to one file per day (UTC) in the audit
// log directory. The file of the current day is plain text, so that it can
// be followed while it is written. Once a day is over, its file is gzipped in
// the background. Events are written asynchronously by a single goroutine,
// so logging never waits for the disk.
package audit

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	filePrefix = "audit-"
	fileSuffix = ".jsonl"
	gzSuffix   = ".gz"
	dayLayout  = "2006-01-02"

	// Number of events that can be queued for writing. At the expected
	// volume of a few events per second this absorbs long stalls of the
	// disk. Events are dropped, and the drop is logged, if it fills up.
	queueSize = 65536

	// Maximum number of queued events to write before flushing to disk.
	batchSize = 1024
)

// Event is one entry in the audit log. The client details are captured when
// the event happens, so that they stay correct after the client record in
// the database is updated or removed.
type Event struct {
	Time            time.Time `json:"time"`
	Completed       time.Time `json:"completed,omitempty"`
	DurationSeconds float64   `json:"duration_seconds,omitempty"`
	Event           string    `json:"event"`
	Actor           string    `json:"actor,omitempty"`
	RequestID       string    `json:"request_id,omitempty"`

	Method string `json:"method,omitempty"`
	Path   string `json:"path,omitempty"`
	Status int    `json:"status,omitempty"`
	// Outcome of the request: OutcomeComplete or OutcomeAborted. Set for
	// all requests, so that a missing value is never mistaken for either.
	Outcome   string `json:"outcome,omitempty"`
	ReqBytes  int64  `json:"req_bytes,omitempty"`
	RespBytes int64  `json:"resp_bytes,omitempty"`

	Bin      string `json:"bin,omitempty"`
	Filename string `json:"filename,omitempty"`
	MD5      string `json:"md5,omitempty"`
	SHA1     string `json:"sha1,omitempty"`
	SHA256   string `json:"sha256,omitempty"`
	// Perceptual hash, for images only. Similar images have hashes that
	// differ in few bits, which can find altered copies of an image.
	PHash string `json:"phash,omitempty"`

	IP        string `json:"ip,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
	Referer   string `json:"referer,omitempty"`

	// Request headers that can help identify the client. They are logged
	// as sent and can be set to anything by the client, apart from what
	// the proxy in front of filebin adds.
	XForwardedFor   string `json:"x_forwarded_for,omitempty"`
	Forwarded       string `json:"forwarded,omitempty"`
	Via             string `json:"via,omitempty"`
	AcceptLanguage  string `json:"accept_language,omitempty"`
	SecCHUA         string `json:"sec_ch_ua,omitempty"`
	SecCHUAPlatform string `json:"sec_ch_ua_platform,omitempty"`
	SecCHUAMobile   string `json:"sec_ch_ua_mobile,omitempty"`
	Origin          string `json:"origin,omitempty"`
	SecFetchSite    string `json:"sec_fetch_site,omitempty"`
	ContentType     string `json:"content_type,omitempty"`

	ASN             int    `json:"asn,omitempty"`
	ASNOrganization string `json:"asn_organization,omitempty"`
	Network         string `json:"network,omitempty"`
	Country         string `json:"country,omitempty"`
	City            string `json:"city,omitempty"`
	Continent       string `json:"continent,omitempty"`
}

// Outcomes of requests.
const (
	// The handler returned normally. The response may still be an error
	// response, see Status.
	OutcomeComplete = "complete"
	// The handler aborted the response midway, for instance when an
	// archive download failed or the client disconnected mid-stream.
	// Status and the byte counts are what was sent before the abort.
	OutcomeAborted = "aborted"
)

// Actors of events.
const (
	ActorClient = "client"
	ActorAdmin  = "admin"
	ActorLurker = "lurker"
)

// Logger writes events to the audit log. A nil *Logger is valid and
// discards all events, which is used when the audit log is disabled.
type Logger struct {
	dir   string
	queue chan Event
	now   func() time.Time

	// Set when the queue is closed, to make Log after Close a no-op
	// instead of a panic.
	closed   atomic.Bool
	closeMu  sync.RWMutex
	dropped  atomic.Uint64
	reported uint64

	writerDone  chan struct{}
	compressing sync.WaitGroup

	// Owned by the writer goroutine
	file    *os.File
	buf     *bufio.Writer
	enc     *json.Encoder
	fileDay string
}

// New creates the audit log directory if needed, compresses the files of
// previous days that were left uncompressed, for instance by a crash, and
// starts the writer.
func New(dir string) (*Logger, error) {
	return newLogger(dir, time.Now)
}

func newLogger(dir string, now func() time.Time) (*Logger, error) {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("create audit log directory %s: %w", dir, err)
	}
	l := &Logger{
		dir:        dir,
		queue:      make(chan Event, queueSize),
		now:        now,
		writerDone: make(chan struct{}),
	}
	if err := l.compressPreviousDays(); err != nil {
		return nil, err
	}
	go l.writer()
	return l, nil
}

// Log queues an event for writing. It never blocks. The time of the event
// is set to now if it is zero.
func (l *Logger) Log(e Event) {
	if l == nil {
		return
	}
	if e.Time.IsZero() {
		e.Time = l.now()
	}
	e.Time = e.Time.UTC()
	if !e.Completed.IsZero() {
		e.Completed = e.Completed.UTC()
	}

	l.closeMu.RLock()
	defer l.closeMu.RUnlock()
	if l.closed.Load() {
		return
	}
	select {
	case l.queue <- e:
	default:
		l.dropped.Add(1)
	}
}

// Close writes the queued events, closes the current file and waits for
// any compression in progress to complete.
func (l *Logger) Close() {
	if l == nil {
		return
	}
	l.closeMu.Lock()
	if !l.closed.Swap(true) {
		close(l.queue)
	}
	l.closeMu.Unlock()
	<-l.writerDone
	l.compressing.Wait()
}

// writer writes queued events in batches until the queue is closed.
func (l *Logger) writer() {
	defer close(l.writerDone)
	defer l.closeFile()
	for e := range l.queue {
		l.write(e)
		// Write whatever else is queued before flushing, so that a burst
		// of events costs one write to disk.
	batch:
		for i := 1; i < batchSize; i++ {
			select {
			case e, ok := <-l.queue:
				if !ok {
					break batch
				}
				l.write(e)
			default:
				break batch
			}
		}
		if l.buf != nil {
			if err := l.buf.Flush(); err != nil {
				slog.Error("unable to write to audit log", "file", l.file.Name(), "error", err)
			}
		}
		l.reportDropped()
	}
}

// write encodes one event to the file of the current day, rotating to a new
// file when the day has changed. The day is taken from when the event is
// written rather than when it happened, so events close to midnight may end
// up in the file of the next day.
func (l *Logger) write(e Event) {
	day := l.now().UTC().Format(dayLayout)
	if day != l.fileDay {
		l.rotate(day)
	}
	if l.enc == nil {
		return
	}
	if err := l.enc.Encode(e); err != nil {
		slog.Error("unable to write to audit log", "file", l.file.Name(), "error", err)
	}
}

// rotate closes the current file, compresses it in the background and opens
// the file for the given day.
func (l *Logger) rotate(day string) {
	previous := ""
	if l.file != nil {
		previous = l.file.Name()
	}
	l.closeFile()
	if previous != "" {
		l.compressInBackground(previous)
	}

	path := filepath.Join(l.dir, filePrefix+day+fileSuffix)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		// Leave fileDay unset so that the next event tries again.
		slog.Error("unable to open audit log", "file", path, "error", err)
		return
	}
	l.file = f
	l.buf = bufio.NewWriterSize(f, 64*1024)
	l.enc = json.NewEncoder(l.buf)
	l.enc.SetEscapeHTML(false)
	l.fileDay = day
}

func (l *Logger) closeFile() {
	if l.file == nil {
		return
	}
	if err := l.buf.Flush(); err != nil {
		slog.Error("unable to write to audit log", "file", l.file.Name(), "error", err)
	}
	if err := l.file.Sync(); err != nil {
		slog.Error("unable to sync audit log", "file", l.file.Name(), "error", err)
	}
	if err := l.file.Close(); err != nil {
		slog.Error("unable to close audit log", "file", l.file.Name(), "error", err)
	}
	l.file = nil
	l.buf = nil
	l.enc = nil
	l.fileDay = ""
}

func (l *Logger) reportDropped() {
	dropped := l.dropped.Load()
	if dropped > l.reported {
		slog.Error("audit log queue is full, events were dropped", "dropped", dropped-l.reported, "dropped_total", dropped)
		l.reported = dropped
	}
}

// compressPreviousDays compresses the uncompressed files of days before
// today, and removes partial compressed files left by an interrupted
// compression.
func (l *Logger) compressPreviousDays() error {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return fmt.Errorf("read audit log directory %s: %w", l.dir, err)
	}
	today := l.now().UTC().Format(dayLayout)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, filePrefix) {
			continue
		}
		path := filepath.Join(l.dir, name)
		if strings.HasSuffix(name, gzSuffix+".tmp") {
			if err := os.Remove(path); err != nil {
				slog.Warn("unable to remove partial audit log", "file", path, "error", err)
			}
			continue
		}
		if !strings.HasSuffix(name, fileSuffix) {
			continue
		}
		day, ok := fileDay(name)
		if !ok || day >= today {
			continue
		}
		l.compressInBackground(path)
	}
	return nil
}

// RemoveExpired removes the files of days that are more than retentionDays
// days before today, and returns the number of files removed. The file of
// the current day is never removed. A retention of 0 keeps all files.
func (l *Logger) RemoveExpired(retentionDays uint64) (removed int, err error) {
	if l == nil || retentionDays == 0 {
		return 0, nil
	}
	cutoff := l.now().UTC().AddDate(0, 0, -int(retentionDays)).Format(dayLayout)
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return 0, fmt.Errorf("read audit log directory %s: %w", l.dir, err)
	}
	for _, entry := range entries {
		day, ok := fileDay(entry.Name())
		if !ok || day >= cutoff {
			continue
		}
		path := filepath.Join(l.dir, entry.Name())
		if err := os.Remove(path); err != nil {
			slog.Error("unable to remove expired audit log", "file", path, "error", err)
			continue
		}
		slog.Info("removed expired audit log", "file", path, "retention_days", retentionDays)
		removed++
	}
	return removed, nil
}

// fileDay returns the day of an audit log file, compressed or not, in the
// 2006-01-02 layout. It returns false for files that are not audit logs,
// including partial compressed files.
func fileDay(name string) (string, bool) {
	if !strings.HasPrefix(name, filePrefix) {
		return "", false
	}
	day := strings.TrimPrefix(name, filePrefix)
	switch {
	case strings.HasSuffix(day, fileSuffix+gzSuffix):
		day = strings.TrimSuffix(day, fileSuffix+gzSuffix)
	case strings.HasSuffix(day, fileSuffix):
		day = strings.TrimSuffix(day, fileSuffix)
	default:
		return "", false
	}
	if _, err := time.Parse(dayLayout, day); err != nil {
		return "", false
	}
	return day, true
}

func (l *Logger) compressInBackground(path string) {
	l.compressing.Add(1)
	go func() {
		defer l.compressing.Done()
		t0 := time.Now()
		if err := compressFile(path); err != nil {
			slog.Error("unable to compress audit log", "file", path, "error", err)
			return
		}
		slog.Info("compressed audit log", "file", path+gzSuffix, "duration_seconds", time.Since(t0).Seconds())
	}()
}

// compressFile gzips path to path.gz and removes path. The compressed file is
// written under a temporary name and renamed when complete, so a crash never
// leaves a truncated file that looks complete.
func compressFile(path string) error {
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	tmp := path + gzSuffix + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0640)
	if err != nil {
		return err
	}
	zw := gzip.NewWriter(out)
	zw.Name = filepath.Base(path)
	if _, err := io.Copy(zw, in); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := zw.Close(); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path+gzSuffix); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Remove(path)
}
