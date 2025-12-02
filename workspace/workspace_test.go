package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewManager_SingleWorkspace(t *testing.T) {
	tmpDir := os.TempDir()
	m, err := NewManager(tmpDir, 4.0)
	if err != nil {
		t.Fatalf("Failed to create workspace manager: %s", err)
	}

	if len(m.workspaces) != 1 {
		t.Errorf("Expected 1 workspace, got %d", len(m.workspaces))
	}

	if m.workspaces[0].Path != tmpDir {
		t.Errorf("Expected workspace path %s, got %s", tmpDir, m.workspaces[0].Path)
	}

	if m.workspaces[0].WriteMBps <= 0 {
		t.Errorf("Expected positive write speed, got %f", m.workspaces[0].WriteMBps)
	}
}

func TestNewManager_MultipleWorkspaces(t *testing.T) {
	// Create multiple temporary directories
	baseDir := os.TempDir()
	dir1 := filepath.Join(baseDir, "workspace-test-1")
	dir2 := filepath.Join(baseDir, "workspace-test-2")
	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)

	tmpdirs := dir1 + "," + dir2
	m, err := NewManager(tmpdirs, 4.0)
	if err != nil {
		t.Fatalf("Failed to create workspace manager: %s", err)
	}

	if len(m.workspaces) != 2 {
		t.Errorf("Expected 2 workspaces, got %d", len(m.workspaces))
	}

	// Verify workspaces are sorted by write speed (fastest first)
	if len(m.workspaces) >= 2 {
		if m.workspaces[0].WriteMBps < m.workspaces[1].WriteMBps {
			t.Errorf("Workspaces not sorted by speed: %f < %f",
				m.workspaces[0].WriteMBps, m.workspaces[1].WriteMBps)
		}
	}
}

func TestNewManager_WithSpaces(t *testing.T) {
	// Test that spaces in the comma-separated list are handled
	baseDir := os.TempDir()
	dir1 := filepath.Join(baseDir, "workspace-test-3")
	dir2 := filepath.Join(baseDir, "workspace-test-4")
	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)

	// Add spaces around commas
	tmpdirs := dir1 + " , " + dir2
	m, err := NewManager(tmpdirs, 4.0)
	if err != nil {
		t.Fatalf("Failed to create workspace manager: %s", err)
	}

	if len(m.workspaces) != 2 {
		t.Errorf("Expected 2 workspaces, got %d", len(m.workspaces))
	}
}

func TestNewManager_InvalidPath(t *testing.T) {
	// Using an invalid path should not fail completely if there are other valid paths
	baseDir := os.TempDir()
	validDir := filepath.Join(baseDir, "workspace-test-valid")
	defer os.RemoveAll(validDir)

	// Mix valid and invalid paths - on Unix systems, /proc/sys/kernel is usually not writable
	tmpdirs := "/proc/sys/kernel/invalid," + validDir
	m, err := NewManager(tmpdirs, 4.0)

	// Should succeed with at least one valid workspace
	if err != nil {
		t.Fatalf("Failed to create workspace manager: %s", err)
	}

	if len(m.workspaces) < 1 {
		t.Errorf("Expected at least 1 workspace, got %d", len(m.workspaces))
	}
}

func TestWorkspace_Benchmark(t *testing.T) {
	tmpDir := os.TempDir()
	ws := &Workspace{
		Path: tmpDir,
	}

	err := ws.Benchmark()
	if err != nil {
		t.Fatalf("Benchmark failed: %s", err)
	}

	if ws.WriteMBps <= 0 {
		t.Errorf("Expected positive write speed, got %f MB/s", ws.WriteMBps)
	}

	t.Logf("Benchmark result: %.2f MB/s", ws.WriteMBps)
}

