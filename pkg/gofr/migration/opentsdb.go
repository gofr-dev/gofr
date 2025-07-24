package migration

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gofr.dev/pkg/gofr/container"
)

type openTSDBDS struct {
	container.OpenTSDB
	filePath string
}

type openTSDBMigrator struct {
	filePath string
	migrator
	mu sync.Mutex
}

type tsdbMigrationRecord struct {
	Version   int64  `json:"version"`
	Method    string `json:"method"`
	StartTime string `json:"start_time"`
	Duration  int64  `json:"duration"`
}

const dirPerm = 0755

var errNilFileHandle = errors.New("failed to create migration file: received nil file handle")

// apply initializes openTSDBMigrator using the openTsdbDS.
func (ds openTSDBDS) apply(m migrator) migrator {
	return &openTSDBMigrator{ // Return pointer to avoid copying the mutex
		filePath: ds.filePath,
		migrator: m,
	}
}

// checkAndCreateMigrationTable ensures the JSON file exists (creates if not).
func (om *openTSDBMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	om.mu.Lock()

	defer om.mu.Unlock()

	dir := filepath.Dir(om.filePath)
	if dir != "." {
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			return fmt.Errorf("failed to create migration directory %q: %w", dir, err)
		}
	}

	_, statErr := os.Stat(om.filePath)
	if statErr == nil {
		// File already exists
		return nil
	}

	if !os.IsNotExist(statErr) {
		// Some other error accessing the file
		return fmt.Errorf("unexpected error stating migration file: %w", statErr)
	}

	f, err := os.Create(om.filePath)
	if err != nil {
		return fmt.Errorf("failed to create migration file: %w", err)
	}

	if f == nil {
		return errNilFileHandle
	}

	defer func() {
		if cerr := f.Close(); cerr != nil {
			c.Debugf("Error closing migration file: %v", cerr)
		}
	}()

	if _, err = f.WriteString("[]"); err != nil {
		return fmt.Errorf("failed to initialize migration file: %w", err)
	}

	c.Debugf("Created new migration file: %s", om.filePath)

	return nil
}

// getLasomigration reads JSON file to find the highest applied migration version.
func (om *openTSDBMigrator) getLastMigration(c *container.Container) int64 {
	om.mu.Lock()

	defer om.mu.Unlock()

	file, err := os.Open(om.filePath)
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
	baseMigration := om.migrator.getLastMigration(c)

	return max(lastMigration, baseMigration)
}

// beginTransaction delegates to base migrator.
func (om *openTSDBMigrator) beginTransaction(c *container.Container) transactionData {
	return om.migrator.beginTransaction(c)
}

// commitMigration records a new migration in a JSON file in a thread-safe manner.
// It prevents duplicates and delegates the actual migration logic to the embedded migrator.
func (om *openTSDBMigrator) commitMigration(c *container.Container, data transactionData) error {
	// Lock to ensure thread-safe write access
	om.mu.Lock()
	defer om.mu.Unlock()

	// Load existing migrations from file
	migrations, err := om.loadMigrations()
	if err != nil {
		return err
	}

	// Skip if migration already exists
	if migrationExists(migrations, data.MigrationNumber) {
		c.Debugf("Migration %v already exists in JSON file, skipping", data.MigrationNumber)
		return om.migrator.commitMigration(c, data)
	}

	// Add new migration entry
	newRecord := tsdbMigrationRecord{
		Version:   data.MigrationNumber,
		Method:    "UP",
		StartTime: data.StartTime.Format(time.RFC3339),
		Duration:  time.Since(data.StartTime).Milliseconds(),
	}
	migrations = append(migrations, newRecord)

	// Atomically write updated migration list to file
	if err := om.writeMigrationsAtomically(migrations); err != nil {
		return err
	}

	c.Debugf("Committed migration %v to JSON file", data.MigrationNumber)

	return om.migrator.commitMigration(c, data)
}

// loadMigrations loads all previously committed migration records from a JSON file.
// Returns an empty list if the file does not exist.
func (om *openTSDBMigrator) loadMigrations() ([]tsdbMigrationRecord, error) {
	var migrations []tsdbMigrationRecord

	file, err := os.Open(om.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File does not exist yet, return empty list
			return migrations, nil
		}

		return nil, fmt.Errorf("failed to open migration file: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&migrations); err != nil {
		return nil, fmt.Errorf("failed to decode existing migrations: %w", err)
	}

	return migrations, nil
}

// migrationExists checks if a given migration version already exists in the list.
func migrationExists(migrations []tsdbMigrationRecord, version int64) bool {
	for _, existing := range migrations {
		if existing.Version == version {
			return true
		}
	}

	return false
}

// writeMigrationsAtomically writes the migration list to disk using a temp file,
// ensuring that the operation is atomic and safe against partial writes.
func (om *openTSDBMigrator) writeMigrationsAtomically(migrations []tsdbMigrationRecord) error {
	tmpFilePath := om.filePath + ".tmp"

	tmpFile, err := os.Create(tmpFilePath)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}

	defer func() {
		tmpFile.Close()

		if err != nil {
			os.Remove(tmpFilePath) // Clean up temp file on failure
		}
	}()

	// Write JSON with indentation
	enc := json.NewEncoder(tmpFile)
	enc.SetIndent("", "  ")

	if err = enc.Encode(migrations); err != nil {
		return fmt.Errorf("failed to encode migrations to JSON: %w", err)
	}

	// Ensure data is flushed to disk
	if err = tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temporary file: %w", err)
	}

	// Replace original file with temp file
	if err = os.Rename(tmpFilePath, om.filePath); err != nil {
		return fmt.Errorf("failed to rename temporary file: %w", err)
	}

	return nil
}

// rollback logs the failure and handles cleanup.
func (om *openTSDBMigrator) rollback(c *container.Container, data transactionData) {
	// Clean up any temporary files
	tmpFilePath := om.filePath + ".tmp"
	if _, err := os.Stat(tmpFilePath); err == nil {
		os.Remove(tmpFilePath)
		c.Debugf("Cleaned up temporary migration file: %s", tmpFilePath)
	}

	// Delegate to base migrator
	om.migrator.rollback(c, data)
	c.Fatalf("Migration %v failed.", data.MigrationNumber)
}

// Additional helper methods for OpenTSDB migration management

// GetMigrationHistory returns all applied migrations from the JSON file.
func (om *openTSDBMigrator) GetMigrationHistory(_ *container.Container) ([]tsdbMigrationRecord, error) {
	om.mu.Lock()

	defer om.mu.Unlock()

	file, err := os.Open(om.filePath)
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

// ValidateMigrationFile checks if the migration file is valid JSON.
func (om *openTSDBMigrator) ValidateMigrationFile(c *container.Container) error {
	om.mu.Lock()

	defer om.mu.Unlock()

	file, err := os.Open(om.filePath)
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
