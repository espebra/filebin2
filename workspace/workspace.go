package workspace

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"golang.org/x/sys/unix"
)

// Workspace represents a temporary file storage location with its performance characteristics
type Workspace struct {
	Path           string
	WriteMBps      float64 // Write throughput in MB/s
	LastChecked    time.Time
	AvailableBytes uint64
	TotalBytes     uint64
	mutex          sync.RWMutex
}

// Manager manages multiple workspaces and selects the best one for each upload
type Manager struct {
	workspaces        []*Workspace
	capacityThreshold float64 // Multiplier for required space (e.g., 4.0 = require 4x file size)
	mutex             sync.RWMutex
}

// NewManager creates a new workspace manager from a comma-separated list of directories
// capacityThreshold specifies the safety margin multiplier (e.g., 4.0 requires 4x file size available)
func NewManager(tmpdirFlag string, capacityThreshold float64) (*Manager, error) {
	paths := strings.Split(tmpdirFlag, ",")
	if len(paths) == 0 {
		return nil, fmt.Errorf("no tmpdir paths provided")
	}

	if capacityThreshold <= 0 {
		capacityThreshold = 4.0 // Default to 4x if invalid value provided
	}

	m := &Manager{
		workspaces:        make([]*Workspace, 0, len(paths)),
		capacityThreshold: capacityThreshold,
	}

	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}

		// Verify the directory exists and is writable
		if err := os.MkdirAll(path, 0755); err != nil {
			fmt.Printf("Warning: Unable to create/access tmpdir %q: %s\n", path, err.Error())
			continue
		}

		ws := &Workspace{
			Path: path,
		}

		// Benchmark the workspace
		if err := ws.Benchmark(); err != nil {
			fmt.Printf("Warning: Unable to benchmark tmpdir %q: %s\n", path, err.Error())
			continue
		}

		// Get initial capacity
		if err := ws.UpdateCapacity(); err != nil {
			fmt.Printf("Warning: Unable to check capacity for tmpdir %q: %s\n", path, err.Error())
			continue
		}

		m.workspaces = append(m.workspaces, ws)
		fmt.Printf("Workspace %q: %.2f MB/s write speed, %s available of %s total\n",
			ws.Path, ws.WriteMBps, humanize.Bytes(ws.AvailableBytes), humanize.Bytes(ws.TotalBytes))
	}

	if len(m.workspaces) == 0 {
		return nil, fmt.Errorf("no usable tmpdir paths found")
	}

	// Sort workspaces by write speed (fastest first)
	sort.Slice(m.workspaces, func(i, j int) bool {
		return m.workspaces[i].WriteMBps > m.workspaces[j].WriteMBps
	})

	return m, nil
}