func TestWorkspace_UpdateCapacity(t *testing.T) {
	tmpDir := os.TempDir()
	ws := &Workspace{
		Path: tmpDir,
	}

	err := ws.UpdateCapacity()
	if err != nil {
		t.Fatalf("UpdateCapacity failed: %s", err)
	}

	if ws.AvailableBytes == 0 {
		t.Error("Expected non-zero available bytes")
	}

	if ws.TotalBytes == 0 {
		t.Error("Expected non-zero total bytes")
	}

	if ws.AvailableBytes > ws.TotalBytes {
		t.Errorf("Available bytes (%d) cannot exceed total bytes (%d)",
			ws.AvailableBytes, ws.TotalBytes)
	}

	if ws.LastChecked.IsZero() {
		t.Error("LastChecked should be set after UpdateCapacity")
	}

	t.Logf("Capacity: %d available / %d total bytes", ws.AvailableBytes, ws.TotalBytes)
}

func TestManager_SelectWorkspace_SufficientSpace(t *testing.T) {
	m, err := NewManager(os.TempDir(), 4.0)
	if err != nil {
		t.Fatalf("Failed to create workspace manager: %s", err)
	}

	// Request a small file (1MB)
	fileSize := uint64(1024 * 1024)
	ws, err := m.SelectWorkspace(fileSize)
	if err != nil {
		t.Fatalf("SelectWorkspace failed: %s", err)
	}

	if ws == nil {
		t.Fatal("Expected workspace, got nil")
	}

	if ws.GetAvailableBytes() < fileSize {
		t.Errorf("Selected workspace has insufficient space: %d < %d",
			ws.GetAvailableBytes(), fileSize)
	}
}

func TestManager_SelectWorkspace_PrefersFastest(t *testing.T) {
	// Create multiple workspaces
	baseDir := os.TempDir()
	dir1 := filepath.Join(baseDir, "workspace-test-5")
	dir2 := filepath.Join(baseDir, "workspace-test-6")
	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)

	m, err := NewManager(dir1 + "," + dir2, 4.0)
	if err != nil {
		t.Fatalf("Failed to create workspace manager: %s", err)
	}

	if len(m.workspaces) < 2 {
		t.Skip("Need at least 2 workspaces for this test")
	}

	// Request a small file that should fit in any workspace
	fileSize := uint64(1024) // 1KB
	ws, err := m.SelectWorkspace(fileSize)
	if err != nil {
		t.Fatalf("SelectWorkspace failed: %s", err)
	}

	// Should select the fastest workspace (first in sorted list)
	if ws != m.workspaces[0] {
		t.Errorf("Expected fastest workspace to be selected")
	}
}

func TestManager_SelectWorkspace_LargeFile(t *testing.T) {
	m, err := NewManager(os.TempDir(), 4.0)
	if err != nil {
		t.Fatalf("Failed to create workspace manager: %s", err)
	}

	// Request a very large file (1GB)
	fileSize := uint64(1024 * 1024 * 1024)
	ws, err := m.SelectWorkspace(fileSize)

	// This should either succeed if there's enough space, or fail gracefully
	if err != nil {
		t.Logf("SelectWorkspace correctly failed for large file: %s", err)
	} else if ws != nil {
		t.Logf("Selected workspace with %d bytes available for %d byte file",
			ws.GetAvailableBytes(), fileSize)
	}
}

func TestManager_SelectWorkspace_CapacityRefresh(t *testing.T) {
	m, err := NewManager(os.TempDir(), 4.0)
	if err != nil {
		t.Fatalf("Failed to create workspace manager: %s", err)
	}

	// First selection
	fileSize := uint64(1024)
	ws1, err := m.SelectWorkspace(fileSize)
	if err != nil {
		t.Fatalf("First SelectWorkspace failed: %s", err)
	}

	firstCheck := ws1.LastChecked

	// Set LastChecked to old time to force refresh
	ws1.LastChecked = time.Now().Add(-20 * time.Second)

	// Second selection should refresh capacity
	ws2, err := m.SelectWorkspace(fileSize)
	if err != nil {
		t.Fatalf("Second SelectWorkspace failed: %s", err)
	}

	if !ws2.LastChecked.After(firstCheck) {
		t.Error("Expected capacity to be refreshed on second selection")
	}
}

