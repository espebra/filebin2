# Multi-Workspace Temporary File Storage

## Overview

This feature adds support for multiple temporary file storage workspaces with automatic performance-based selection. The system benchmarks each workspace at startup and intelligently selects the fastest workspace with sufficient capacity for each file upload.

## Benefits

- **Reduced IO wait**: Small files can use fast tmpfs-backed storage (e.g., `/dev/shm`)
- **Automatic failover**: Larger files or low-space scenarios automatically fall back to disk-based storage
- **Self-configuring**: No manual designation of "fast" vs "slow" - the system measures and adapts
- **Scalable**: Supports any number of workspaces

## Usage

### Single Workspace (Backward Compatible)
```bash
./filebin2 --tmpdir=/tmp
```

### Multiple Workspaces
```bash
./filebin2 --tmpdir=/dev/shm,/tmp,/var/tmp
```

### With Custom Capacity Threshold
```bash
# Default is 4x (requires 4x the file size available)
./filebin2 --tmpdir=/dev/shm,/tmp --tmpdir-capacity-threshold=4.0

# More conservative: 8x for extra safety margin
./filebin2 --tmpdir=/dev/shm,/tmp --tmpdir-capacity-threshold=8.0

# More aggressive: 2x for tighter space utilization
./filebin2 --tmpdir=/dev/shm,/tmp --tmpdir-capacity-threshold=2.0
```

### Example Output
```
Workspace "/dev/shm": 2500.00 MB/s write speed, 8.0 GB available of 16 GB total
Workspace "/tmp": 749.05 MB/s write speed, 88 GB available of 106 GB total
Workspace "/var/tmp": 450.20 MB/s write speed, 200 GB available of 500 GB total
```

## How It Works

### 1. Startup Benchmarking
Each workspace is benchmarked by:
- Writing 10MB of test data in 1MB chunks
- Syncing to ensure data is flushed to storage
- Calculating throughput in MB/s
- Checking available and total capacity

### 2. Workspace Sorting
Workspaces are sorted by write speed (fastest first) for optimal selection.

### 3. Selection Strategy
For each file upload, the system:
1. Tries to find the fastest workspace with ≥ threshold × file size available (default: 4x)
2. Falls back to workspaces with ≥ (threshold/2) × file size if needed
3. Uses the workspace with the most available space as a last resort
4. Periodically refreshes capacity (every 10 seconds)

The capacity threshold is configurable via `--tmpdir-capacity-threshold`:
- **4.0 (default)**: Requires 4× the file size available (conservative, recommended)
- **2.0**: Requires 2× the file size available (moderate)
- **8.0**: Requires 8× the file size available (very conservative)

### 4. File Creation
Once a workspace is selected, a temporary file is created in that workspace for the upload.

## Architecture

```
workspace/
├── workspace.go       # Main implementation
└── workspace_test.go  # Comprehensive tests (82.4% coverage)

main.go                # Initialization
http.go                # HTTP struct integration
http_file.go           # Upload handler integration
http_test.go           # Test setup
```

### Key Components

**Manager**: Manages multiple workspaces
- `NewManager(tmpdirFlag string)`: Creates manager from comma-separated paths
- `SelectWorkspace(fileSize uint64)`: Selects optimal workspace
- `CreateTempFile(fileSize uint64, prefix string)`: Creates temp file in selected workspace
- `GetStats()`: Returns statistics for all workspaces

**Workspace**: Represents a single storage location
- `Path`: Filesystem path
- `WriteMBps`: Measured write throughput
- `AvailableBytes`: Current available space
- `TotalBytes`: Total capacity
- `Benchmark()`: Measures write performance
- `UpdateCapacity()`: Refreshes space statistics

## Test Coverage

The workspace package includes 20 comprehensive tests covering:

✅ Single and multiple workspace initialization
✅ Workspace benchmarking accuracy
✅ Capacity detection and refresh
✅ Selection logic with various file sizes
✅ Preference for fastest workspace
✅ Fallback behavior when space is limited
✅ Thread-safety of concurrent operations
✅ Edge cases (no workspaces, invalid paths, huge files)
✅ Integration with temp file creation
✅ Configurable capacity thresholds (2x, 4x, 8x)
✅ Threshold validation and defaults
✅ Threshold impact on workspace selection

**Test Results:**
```
ok  	github.com/espebra/filebin2/workspace	1.180s	coverage: 82.4% of statements
```

## Configuration Examples

### Optimal for Small Files (< 8GB)
```bash
# tmpfs for small files, disk for large files (default 4x threshold)
--tmpdir=/dev/shm,/tmp

# More aggressive to maximize tmpfs usage (2x threshold)
--tmpdir=/dev/shm,/tmp --tmpdir-capacity-threshold=2.0
```

### Multi-Tier Storage
```bash
# Fast NVMe, slower SSD, bulk HDD
--tmpdir=/mnt/nvme/tmp,/mnt/ssd/tmp,/mnt/hdd/tmp

# With conservative threshold for production
--tmpdir=/mnt/nvme/tmp,/mnt/ssd/tmp,/mnt/hdd/tmp --tmpdir-capacity-threshold=8.0
```

### Development
```bash
# Single directory (existing behavior)
--tmpdir=/tmp

# Or with custom threshold
--tmpdir=/tmp --tmpdir-capacity-threshold=3.0
```

## Monitoring

The system logs workspace information at startup:
```
Workspace capacity threshold: 4.0x file size
Workspace "/dev/shm": 2500.00 MB/s write speed, 8.0 GB available of 16 GB total
Workspace "/tmp": 749.05 MB/s write speed, 88 GB available of 106 GB total
```

And warns when space is limited:
```
Warning: Using workspace "/tmp" with limited space (45.2% of preferred buffer)
Warning: All workspaces are low on space. Using "/tmp" with 2.1 GB available for 1.5 GB file
```

## Performance Characteristics

- **Startup overhead**: ~10-20ms per workspace (one-time benchmark)
- **Selection overhead**: <1ms (capacity checked every 10 seconds)
- **Thread-safe**: All operations use appropriate locking
- **Memory usage**: Minimal (~1KB per workspace)

## Future Enhancements

Potential improvements:
- Configurable benchmark size and strategy
- Prometheus metrics for workspace utilization
- Admin dashboard showing workspace statistics
- Support for network-mounted storage with latency detection
- Per-workspace thresholds (different thresholds for different workspaces)
- Dynamic threshold adjustment based on historical usage patterns
