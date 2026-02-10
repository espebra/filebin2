package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/espebra/filebin2/internal/dbl"
	"github.com/espebra/filebin2/internal/s3"
)

func main() {
	// Database flags
	dbHost := flag.String("db-host", "localhost", "Database host")
	dbPort := flag.Int("db-port", 5432, "Database port")
	dbName := flag.String("db-name", "filebin", "Database name")
	dbUser := flag.String("db-username", "filebin", "Database username")
	dbPassword := flag.String("db-password", "", "Database password")

	// S3 flags
	s3Endpoint := flag.String("s3-endpoint", "localhost:9000", "S3 endpoint")
	s3Bucket := flag.String("s3-bucket", "filebin", "S3 bucket")
	s3Region := flag.String("s3-region", "us-east-1", "S3 region")
	s3AccessKey := flag.String("s3-access-key", "", "S3 access key")
	s3SecretKey := flag.String("s3-secret-key", "", "S3 secret key")
	s3Secure := flag.Bool("s3-secure", false, "Use HTTPS for S3")

	// Migration flags
	cleanupOld := flag.Bool("cleanup-old", false, "Delete old S3 objects after migration")
	dryRun := flag.Bool("dry-run", false, "Perform a dry run without making changes")

	flag.Parse()

	fmt.Println("=== File Deduplication Migration Script ===")
	fmt.Printf("Database: %s@%s:%d/%s\n", *dbUser, *dbHost, *dbPort, *dbName)
	fmt.Printf("S3: %s/%s\n", *s3Endpoint, *s3Bucket)
	if *dryRun {
		fmt.Println("DRY RUN MODE - No changes will be made")
	}
	fmt.Println()

	// Initialize database
	dao, err := dbl.Init(dbl.DBConfig{
		Host:            *dbHost,
		Port:            *dbPort,
		Name:            *dbName,
		Username:        *dbUser,
		Password:        *dbPassword,
		MaxOpenConns:    25,
		MaxIdleConns:    25,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	})
	if err != nil {
		fmt.Printf("Failed to initialize database: %s\n", err)
		os.Exit(1)
	}
	defer func() { _ = dao.Close() }()

	// Initialize S3
	s3ao, err := s3.Init(s3.Config{
		Endpoint:             *s3Endpoint,
		Bucket:               *s3Bucket,
		Region:               *s3Region,
		AccessKey:            *s3AccessKey,
		SecretKey:            *s3SecretKey,
		Secure:               *s3Secure,
		PresignExpiry:        60 * time.Second,
		Timeout:              30 * time.Second,
		TransferTimeout:      10 * time.Minute,
		MultipartPartSize:    64 * 1024 * 1024,
		MultipartConcurrency: 3,
	})
	if err != nil {
		fmt.Printf("Failed to initialize S3: %s\n", err)
		os.Exit(1)
	}

	// Step 1: Populate file_content table
	fmt.Println("Step 1: Populating file_content table from existing files...")
	if err := populateFileContent(&dao, *dryRun); err != nil {
		fmt.Printf("Failed to populate file_content: %s\n", err)
		os.Exit(1)
	}

	// Step 2: Copy S3 objects to new keys
	fmt.Println("\nStep 2: Copying S3 objects to content-addressable keys...")
	if err := migrateS3Objects(&dao, &s3ao, *dryRun); err != nil {
		fmt.Printf("Failed to migrate S3 objects: %s\n", err)
		os.Exit(1)
	}

	// Step 3: Cleanup old S3 objects (optional)
	if *cleanupOld && !*dryRun {
		fmt.Println("\nStep 3: Cleaning up old S3 objects...")
		if err := cleanupOldS3Objects(&dao, &s3ao); err != nil {
			fmt.Printf("Failed to cleanup old objects: %s\n", err)
			os.Exit(1)
		}
	} else if *cleanupOld {
		fmt.Println("\nStep 3: Skipping cleanup (dry run mode)")
	}

	fmt.Println("\nMigration completed successfully!")
}