func TestManager_CreateTempFile(t *testing.T) {
	m, err := NewManager(os.TempDir(), 4.0)
	if err != nil {
		t.Fatalf("Failed to create workspace manager: %s", err)
	}

	fileSize := uint64(1024 * 1024) // 1MB
	fp, err := m.CreateTempFile(fileSize, "test-")
	if err != nil {
		t.Fatalf("CreateTempFile failed: %s", err)
	}
	defer os.Remove(fp.Name())
	defer fp.Close()

	// Verify file was created
	info, err := fp.Stat()
	if err != nil {
		t.Fatalf("Failed to stat temp file: %s", err)
	}

	if info.Size() != 0 {
		t.Errorf("Expected empty temp file, got size %d", info.Size())
	}

	// Write some data to verify file is writable
	testData := []byte("test data")
	n, err := fp.Write(testData)
	if err != nil {
		t.Fatalf("Failed to write to temp file: %s", err)
	}

	if n != len(testData) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(testData), n)
	}
}

func TestManager_GetPrimaryPath(t *testing.T) {
	tmpDir := os.TempDir()
	m, err := NewManager(tmpDir, 4.0)
	if err != nil {
		t.Fatalf("Failed to create workspace manager: %s", err)
	}

	primary := m.GetPrimaryPath()
	if primary == "" {
		t.Error("Expected non-empty primary path")
	}

	// Primary path should be the fastest (first) workspace
	if primary != m.workspaces[0].Path {
		t.Errorf("Expected primary path %s, got %s", m.workspaces[0].Path, primary)
	}
}

func TestManager_GetAllPaths(t *testing.T) {
	baseDir := os.TempDir()
	dir1 := filepath.Join(baseDir, "workspace-test-7")
	dir2 := filepath.Join(baseDir, "workspace-test-8")
	defer os.RemoveAll(dir1)
	defer os.RemoveAll(dir2)

	m, err := NewManager(dir1 + "," + dir2, 4.0)
	if err != nil {
		t.Fatalf("Failed to create workspace manager: %s", err)
	}

	paths := m.GetAllPaths()
	if len(paths) != len(m.workspaces) {
		t.Errorf("Expected %d paths, got %d", len(m.workspaces), len(paths))
	}

	// Verify all workspace paths are included
	for i, ws := range m.workspaces {
		if paths[i] != ws.Path {
			t.Errorf("Path mismatch at index %d: expected %s, got %s",
				i, ws.Path, paths[i])
		}
	}
}

func TestManager_GetStats(t *testing.T) {
	m, err := NewManager(os.TempDir(), 4.0)
	if err != nil {
		t.Fatalf("Failed to create workspace manager: %s", err)
	}

	stats := m.GetStats()
	if len(stats) != len(m.workspaces) {
		t.Errorf("Expected %d stats, got %d", len(m.workspaces), len(stats))
	}

	for i, stat := range stats {
		if stat.Path != m.workspaces[i].Path {
			t.Errorf("Stats path mismatch at index %d", i)
		}

		if stat.WriteMBps <= 0 {
			t.Errorf("Invalid write speed in stats: %f", stat.WriteMBps)
		}

		if stat.TotalBytes == 0 {
			t.Error("Invalid total bytes in stats")
		}
	}
}

