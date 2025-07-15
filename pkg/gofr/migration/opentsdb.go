package migration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
	"gofr.dev/pkg/gofr/container"
)

type openTsdbDS struct {
	container.OpenTSDB
	filePath string
}

type openTsdbMigrator struct {
	filePath string
	migrator
	mu sync.Mutex 
}

// apply initializes openTsdbMigrator using the openTsdbDS.
func (ds openTsdbDS) apply(m migrator) migrator {
	return &openTsdbMigrator{ // Return pointer to avoid copying the mutex
		filePath: ds.filePath,
		migrator: m,
	}
}

type tsdbMigrationRecord struct {
	Version   int64  `json:"version"`
	Method    string `json:"method"`
	StartTime string `json:"start_time"`
	Duration  int64  `json:"duration"`
}

// checkAndCreateMigrationTable ensures the JSON file exists (creates if not).
func (jm *openTsdbMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	dir := filepath.Dir(jm.filePath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create migration directory %q: %w", dir, err)
		}
	}

	_, statErr := os.Stat(jm.filePath)
	if statErr == nil {
		// File already exists
		return nil
	}
	if !os.IsNotExist(statErr) {
		// Some other error accessing the file
		return fmt.Errorf("unexpected error stating migration file: %w", statErr)
	}

	f, err := os.Create(jm.filePath)
	if err != nil {
		return fmt.Errorf("failed to create migration file: %w", err)
	}
	if f == nil {
		return fmt.Errorf("failed to create migration file: received nil file handle")
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			c.Debugf("Error closing migration file: %v", cerr)
		}
	}()

	if _, err = f.Write([]byte("[]")); err != nil {
		return fmt.Errorf("failed to initialize migration file: %w", err)
	}

	c.Debugf("Created new migration file: %s", jm.filePath)
	return nil
}

// getLastMigration reads JSON file to find the highest applied migration version.
func (jm *openTsdbMigrator) getLastMigration(c *container.Container) int64 {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	file, err := os.Open(jm.filePath)
	if err != nil {
		c.Errorf("Failed to open migration file: %v", err)
		return 0
	}
	defer file.Close()

	var migrations []tsdbMigrationRecord
	if err = json.NewDecoder(file).Decode(&migrations); err != nil {
		c.Errorf("Failed to decode migration file: %v", err)
		return 0
	}

	var lastMigration int64
	for _, m := range migrations {
		if m.Version > lastMigration {
			lastMigration = m.Version
		}
	}

	c.Debugf("JSON migration file last migration: %v", lastMigration)

	// Get last migration from base migrator and return the maximum
	baseMigration := jm.migrator.getLastMigration(c)
	return max(lastMigration, baseMigration)
}

// beginTransaction delegates to base migrator.
func (jm *openTsdbMigrator) beginTransaction(c *container.Container) transactionData {
	return jm.migrator.beginTransaction(c)
}

// commitMigration appends the migration record to the JSON file.
func (jm *openTsdbMigrator) commitMigration(c *container.Container, data transactionData) error {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	// Load existing records
	var migrations []tsdbMigrationRecord
	
	// Read existing file content
	if file, err := os.Open(jm.filePath); err == nil {
		decoder := json.NewDecoder(file)
		if decodeErr := decoder.Decode(&migrations); decodeErr != nil {
			file.Close()
			return fmt.Errorf("failed to decode existing migrations: %w", decodeErr)
		}
		file.Close()
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to open migration file: %w", err)
	}

	// Check if migration already exists (prevent duplicates)
	for _, existing := range migrations {
		if existing.Version == data.MigrationNumber {
			c.Debugf("Migration %v already exists in JSON file, skipping", data.MigrationNumber)
			return jm.migrator.commitMigration(c, data)
		}
	}

	// Create new migration record
	newRecord := tsdbMigrationRecord{
		Version:   data.MigrationNumber,
		Method:    "UP",
		StartTime: data.StartTime.Format(time.RFC3339),
		Duration:  time.Since(data.StartTime).Milliseconds(),
	}
	migrations = append(migrations, newRecord)

	// Write to temporary file first for atomic operation
	tmpFilePath := jm.filePath + ".tmp"
	tmpFile, err := os.Create(tmpFilePath)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer func() {
		tmpFile.Close()
		// Clean up temp file if something goes wrong
		if err != nil {
			os.Remove(tmpFilePath)
		}
	}()

	enc := json.NewEncoder(tmpFile)
	enc.SetIndent("", "  ")
	if err = enc.Encode(migrations); err != nil {
		return fmt.Errorf("failed to encode migrations to JSON: %w", err)
	}

	if err = tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temporary file: %w", err)
	}

	tmpFile.Close()

	// Atomic rename
	if err = os.Rename(tmpFilePath, jm.filePath); err != nil {
		return fmt.Errorf("failed to rename temporary file: %w", err)
	}

	c.Debugf("Committed migration %v to JSON file", data.MigrationNumber)

	// Delegate to base migrator
	return jm.migrator.commitMigration(c, data)
}

// rollback logs the failure and handles cleanup
func (jm *openTsdbMigrator) rollback(c *container.Container, data transactionData) {
	// Clean up any temporary files
	tmpFilePath := jm.filePath + ".tmp"
	if _, err := os.Stat(tmpFilePath); err == nil {
		os.Remove(tmpFilePath)
		c.Debugf("Cleaned up temporary migration file: %s", tmpFilePath)
	}

	// Delegate to base migrator
	jm.migrator.rollback(c, data)
	c.Fatalf("Migration %v failed.", data.MigrationNumber)
}

// Additional helper methods for OpenTSDB migration management

// GetMigrationHistory returns all applied migrations from the JSON file
func (jm *openTsdbMigrator) GetMigrationHistory(c *container.Container) ([]tsdbMigrationRecord, error) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	file, err := os.Open(jm.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []tsdbMigrationRecord{}, nil
		}
		return nil, fmt.Errorf("failed to open migration file: %w", err)
	}
	defer file.Close()

	var migrations []tsdbMigrationRecord
	if err = json.NewDecoder(file).Decode(&migrations); err != nil {
		return nil, fmt.Errorf("failed to decode migration file: %w", err)
	}

	return migrations, nil
}

// ValidateMigrationFile checks if the migration file is valid JSON
func (jm *openTsdbMigrator) ValidateMigrationFile(c *container.Container) error {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	file, err := os.Open(jm.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet, that's ok
		}
		return fmt.Errorf("failed to open migration file: %w", err)
	}
	defer file.Close()

	var migrations []tsdbMigrationRecord
	if err = json.NewDecoder(file).Decode(&migrations); err != nil {
		return fmt.Errorf("migration file contains invalid JSON: %w", err)
	}

	c.Debugf("Migration file validation successful, contains %d migrations", len(migrations))
	return nil
}