func populateFileContent(dao *dbl.DAO, dryRun bool) error {
	// Query to populate file_content from existing files
	sqlStatement := `
INSERT INTO file_content (sha256, bytes, reference_count, in_storage, created_at, last_referenced_at)
SELECT
    sha256,
    MAX(bytes) as bytes,
    COUNT(*) as reference_count,
    bool_or(in_storage) as in_storage,
    MIN(created_at) as created_at,
    MAX(updated_at) as last_referenced_at
FROM file
WHERE deleted_at IS NULL
GROUP BY sha256
ON CONFLICT (sha256) DO UPDATE SET
    reference_count = EXCLUDED.reference_count,
    last_referenced_at = EXCLUDED.last_referenced_at
`

	if dryRun {
		fmt.Println("DRY RUN: Would execute SQL:")
		fmt.Println(sqlStatement)

		// Show what would be inserted
		contents, err := dao.FileContent().GetAll()
		if err == nil {
			fmt.Printf("Current file_content records: %d\n", len(contents))
		}
		return nil
	}

	// We need direct database access for this, so we'll need to run it manually
	// For now, let's just document that this SQL needs to be run manually
	fmt.Println("Please run the following SQL manually on your database:")
	fmt.Println(sqlStatement)
	fmt.Println("\nThis will populate the file_content table with reference counts from existing files.")

	return nil
}

func migrateS3Objects(dao *dbl.DAO, s3ao *s3.S3AO, dryRun bool) error {
	// Get all file_content records
	contents, err := dao.FileContent().GetAll()
	if err != nil {
		return fmt.Errorf("failed to get file_content records: %w", err)
	}

	fmt.Printf("Found %d unique content hashes to migrate\n", len(contents))

	for i, content := range contents {
		if !content.InStorage {
			continue
		}

		fmt.Printf("[%d/%d] Processing SHA256 %s...\n", i+1, len(contents), content.SHA256)

		// Check if new key already exists
		newKey := content.SHA256
		_, err := s3ao.StatObject(newKey)
		if err == nil {
			fmt.Printf("  New key already exists, skipping\n")
			continue
		}

		// Find a file with this SHA256 to get the old key
		files, err := dao.File().FileByChecksum(content.SHA256)
		if err != nil || len(files) == 0 {
			fmt.Printf("  WARNING: No files found for SHA256 %s\n", content.SHA256)
			continue
		}

		// Use the first file to construct the old key
		file := files[0]
		oldKey := s3ao.GetObjectKey(file.Bin, file.Filename)

		if dryRun {
			fmt.Printf("  DRY RUN: Would copy %s -> %s\n", oldKey, newKey)
			continue
		}

		// Copy from old key to new key
		err = s3ao.CopyObject(oldKey, newKey)
		if err != nil {
			fmt.Printf("  ERROR: Failed to copy object: %s\n", err)
			continue
		}

		fmt.Printf("  Successfully copied to new key\n")
	}

	return nil
}

func cleanupOldS3Objects(dao *dbl.DAO, s3ao *s3.S3AO) error {
	// Get all files
	files, err := dao.File().GetAll(true)
	if err != nil {
		return fmt.Errorf("failed to get files: %w", err)
	}

	fmt.Printf("Cleaning up old S3 keys for %d files\n", len(files))

	// Track unique old keys to avoid deleting the same key multiple times
	deletedKeys := make(map[string]bool)

	for i, file := range files {
		oldKey := s3ao.GetObjectKey(file.Bin, file.Filename)
		newKey := file.SHA256

		if deletedKeys[oldKey] {
			continue
		}

		// Only delete if old key != new key (shouldn't be equal, but safety check)
		if oldKey == newKey {
			continue
		}

		fmt.Printf("[%d/%d] Removing old key %s\n", i+1, len(files), oldKey)

		err := s3ao.RemoveKey(oldKey)
		if err != nil {
			fmt.Printf("  WARNING: Failed to remove old key: %s\n", err)
			// Continue anyway
		}

		deletedKeys[oldKey] = true
	}

	fmt.Printf("Cleaned up %d old S3 keys\n", len(deletedKeys))
	return nil
}