// Benchmark tests the write performance of this workspace
func (ws *Workspace) Benchmark() error {
	// Create a temporary file for benchmarking
	tmpFile, err := ioutil.TempFile(ws.Path, "filebin-benchmark-")
	if err != nil {
		return fmt.Errorf("failed to create benchmark file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Write 10MB of data to measure performance
	benchmarkSize := 10 * 1024 * 1024 // 10MB
	data := make([]byte, 1024*1024)   // 1MB chunks

	start := time.Now()
	for written := 0; written < benchmarkSize; written += len(data) {
		if _, err := tmpFile.Write(data); err != nil {
			return fmt.Errorf("failed to write benchmark data: %w", err)
		}
	}

	// Ensure data is flushed to disk
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync benchmark file: %w", err)
	}

	elapsed := time.Since(start).Seconds()
	ws.WriteMBps = float64(benchmarkSize) / (1024 * 1024) / elapsed

	return nil
}

// UpdateCapacity checks the available disk space for this workspace
func (ws *Workspace) UpdateCapacity() error {
	ws.mutex.Lock()
	defer ws.mutex.Unlock()

	var stat unix.Statfs_t
	if err := unix.Statfs(ws.Path, &stat); err != nil {
		return fmt.Errorf("failed to stat filesystem: %w", err)
	}

	// Available blocks * block size
	ws.AvailableBytes = stat.Bavail * uint64(stat.Bsize)
	ws.TotalBytes = stat.Blocks * uint64(stat.Bsize)
	ws.LastChecked = time.Now()

	return nil
}

// GetAvailableBytes returns the currently known available space (thread-safe)
func (ws *Workspace) GetAvailableBytes() uint64 {
	ws.mutex.RLock()
	defer ws.mutex.RUnlock()
	return ws.AvailableBytes
}

// SelectWorkspace selects the best workspace for a file of the given size
// Strategy: Use the fastest workspace that has at least capacityThreshold x file size available
func (m *Manager) SelectWorkspace(fileSize uint64) (*Workspace, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if len(m.workspaces) == 0 {
		return nil, fmt.Errorf("no workspaces available")
	}

	// Minimum required space: capacityThreshold x file size for safety margin
	minRequired := uint64(float64(fileSize) * m.capacityThreshold)

	// Try to find a workspace with sufficient space, preferring faster ones
	for _, ws := range m.workspaces {
		// Update capacity if it hasn't been checked recently (within last 10 seconds)
		if time.Since(ws.LastChecked) > 10*time.Second {
			if err := ws.UpdateCapacity(); err != nil {
				fmt.Printf("Warning: Failed to update capacity for %q: %s\n", ws.Path, err.Error())
				continue
			}
		}

		available := ws.GetAvailableBytes()
		if available >= minRequired {
			return ws, nil
		}
	}

	// If no workspace has enough space for the full threshold, try to find one with at least half the threshold
	minRequiredFallback := uint64(float64(fileSize) * (m.capacityThreshold / 2.0))
	for _, ws := range m.workspaces {
		available := ws.GetAvailableBytes()
		if available >= minRequiredFallback {
			fmt.Printf("Warning: Using workspace %q with limited space (%.2f%% of preferred buffer)\n",
				ws.Path, float64(available)/float64(minRequired)*100)
			return ws, nil
		}
	}

	// If still no workspace found, return the one with the most space
	var best *Workspace
	var maxAvailable uint64
	for _, ws := range m.workspaces {
		available := ws.GetAvailableBytes()
		if available > maxAvailable {
			maxAvailable = available
			best = ws
		}
	}

	if best != nil {
		fmt.Printf("Warning: All workspaces are low on space. Using %q with %s available for %s file\n",
			best.Path, humanize.Bytes(maxAvailable), humanize.Bytes(fileSize))
		return best, nil
	}

	return nil, fmt.Errorf("no workspace has sufficient space for %s file", humanize.Bytes(fileSize))
}

// Stats returns statistics about all workspaces
type Stats struct {
	Path           string
	WriteMBps      float64
	AvailableBytes uint64
	TotalBytes     uint64
	LastChecked    time.Time
}

// GetStats returns statistics about all workspaces
func (m *Manager) GetStats() []Stats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	stats := make([]Stats, len(m.workspaces))
	for i, ws := range m.workspaces {
		ws.mutex.RLock()
		stats[i] = Stats{
			Path:           ws.Path,
			WriteMBps:      ws.WriteMBps,
			AvailableBytes: ws.AvailableBytes,
			TotalBytes:     ws.TotalBytes,
			LastChecked:    ws.LastChecked,
		}
		ws.mutex.RUnlock()
	}

	return stats
}

// CreateTempFile creates a temporary file in the selected workspace
func (m *Manager) CreateTempFile(fileSize uint64, prefix string) (*os.File, error) {
	ws, err := m.SelectWorkspace(fileSize)
	if err != nil {
		return nil, err
	}

	// Create the temp file in the selected workspace
	fp, err := ioutil.TempFile(ws.Path, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file in %q: %w", ws.Path, err)
	}

	return fp, nil
}

// GetPrimaryPath returns the fastest workspace path (used for non-upload temp files)
func (m *Manager) GetPrimaryPath() string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if len(m.workspaces) > 0 {
		return m.workspaces[0].Path
	}
	return ""
}

// GetAllPaths returns all workspace paths
func (m *Manager) GetAllPaths() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	paths := make([]string, len(m.workspaces))
	for i, ws := range m.workspaces {
		paths[i] = ws.Path
	}
	return paths
}
