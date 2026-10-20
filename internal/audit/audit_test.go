package audit

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// clock is a settable time source for tests.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = t
}

func readEvents(t *testing.T, path string) []Event {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Unable to open %s: %s", path, err)
	}
	defer func() { _ = f.Close() }()
	var r io.Reader = f
	if filepath.Ext(path) == gzSuffix {
		zr, err := gzip.NewReader(f)
		if err != nil {
			t.Fatalf("Unable to read gzip %s: %s", path, err)
		}
		r = zr
	}
	var events []Event
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		var e Event
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("Invalid JSON line in %s: %s: %q", path, err, scanner.Text())
		}
		events = append(events, e)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("Unable to read %s: %s", path, err)
	}
	return events
}

func TestLogWritesEvents(t *testing.T) {
	dir := t.TempDir()
	c := &clock{t: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)}
	l, err := newLogger(dir, c.now)
	if err != nil {
		t.Fatal(err)
	}

	l.Log(Event{Event: "file_upload", Actor: ActorClient, Bin: "mybin", Filename: "a.txt", SHA256: "abc", IP: "192.0.2.1"})
	l.Log(Event{Event: "bin_expired", Actor: ActorLurker, Bin: "mybin"})
	l.Close()

	events := readEvents(t, filepath.Join(dir, "audit-2026-10-20.jsonl"))
	if len(events) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(events))
	}
	if events[0].Event != "file_upload" || events[0].Bin != "mybin" || events[0].SHA256 != "abc" || events[0].IP != "192.0.2.1" {
		t.Errorf("Unexpected first event: %+v", events[0])
	}
	if !events[0].Time.Equal(c.now()) {
		t.Errorf("Expected the time to default to now, got %s", events[0].Time)
	}
	if events[1].Event != "bin_expired" || events[1].Actor != ActorLurker {
		t.Errorf("Unexpected second event: %+v", events[1])
	}
}

func TestRotationCompressesPreviousDay(t *testing.T) {
	dir := t.TempDir()
	c := &clock{t: time.Date(2026, 10, 20, 23, 59, 0, 0, time.UTC)}
	l, err := newLogger(dir, c.now)
	if err != nil {
		t.Fatal(err)
	}

	l.Log(Event{Event: "first"})
	// Wait for the first event to be written before moving the clock, so
	// that it lands in the file of the first day.
	waitFor(t, func() bool {
		_, err := os.Stat(filepath.Join(dir, "audit-2026-10-20.jsonl"))
		return err == nil
	})
	c.set(time.Date(2026, 10, 21, 0, 1, 0, 0, time.UTC))
	l.Log(Event{Event: "second"})
	l.Close()

	if _, err := os.Stat(filepath.Join(dir, "audit-2026-10-20.jsonl")); !os.IsNotExist(err) {
		t.Errorf("Expected the uncompressed file of the previous day to be removed, got %v", err)
	}
	previous := readEvents(t, filepath.Join(dir, "audit-2026-10-20.jsonl.gz"))
	if len(previous) != 1 || previous[0].Event != "first" {
		t.Errorf("Unexpected events in the previous day: %+v", previous)
	}
	current := readEvents(t, filepath.Join(dir, "audit-2026-10-21.jsonl"))
	if len(current) != 1 || current[0].Event != "second" {
		t.Errorf("Unexpected events in the current day: %+v", current)
	}
}

func TestStartupCompressesLeftoverFiles(t *testing.T) {
	dir := t.TempDir()
	leftover := filepath.Join(dir, "audit-2026-10-19.jsonl")
	if err := os.WriteFile(leftover, []byte(`{"event":"leftover"}`+"\n"), 0640); err != nil {
		t.Fatal(err)
	}
	partial := filepath.Join(dir, "audit-2026-10-18.jsonl.gz.tmp")
	if err := os.WriteFile(partial, []byte("partial"), 0640); err != nil {
		t.Fatal(err)
	}
	today := filepath.Join(dir, "audit-2026-10-20.jsonl")
	if err := os.WriteFile(today, []byte(`{"event":"today"}`+"\n"), 0640); err != nil {
		t.Fatal(err)
	}

	c := &clock{t: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)}
	l, err := newLogger(dir, c.now)
	if err != nil {
		t.Fatal(err)
	}
	l.Log(Event{Event: "appended"})
	l.Close()

	events := readEvents(t, leftover+gzSuffix)
	if len(events) != 1 || events[0].Event != "leftover" {
		t.Errorf("Unexpected events in the compressed leftover: %+v", events)
	}
	if _, err := os.Stat(leftover); !os.IsNotExist(err) {
		t.Errorf("Expected the leftover to be removed, got %v", err)
	}
	if _, err := os.Stat(partial); !os.IsNotExist(err) {
		t.Errorf("Expected the partial file to be removed, got %v", err)
	}

	// The file of the current day is appended to, not compressed
	events = readEvents(t, today)
	if len(events) != 2 || events[0].Event != "today" || events[1].Event != "appended" {
		t.Errorf("Unexpected events in the current day: %+v", events)
	}
}

func TestRemoveExpired(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		"audit-2026-10-01.jsonl.gz", // older than the retention
		"audit-2026-10-05.jsonl.gz", // older than the retention
		"audit-2026-10-05.jsonl",    // older than the retention, left uncompressed
		"audit-2026-10-06.jsonl.gz", // exactly at the retention, kept
		"audit-2026-10-19.jsonl.gz",
		"audit-2026-10-20.jsonl", // today
		"audit-2026-10-01.jsonl.gz.tmp",
		"audit-notaday.jsonl.gz",
		"other-2026-10-01.jsonl.gz",
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{}\n"), 0640); err != nil {
			t.Fatal(err)
		}
	}

	c := &clock{t: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)}
	l := &Logger{dir: dir, now: c.now}

	// A retention of 0 keeps everything
	removed, err := l.RemoveExpired(0)
	if err != nil || removed != 0 {
		t.Fatalf("Expected nothing removed with retention 0, got %d (%v)", removed, err)
	}

	removed, err = l.RemoveExpired(14)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 3 {
		t.Errorf("Expected 3 files removed, got %d", removed)
	}
	for _, name := range files {
		_, err := os.Stat(filepath.Join(dir, name))
		expired := name == "audit-2026-10-01.jsonl.gz" || name == "audit-2026-10-05.jsonl.gz" || name == "audit-2026-10-05.jsonl"
		if expired && !os.IsNotExist(err) {
			t.Errorf("Expected %s to be removed", name)
		}
		if !expired && err != nil {
			t.Errorf("Expected %s to be kept, got %v", name, err)
		}
	}

	var nilLogger *Logger
	if removed, err := nilLogger.RemoveExpired(14); removed != 0 || err != nil {
		t.Errorf("Expected a nil logger to remove nothing, got %d (%v)", removed, err)
	}
}

func TestNilLoggerAndLogAfterClose(t *testing.T) {
	var nilLogger *Logger
	nilLogger.Log(Event{Event: "ignored"})
	nilLogger.Close()

	l, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
	l.Log(Event{Event: "ignored"})
	l.Close()
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("Timed out waiting for condition")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
