package web

import (
	"testing"
	"time"

	"github.com/espebra/filebin2/internal/ds"
	"github.com/prometheus/client_golang/prometheus"
)

func TestStorageBytesCache(t *testing.T) {
	dao, s3ao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tearDown(dao) }()

	c := ds.Config{}

	metricsRegistry := prometheus.NewRegistry()
	metrics := ds.NewMetrics("test", metricsRegistry)

	h := &HTTP{
		staticBox:       &staticBox,
		templateBox:     &templateBox,
		dao:             &dao,
		s3:              &s3ao,
		config:          &c,
		metrics:         metrics,
		metricsRegistry: metricsRegistry,
	}
	if err := h.Init(); err != nil {
		t.Fatalf("Failed to initialize HTTP handler: %v", err)
	}
	defer h.Stop()

	// After Init(), the cache should be populated with the initial value
	cachedValue := h.getCachedStorageBytes()

	// Get the actual value from the database
	actualValue := dao.Metrics().StorageBytesAllocated()

	// The cached value should match the actual value (since no time has passed)
	if cachedValue != actualValue {
		t.Errorf("Expected cached value %d to match actual value %d", cachedValue, actualValue)
	}
}

func TestStorageBytesCache_ThreadSafety(t *testing.T) {
	dao, s3ao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tearDown(dao) }()

	c := ds.Config{}

	metricsRegistry := prometheus.NewRegistry()
	metrics := ds.NewMetrics("test", metricsRegistry)

	h := &HTTP{
		staticBox:       &staticBox,
		templateBox:     &templateBox,
		dao:             &dao,
		s3:              &s3ao,
		config:          &c,
		metrics:         metrics,
		metricsRegistry: metricsRegistry,
	}
	if err := h.Init(); err != nil {
		t.Fatalf("Failed to initialize HTTP handler: %v", err)
	}
	defer h.Stop()

	// Test concurrent reads don't panic or race
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func() {
			_ = h.getCachedStorageBytes()
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 100; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent reads")
		}
	}
}

func TestStorageBytesCache_InitialValueAvailable(t *testing.T) {
	dao, s3ao, err := tearUp()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tearDown(dao) }()

	c := ds.Config{}

	metricsRegistry := prometheus.NewRegistry()
	metrics := ds.NewMetrics("test", metricsRegistry)

	h := &HTTP{
		staticBox:       &staticBox,
		templateBox:     &templateBox,
		dao:             &dao,
		s3:              &s3ao,
		config:          &c,
		metrics:         metrics,
		metricsRegistry: metricsRegistry,
	}

	// Before Init(), the cache should be zero
	if h.storageBytesCache != 0 {
		t.Errorf("Expected cache to be 0 before Init(), got %d", h.storageBytesCache)
	}

	if err := h.Init(); err != nil {
		t.Fatalf("Failed to initialize HTTP handler: %v", err)
	}
	defer h.Stop()

	// After Init(), getCachedStorageBytes should return immediately without blocking
	start := time.Now()
	_ = h.getCachedStorageBytes()
	elapsed := time.Since(start)

	// Should be nearly instant (less than 100ms) since value is cached
	if elapsed > 100*time.Millisecond {
		t.Errorf("getCachedStorageBytes took too long: %v", elapsed)
	}
}