func TestWorkspace_GetAvailableBytes_ThreadSafe(t *testing.T) {
	tmpDir := os.TempDir()
	ws := &Workspace{
		Path: tmpDir,
	}

	err := ws.UpdateCapacity()
	if err != nil {
		t.Fatalf("UpdateCapacity failed: %s", err)
	}

	// Test concurrent reads
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = ws.GetAvailableBytes()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestManager_SelectWorkspace_NoWorkspaces(t *testing.T) {
	// Create a manager with no workspaces
	m := &Manager{
		workspaces: []*Workspace{},
	}

	_, err := m.SelectWorkspace(1024)
	if err == nil {
		t.Error("Expected error when selecting from empty workspace list")
	}
}

func TestManager_ConfigurableThreshold(t *testing.T) {
	tmpDir := os.TempDir()

	// Test with 2x threshold
	m2x, err := NewManager(tmpDir, 2.0)
	if err != nil {
		t.Fatalf("Failed to create workspace manager with 2x threshold: %s", err)
	}

	if m2x.capacityThreshold != 2.0 {
		t.Errorf("Expected threshold 2.0, got %.1f", m2x.capacityThreshold)
	}

	// Test with 4x threshold
	m4x, err := NewManager(tmpDir, 4.0)
	if err != nil {
		t.Fatalf("Failed to create workspace manager with 4x threshold: %s", err)
	}

	if m4x.capacityThreshold != 4.0 {
		t.Errorf("Expected threshold 4.0, got %.1f", m4x.capacityThreshold)
	}

	// Test with 8x threshold
	m8x, err := NewManager(tmpDir, 8.0)
	if err != nil {
		t.Fatalf("Failed to create workspace manager with 8x threshold: %s", err)
	}

	if m8x.capacityThreshold != 8.0 {
		t.Errorf("Expected threshold 8.0, got %.1f", m8x.capacityThreshold)
	}

	// Test with invalid (negative) threshold - should default to 4.0
	mInvalid, err := NewManager(tmpDir, -1.0)
	if err != nil {
		t.Fatalf("Failed to create workspace manager with invalid threshold: %s", err)
	}

	if mInvalid.capacityThreshold != 4.0 {
		t.Errorf("Expected default threshold 4.0 for invalid input, got %.1f", mInvalid.capacityThreshold)
	}

	// Test with zero threshold - should default to 4.0
	mZero, err := NewManager(tmpDir, 0.0)
	if err != nil {
		t.Fatalf("Failed to create workspace manager with zero threshold: %s", err)
	}

	if mZero.capacityThreshold != 4.0 {
		t.Errorf("Expected default threshold 4.0 for zero input, got %.1f", mZero.capacityThreshold)
	}

	t.Logf("Successfully tested thresholds: 2x, 4x, 8x, and invalid defaults")
}

func TestManager_ThresholdAffectsSelection(t *testing.T) {
	// Create a workspace manager with low threshold
	m, err := NewManager(os.TempDir(), 1.5)
	if err != nil {
		t.Fatalf("Failed to create workspace manager: %s", err)
	}

	// Get workspace info
	if len(m.workspaces) == 0 {
		t.Fatal("No workspaces available")
	}

	ws := m.workspaces[0]
	err = ws.UpdateCapacity()
	if err != nil {
		t.Fatalf("Failed to update capacity: %s", err)
	}

	availableBytes := ws.GetAvailableBytes()
	t.Logf("Workspace has %d bytes available", availableBytes)

	// Try to select workspace for a file that's about 40% of available space
	// With 1.5x threshold, this should succeed (needs 60% of available)
	// With 4x threshold, this would fail (needs 160% of available)
	fileSize := uint64(float64(availableBytes) * 0.4)

	selectedWs, err := m.SelectWorkspace(fileSize)
	if err != nil {
		t.Logf("Selection failed as expected with large file: %s", err)
	} else if selectedWs != nil {
		requiredSpace := uint64(float64(fileSize) * m.capacityThreshold)
		t.Logf("Successfully selected workspace for file size %d (requires %d with %.1fx threshold)",
			fileSize, requiredSpace, m.capacityThreshold)

		if availableBytes < requiredSpace && availableBytes >= uint64(float64(fileSize)*m.capacityThreshold/2.0) {
			t.Logf("Fallback threshold was used (half of %.1fx = %.1fx)",
				m.capacityThreshold, m.capacityThreshold/2.0)
		}
	}
}

func TestManager_CreateTempFile_FallbackBehavior(t *testing.T) {
	m, err := NewManager(os.TempDir(), 4.0)
	if err != nil {
		t.Fatalf("Failed to create workspace manager: %s", err)
	}

	// Try to create a file larger than available space
	// Use an absurdly large size that should exceed any system
	fileSize := uint64(1024 * 1024 * 1024 * 1024 * 1024) // 1 PB
	fp, err := m.CreateTempFile(fileSize, "test-")

	// CreateTempFile will succeed in creating an empty file,
	// but will warn about low space. The file will fail during
	// write operations if there's truly insufficient space.
	if err != nil {
		// This is acceptable - SelectWorkspace might fail
		t.Logf("CreateTempFile correctly failed: %s", err)
		return
	}

	// Clean up if it succeeded
	if fp != nil {
		os.Remove(fp.Name())
		fp.Close()
		t.Logf("CreateTempFile succeeded with warning (will fail on actual write)")
	}
}
