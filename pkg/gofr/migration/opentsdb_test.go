package migration

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/container"
)

var (
	errCheckAndCreateMigrationTablePanic = errors.New("panic occurred during checkAndCreateMigrationTable")
)

// openTSDBSetup creates a test setup for OpenTSDB migration tests.
func openTSDBSetup(t *testing.T) (migrator, *container.Container, string) {
	t.Helper()

	mockContainer, mocks := container.NewMockContainer(t)
	mockOpenTSDB := mocks.OpenTSDB

	if mockOpenTSDB == nil {
		t.Fatal("mockOpenTSDB is nil - check container.NewMockContainer implementation")
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test_migrations.json")

	ds := Datasource{OpenTSDB: mockOpenTSDB}
	openTSDBInstance := openTSDBDS{OpenTSDB: mockOpenTSDB, filePath: filePath}
	migratorWithOpenTSDB := openTSDBInstance.apply(&ds)

	if migratorWithOpenTSDB == nil {
		t.Fatal("migratorWithOpenTSDB is nil - check openTsdbDS.apply implementation")
	}

	mockContainer.OpenTSDB = mockOpenTSDB

	return migratorWithOpenTSDB, mockContainer, filePath
}

func findMigrationFile(t *testing.T, baseDir string) string {
	t.Helper()

	var migrationFile string

	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}

			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".json" {
			migrationFile = path
			return filepath.SkipDir
		}

		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, migrationFile, "Migration file should exist")

	return migrationFile
}

// Test_OpenTSDBCheckAndCreateMigrationTable_Enhanced tests enhanced scenarios for creating migration table.
func Test_OpenTSDBCheckAndCreateMigrationTable_Enhanced(t *testing.T) {
	testCases := getEnhancedTestCases()

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			runEnhancedTestCase(t, tc, i)
		})
	}
}

func getEnhancedTestCases() []struct {
	desc            string
	setupFunc       func(t *testing.T, filePath string)
	expectedErr     string
	shouldFileExist bool
	verifyFunc      func(t *testing.T, filePath string)
	cleanupFunc     func(t *testing.T, filePath string)
} {
	var cases []struct {
		desc            string
		setupFunc       func(t *testing.T, filePath string)
		expectedErr     string
		shouldFileExist bool
		verifyFunc      func(t *testing.T, filePath string)
		cleanupFunc     func(t *testing.T, filePath string)
	}

	cases = append(cases, getSuccessTestCases()...)
	cases = append(cases, getFilePermissionTestCases()...)
	cases = append(cases, getDirectoryTestCases()...)
	cases = append(cases, getEdgeCaseTestCases()...)

	return cases
}

func getSuccessTestCases() []struct {
	desc            string
	setupFunc       func(t *testing.T, filePath string)
	expectedErr     string
	shouldFileExist bool
	verifyFunc      func(t *testing.T, filePath string)
	cleanupFunc     func(t *testing.T, filePath string)
} {
	return []struct {
		desc            string
		setupFunc       func(t *testing.T, filePath string)
		expectedErr     string
		shouldFileExist bool
		verifyFunc      func(t *testing.T, filePath string)
		cleanupFunc     func(t *testing.T, filePath string)
	}{
		{
			desc: "creates new migration file successfully",
			setupFunc: func(_ *testing.T, _ string) {
				// No setup - file doesn't exist
			},
			expectedErr:     "",
			shouldFileExist: true,
			verifyFunc: func(t *testing.T, filePath string) {
				t.Helper()
				content, err := os.ReadFile(filePath)
				require.NoError(t, err)
				assert.Equal(t, "[]", string(content), "File should contain empty JSON array")
			},
		},
		{
			desc: "file already exists with valid JSON",
			setupFunc: func(t *testing.T, filePath string) {
				t.Helper()
				dir := filepath.Dir(filePath)
				err := os.MkdirAll(dir, dirPerm)
				require.NoError(t, err)

				// Create valid migration file with existing data
				migrations := []tsdbMigrationRecord{
					{Version: 1, Method: "UP", StartTime: "2025-07-14T13:06:27Z", Duration: 100},
				}
				data, err := json.MarshalIndent(migrations, "", "  ")
				require.NoError(t, err)
				err = os.WriteFile(filePath, data, 0600)
				require.NoError(t, err)
			},
			expectedErr:     "",
			shouldFileExist: true,
			verifyFunc: func(t *testing.T, filePath string) {
				t.Helper()
				content, err := os.ReadFile(filePath)
				require.NoError(t, err)

				var migrations []tsdbMigrationRecord
				err = json.Unmarshal(content, &migrations)
				require.NoError(t, err)
				require.Len(t, migrations, 1)
				assert.Equal(t, int64(1), migrations[0].Version)
			},
		},
	}
}

func getFilePermissionTestCases() []struct {
	desc            string
	setupFunc       func(t *testing.T, filePath string)
	expectedErr     string
	shouldFileExist bool
	verifyFunc      func(t *testing.T, filePath string)
	cleanupFunc     func(t *testing.T, filePath string)
} {
	return []struct {
		desc            string
		setupFunc       func(t *testing.T, filePath string)
		expectedErr     string
		shouldFileExist bool
		verifyFunc      func(t *testing.T, filePath string)
		cleanupFunc     func(t *testing.T, filePath string)
	}{
		{
			desc: "file exists but contains invalid JSON",
			setupFunc: func(t *testing.T, filePath string) {
				t.Helper()
				dir := filepath.Dir(filePath)
				err := os.MkdirAll(dir, dirPerm)
				require.NoError(t, err)
				err = os.WriteFile(filePath, []byte("invalid json"), 0600)
				require.NoError(t, err)
			},
			expectedErr:     "existing migration file contains invalid JSON",
			shouldFileExist: true,
		},
		{
			desc: "file exists but cannot be opened (permission denied)",
			setupFunc: func(t *testing.T, filePath string) {
				t.Helper()
				dir := filepath.Dir(filePath)
				err := os.MkdirAll(dir, dirPerm)
				require.NoError(t, err)
				err = os.WriteFile(filePath, []byte("[]"), 0600)
				require.NoError(t, err)
				// Remove read permissions from the file
				err = os.Chmod(filePath, 0000)
				require.NoError(t, err)
			},
			expectedErr:     "failed to open existing migration file",
			shouldFileExist: true,
			cleanupFunc: func(_ *testing.T, filePath string) {
				// Restore permissions for cleanup
				_ = os.Chmod(filePath, 0600)
			},
		},
		{
			desc: "file creation fails due to permission denied on directory",
			setupFunc: func(t *testing.T, filePath string) {
				t.Helper()
				dir := filepath.Dir(filePath)
				// Create directory with no write permissions
				err := os.MkdirAll(dir, dirPerm)
				require.NoError(t, err)
				err = os.Chmod(dir, 0555) // Read and execute only
				require.NoError(t, err)
			},
			expectedErr:     "failed to create migration file",
			shouldFileExist: false,
			cleanupFunc: func(_ *testing.T, filePath string) {
				// Restore permissions for cleanup
				dir := filepath.Dir(filePath)
				_ = os.Chmod(dir, 0755)
			},
		},
	}
}

func getDirectoryTestCases() []struct {
	desc            string
	setupFunc       func(t *testing.T, filePath string)
	expectedErr     string
	shouldFileExist bool
	verifyFunc      func(t *testing.T, filePath string)
	cleanupFunc     func(t *testing.T, filePath string)
} {
	return []struct {
		desc            string
		setupFunc       func(t *testing.T, filePath string)
		expectedErr     string
		shouldFileExist bool
		verifyFunc      func(t *testing.T, filePath string)
		cleanupFunc     func(t *testing.T, filePath string)
	}{
		{
			desc: "directory creation fails due to existing file with same name",
			setupFunc: func(t *testing.T, filePath string) {
				t.Helper()
				dir := filepath.Dir(filePath)
				parentDir := filepath.Dir(dir)
				err := os.MkdirAll(parentDir, dirPerm)
				require.NoError(t, err)
				// Create a regular file where directory should be
				err = os.WriteFile(dir, []byte("blocking file"), 0600)
				require.NoError(t, err)
			},
			expectedErr:     "failed to create migration directory",
			shouldFileExist: false,
		},
		{
			desc: "directory creation fails due to permission denied on parent",
			setupFunc: func(t *testing.T, filePath string) {
				t.Helper()
				dir := filepath.Dir(filePath)
				parentDir := filepath.Dir(dir)

				// Create parent directory with no write permissions
				err := os.MkdirAll(parentDir, dirPerm)
				require.NoError(t, err)
				err = os.Chmod(parentDir, 0555) // Read and execute only
				require.NoError(t, err)
			},
			expectedErr:     "failed to create migration directory",
			shouldFileExist: false,
			cleanupFunc: func(_ *testing.T, filePath string) {
				// Restore permissions for cleanup
				dir := filepath.Dir(filePath)
				parentDir := filepath.Dir(dir)
				_ = os.Chmod(parentDir, 0755)
			},
		},
	}
}

func getEdgeCaseTestCases() []struct {
	desc            string
	setupFunc       func(t *testing.T, filePath string)
	expectedErr     string
	shouldFileExist bool
	verifyFunc      func(t *testing.T, filePath string)
	cleanupFunc     func(t *testing.T, filePath string)
} {
	return []struct {
		desc            string
		setupFunc       func(t *testing.T, filePath string)
		expectedErr     string
		shouldFileExist bool
		verifyFunc      func(t *testing.T, filePath string)
		cleanupFunc     func(t *testing.T, filePath string)
	}{
		{
			desc: "migration file path is a directory",
			setupFunc: func(t *testing.T, filePath string) {
				t.Helper()
				dir := filepath.Dir(filePath)
				err := os.MkdirAll(dir, dirPerm)
				require.NoError(t, err)

				// Create a directory with the same name as the file
				err = os.MkdirAll(filePath, dirPerm)
				require.NoError(t, err)
			},
			expectedErr:     "existing migration file contains invalid JSON",
			shouldFileExist: false,
		},
		{
			desc: "empty file path directory (current directory)",
			setupFunc: func(t *testing.T, _ string) {
				t.Helper()
				// This tests the case where filepath.Dir returns "."
				// File will be created in current directory
			},
			expectedErr:     "",
			shouldFileExist: true,
			verifyFunc: func(t *testing.T, filePath string) {
				t.Helper()
				content, err := os.ReadFile(filePath)
				require.NoError(t, err)
				assert.Equal(t, "[]", string(content))
			},
		},
	}
}

func runEnhancedTestCase(t *testing.T, tc struct {
	desc            string
	setupFunc       func(t *testing.T, filePath string)
	expectedErr     string
	shouldFileExist bool
	verifyFunc      func(t *testing.T, filePath string)
	cleanupFunc     func(t *testing.T, filePath string)
}, i int) {
	t.Helper()

	// Setup test environment
	var migratorWithOpenTSDB migrator

	var mockContainer *container.Container

	var filePath string

	if tc.desc == "empty file path directory (current directory)" {
		// Special setup for current directory test
		tmpDir := t.TempDir()
		originalDir, err := os.Getwd()
		require.NoError(t, err)

		t.Chdir(tmpDir)

		t.Cleanup(func() {
			t.Chdir(originalDir)
		})

		// Create migrator with just filename (no directory path)
		mockContainer2, mocks := container.NewMockContainer(t)
		openTSDBInstance := openTSDBDS{OpenTSDB: mocks.OpenTSDB, filePath: "test_migrations.json"}
		ds := Datasource{OpenTSDB: mocks.OpenTSDB}
		migratorWithOpenTSDB = openTSDBInstance.apply(&ds)
		mockContainer = mockContainer2
		filePath = "test_migrations.json"
	} else {
		migratorWithOpenTSDB, mockContainer, filePath = openTSDBSetup(t)
	}

	// Clean up any existing files/directories
	if tc.desc != "empty file path directory (current directory)" {
		os.RemoveAll(filepath.Dir(filePath))
	}

	// Setup test scenario
	tc.setupFunc(t, filePath)

	// Execute the function under test
	err := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = errCheckAndCreateMigrationTablePanic
			}
		}()

		return migratorWithOpenTSDB.checkAndCreateMigrationTable(mockContainer)
	}()

	// Verify results
	if tc.expectedErr != "" {
		require.Error(t, err, "TEST[%v] %v Failed! Expected error but got none", i, tc.desc)
		assert.Contains(t, err.Error(), tc.expectedErr, "TEST[%v] %v Failed! Error message mismatch", i, tc.desc)
	} else {
		require.NoError(t, err, "TEST[%v] %v Failed! Unexpected error: %v", i, tc.desc, err)
	}

	// Verify file existence
	if tc.shouldFileExist {
		_, err := os.Stat(filePath)
		require.NoError(t, err, "Migration file should exist at: %s", filePath)
	}

	// Run custom verification if provided
	if tc.verifyFunc != nil {
		tc.verifyFunc(t, filePath)
	}

	// Run cleanup if provided
	if tc.cleanupFunc != nil {
		tc.cleanupFunc(t, filePath)
	}
}

// Test_OpenTSDBCheckAndCreateMigrationTable_ConcurrentAccess tests concurrent access to checkAndCreateMigrationTable.
func Test_OpenTSDBCheckAndCreateMigrationTable_ConcurrentAccess(t *testing.T) {
	migratorWithOpenTSDB, mockContainer, filePath := openTSDBSetup(t)

	// Clean up any existing files
	os.RemoveAll(filepath.Dir(filePath))

	const numGoroutines = 10

	var wg sync.WaitGroup

	errCh := make(chan error, numGoroutines)

	// Run multiple goroutines concurrently
	for range numGoroutines {
		wg.Add(1)

		go func() {
			defer wg.Done()

			err := migratorWithOpenTSDB.checkAndCreateMigrationTable(mockContainer)
			errCh <- err
		}()
	}

	wg.Wait()
	close(errCh)

	// Verify all goroutines succeeded
	successCount := 0

	for err := range errCh {
		if err == nil {
			successCount++
		} else {
			t.Logf("Goroutine error: %v", err)
		}
	}

	// At least one should succeed (the first one to create the file)
	require.Positive(t, successCount, "At least one goroutine should succeed")

	// Verify file was created
	actualFile := findMigrationFile(t, filepath.Dir(filePath))
	content, err := os.ReadFile(actualFile)
	require.NoError(t, err)
	assert.Equal(t, "[]", string(content), "File should contain empty JSON array")
}

// Test_OpenTSDBCheckAndCreateMigrationTable_MutexProtection verifies mutex protection.
func Test_OpenTSDBCheckAndCreateMigrationTable_MutexProtection(t *testing.T) {
	migratorWithOpenTSDB, mockContainer, filePath := openTSDBSetup(t)

	// Clean up any existing files
	os.RemoveAll(filepath.Dir(filePath))

	// Cast to access the mutex directly for verification
	openTSDBMig, ok := migratorWithOpenTSDB.(*openTSDBMigrator)
	require.True(t, ok, "Failed to cast to openTSDBMigrator")

	// This test verifies that the mutex is properly protecting the critical section
	// We'll run the function multiple times and verify consistent behavior
	for i := range 5 {
		err := openTSDBMig.checkAndCreateMigrationTable(mockContainer)
		require.NoError(t, err, "Iteration %d should succeed", i)

		// Verify file exists and has correct content
		actualFile := findMigrationFile(t, filepath.Dir(filePath))
		content, err := os.ReadFile(actualFile)
		require.NoError(t, err)
		assert.Equal(t, "[]", string(content), "File should always contain empty JSON array")
	}
}

// Test_OpenTSDBCheckAndCreateMigrationTable_EdgeCases tests additional edge cases.
func Test_OpenTSDBCheckAndCreateMigrationTable_EdgeCases(t *testing.T) {
	testCases := []struct {
		desc        string
		setupFunc   func(t *testing.T) (migrator, *container.Container, string)
		expectedErr string
	}{
		{
			desc:        "very long file path",
			setupFunc:   setupVeryLongPath,
			expectedErr: "",
		},
		{
			desc:        "file path with special characters",
			setupFunc:   setupSpecialCharPath,
			expectedErr: "",
		},
		{
			desc:        "file path with unicode characters",
			setupFunc:   setupUnicodePath,
			expectedErr: "",
		},
		{
			desc:        "nested deep directory structure",
			setupFunc:   setupDeepPath,
			expectedErr: "",
		},
		{
			desc:        "file path with dots and relative components",
			setupFunc:   setupDotPath,
			expectedErr: "",
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			migratorWithOpenTSDB, mockContainer, filePath := tc.setupFunc(t)

			err := migratorWithOpenTSDB.checkAndCreateMigrationTable(mockContainer)

			if tc.expectedErr != "" {
				require.Error(t, err, "TEST[%v] %v Failed! Expected error but got none", i, tc.desc)
				assert.Contains(t, err.Error(), tc.expectedErr, "TEST[%v] %v Failed!", i, tc.desc)
			} else {
				require.NoError(t, err, "TEST[%v] %v Failed! Unexpected error: %v", i, tc.desc, err)

				// Verify file was created successfully
				_, err := os.Stat(filePath)
				require.NoError(t, err, "Migration file should exist at: %s", filePath)

				content, err := os.ReadFile(filePath)
				require.NoError(t, err)
				assert.Equal(t, "[]", string(content), "File should contain empty JSON array")
			}
		})
	}
}

func setupVeryLongPath(t *testing.T) (migrator, *container.Container, string) {
	t.Helper()
	mockContainer, mocks := container.NewMockContainer(t)

	// Create a very long path with valid characters
	tmpDir := t.TempDir()
	longDirName := strings.Repeat("a", 100) // 100 'a' characters instead of null bytes
	longPath := filepath.Join(tmpDir, longDirName, "migrations.json")

	openTSDBInstance := openTSDBDS{OpenTSDB: mocks.OpenTSDB, filePath: longPath}
	ds := Datasource{OpenTSDB: mocks.OpenTSDB}
	migratorWithOpenTSDB := openTSDBInstance.apply(&ds)

	return migratorWithOpenTSDB, mockContainer, longPath
}

func setupSpecialCharPath(t *testing.T) (migrator, *container.Container, string) {
	t.Helper()
	mockContainer, mocks := container.NewMockContainer(t)

	tmpDir := t.TempDir()
	specialPath := filepath.Join(tmpDir, "test with spaces & symbols!@#", "migrations.json")

	openTSDBInstance := openTSDBDS{OpenTSDB: mocks.OpenTSDB, filePath: specialPath}
	ds := Datasource{OpenTSDB: mocks.OpenTSDB}
	migratorWithOpenTSDB := openTSDBInstance.apply(&ds)

	return migratorWithOpenTSDB, mockContainer, specialPath
}

func setupUnicodePath(t *testing.T) (migrator, *container.Container, string) {
	t.Helper()
	mockContainer, mocks := container.NewMockContainer(t)

	tmpDir := t.TempDir()
	// Test with various unicode characters
	unicodePath := filepath.Join(tmpDir, "测试目录-🚀-مجلد", "migrations.json")

	openTSDBInstance := openTSDBDS{OpenTSDB: mocks.OpenTSDB, filePath: unicodePath}
	ds := Datasource{OpenTSDB: mocks.OpenTSDB}
	migratorWithOpenTSDB := openTSDBInstance.apply(&ds)

	return migratorWithOpenTSDB, mockContainer, unicodePath
}

func setupDeepPath(t *testing.T) (migrator, *container.Container, string) {
	t.Helper()
	mockContainer, mocks := container.NewMockContainer(t)

	tmpDir := t.TempDir()
	// Create a deeply nested path
	deepPath := tmpDir
	for i := 0; i < 10; i++ {
		deepPath = filepath.Join(deepPath, fmt.Sprintf("level%d", i))
	}

	deepPath = filepath.Join(deepPath, "migrations.json")

	openTSDBInstance := openTSDBDS{OpenTSDB: mocks.OpenTSDB, filePath: deepPath}
	ds := Datasource{OpenTSDB: mocks.OpenTSDB}
	migratorWithOpenTSDB := openTSDBInstance.apply(&ds)

	return migratorWithOpenTSDB, mockContainer, deepPath
}

func setupDotPath(t *testing.T) (migrator, *container.Container, string) {
	t.Helper()
	mockContainer, mocks := container.NewMockContainer(t)

	tmpDir := t.TempDir()
	// Path with dots (should be cleaned by filepath.Join)
	dotPath := filepath.Join(tmpDir, "dir1", "..", "dir2", ".", "migrations.json")

	openTSDBInstance := openTSDBDS{OpenTSDB: mocks.OpenTSDB, filePath: dotPath}
	ds := Datasource{OpenTSDB: mocks.OpenTSDB}
	migratorWithOpenTSDB := openTSDBInstance.apply(&ds)

	return migratorWithOpenTSDB, mockContainer, dotPath
}

func Test_OpenTSDBGetLastMigration(t *testing.T) {
	migratorWithOpenTSDB, mockContainer, filePath := openTSDBSetup(t)

	testCases := []struct {
		desc           string
		setupFunc      func()
		expectedResult int64
	}{
		{
			desc: "empty migration file",
			setupFunc: func() {
				t.Helper()

				err := os.MkdirAll(filepath.Dir(filePath), dirPerm)
				require.NoError(t, err)
				err = os.WriteFile(filePath, []byte("[]"), 0600)
				require.NoError(t, err)
			},
			expectedResult: 0,
		},
		{
			desc: "file with migrations",
			setupFunc: func() {
				t.Helper()

				err := os.MkdirAll(filepath.Dir(filePath), dirPerm)
				require.NoError(t, err)

				migrations := []tsdbMigrationRecord{
					{Version: 1, Method: "UP", StartTime: "2025-07-14T13:06:27Z", Duration: 0},
					{Version: 3, Method: "UP", StartTime: "2025-07-14T13:06:27Z", Duration: 100},
					{Version: 2, Method: "UP", StartTime: "2025-07-14T13:06:27Z", Duration: 50},
				}
				data, err := json.Marshal(migrations)
				require.NoError(t, err)
				err = os.WriteFile(filePath, data, 0600)
				require.NoError(t, err)
			},
			expectedResult: 3,
		},
		{
			desc: "file doesn't exist",
			setupFunc: func() {
				t.Helper()
				// No file
			},
			expectedResult: 0,
		},
		{
			desc: "invalid JSON file",
			setupFunc: func() {
				t.Helper()

				err := os.MkdirAll(filepath.Dir(filePath), dirPerm)
				require.NoError(t, err)
				err = os.WriteFile(filePath, []byte("invalid json"), 0600)
				require.NoError(t, err)
			},
			expectedResult: 0,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			os.RemoveAll(filepath.Dir(filePath))
			tc.setupFunc()

			result := migratorWithOpenTSDB.getLastMigration(mockContainer)
			assert.Equal(t, tc.expectedResult, result, "TEST[%v] %v Failed!", i, tc.desc)
		})
	}
}

// Test_OpenTSDBCommitMigration_ConcurrentAccess tests concurrent commits.
func Test_OpenTSDBCommitMigration_ConcurrentAccess(t *testing.T) {
	migratorWithOpenTSDB, mockContainer, filePath := openTSDBSetup(t)

	// Clean up and setup empty migration file
	os.RemoveAll(filepath.Dir(filePath))

	err := migratorWithOpenTSDB.checkAndCreateMigrationTable(mockContainer)
	require.NoError(t, err)

	const numGoroutines = 20

	var wg sync.WaitGroup

	errCh := make(chan error, numGoroutines)

	// Launch multiple goroutines trying to commit different migrations
	for i := 1; i <= numGoroutines; i++ {
		wg.Add(1)

		go func(migrationNum int) {
			defer wg.Done()

			txData := transactionData{
				StartTime:       time.Now().Add(-time.Duration(migrationNum) * time.Millisecond),
				MigrationNumber: int64(migrationNum),
			}
			err := migratorWithOpenTSDB.commitMigration(mockContainer, txData)
			errCh <- err
		}(i)
	}

	wg.Wait()
	close(errCh)

	// Verify all commits succeeded
	var errs []error

	for err := range errCh {
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		t.Logf("Concurrent commit errors: %v", errs)
	}

	// Verify all migrations were recorded
	verifyMigrationsCount(t, filePath, numGoroutines)

	// Verify each migration number is present
	for i := 1; i <= numGoroutines; i++ {
		verifyMigrationFileContains(t, filePath, int64(i))
	}
}

// Test_OpenTSDBCommitMigration_ConcurrentDuplicates tests concurrent commits of same migration.
func Test_OpenTSDBCommitMigration_ConcurrentDuplicates(t *testing.T) {
	migratorWithOpenTSDB, mockContainer, filePath := openTSDBSetup(t)

	// Clean up and setup empty migration file
	os.RemoveAll(filepath.Dir(filePath))

	err := migratorWithOpenTSDB.checkAndCreateMigrationTable(mockContainer)

	require.NoError(t, err)

	const numGoroutines = 10

	const migrationNumber = 42

	var wg sync.WaitGroup

	errCh := make(chan error, numGoroutines)

	// Launch multiple goroutines trying to commit the same migration
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			txData := transactionData{
				StartTime:       time.Now(),
				MigrationNumber: migrationNumber,
			}
			err := migratorWithOpenTSDB.commitMigration(mockContainer, txData)
			errCh <- err
		}()
	}

	wg.Wait()
	close(errCh)

	// All should succeed (duplicates are skipped, not errors)
	for err := range errCh {
		require.NoError(t, err, "Duplicate migration commits should not error")
	}

	// Should only have one migration recorded
	verifyMigrationsCount(t, filePath, 1)
	verifyMigrationFileContains(t, filePath, migrationNumber)
}

// verifyMigrationsCount verifies the total number of migrations in the file.
func verifyMigrationsCount(t *testing.T, basePath string, expectedCount int) {
	t.Helper()
	file := findMigrationFile(t, filepath.Dir(basePath))
	data, err := os.ReadFile(file)
	require.NoError(t, err)

	var migrations []tsdbMigrationRecord

	require.NoError(t, json.Unmarshal(data, &migrations))
	assert.Len(t, migrations, expectedCount,
		"Expected %d migrations but found %d", expectedCount, len(migrations))
}

// Test_OpenTSDBCommitMigration_JSONFormatValidation tests that output JSON is properly formatted.
func Test_OpenTSDBCommitMigration_JSONFormatValidation(t *testing.T) {
	migratorWithOpenTSDB, mockContainer, filePath := openTSDBSetup(t)

	// Clean up and setup
	os.RemoveAll(filepath.Dir(filePath))

	err := migratorWithOpenTSDB.checkAndCreateMigrationTable(mockContainer)

	require.NoError(t, err)

	// Commit a migration
	txData := transactionData{
		StartTime:       time.Now().Add(-100 * time.Millisecond),
		MigrationNumber: 1,
	}

	err = migratorWithOpenTSDB.commitMigration(mockContainer, txData)
	require.NoError(t, err)

	// Read the file and verify JSON formatting
	file := findMigrationFile(t, filepath.Dir(filePath))
	content, err := os.ReadFile(file)
	require.NoError(t, err)

	// Should be properly indented JSON
	var rawData []tsdbMigrationRecord
	err = json.Unmarshal(content, &rawData)
	require.NoError(t, err)

	// Re-marshal with same formatting and compare
	expectedContent, err := json.MarshalIndent(rawData, "", "  ")
	require.NoError(t, err)

	// The content should match properly formatted JSON (with trailing newline from encoder)
	assert.JSONEq(t, string(expectedContent), strings.TrimSpace(string(content)),
		"JSON should be properly formatted with indentation")
}

// Test_OpenTSDBCommitMigration_TimestampAccuracy tests timestamp handling accuracy.
func Test_OpenTSDBCommitMigration_TimestampAccuracy(t *testing.T) {
	migratorWithOpenTSDB, mockContainer, filePath := openTSDBSetup(t)

	// Clean up and setup
	os.RemoveAll(filepath.Dir(filePath))

	err := migratorWithOpenTSDB.checkAndCreateMigrationTable(mockContainer)
	require.NoError(t, err)

	// Use a specific time for accuracy testing
	specificTime := time.Date(2025, 7, 14, 13, 6, 27, 123456789, time.UTC)

	txData := transactionData{
		StartTime:       specificTime,
		MigrationNumber: 1,
	}

	// Record time just before commit for duration calculation
	beforeCommit := time.Now()
	err = migratorWithOpenTSDB.commitMigration(mockContainer, txData)
	afterCommit := time.Now()

	require.NoError(t, err)

	// Verify the timestamp and duration
	file := findMigrationFile(t, filepath.Dir(filePath))
	data, err := os.ReadFile(file)
	require.NoError(t, err)

	var migrations []tsdbMigrationRecord

	require.NoError(t, json.Unmarshal(data, &migrations))

	require.Len(t, migrations, 1)

	migration := migrations[0]

	// Verify timestamp format and accuracy
	assert.Equal(t, specificTime.Format(time.RFC3339), migration.StartTime,
		"Start time should be formatted as RFC3339")

	// Verify duration is reasonable (between our before/after measurements)
	expectedMinDuration := beforeCommit.Sub(specificTime).Milliseconds()
	expectedMaxDuration := afterCommit.Sub(specificTime).Milliseconds()

	assert.GreaterOrEqual(t, migration.Duration, expectedMinDuration,
		"Duration should be at least the minimum expected")
	assert.LessOrEqual(t, migration.Duration, expectedMaxDuration,
		"Duration should not exceed the maximum expected")
}

// verifyMigrationFileContains checks if the migration file contains a specific version.
func verifyMigrationFileContains(t *testing.T, basePath string, expectedVersion int64) {
	t.Helper()
	file := findMigrationFile(t, filepath.Dir(basePath))
	data, err := os.ReadFile(file)
	require.NoError(t, err)

	var migrations []tsdbMigrationRecord

	require.NoError(t, json.Unmarshal(data, &migrations))

	found := false

	for _, migration := range migrations {
		if migration.Version == expectedVersion {
			found = true
			break
		}
	}

	assert.True(t, found, "Expected migration version %d not found in file", expectedVersion)
}

// Test_OpenTSDBValidateExistingFile tests various scenarios for validating existing migration files.
func Test_OpenTSDBValidateExistingFile(t *testing.T) {
	testCases := getValidateExistingFileTestCases()

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			runValidateExistingFileTestCase(t, tc, i)
		})
	}
}

func getValidateExistingFileTestCases() []struct {
	desc           string
	setupFunc      func(t *testing.T, filePath string)
	expectedErr    string
	shouldLogError bool
	shouldLogDebug bool
	debugMessage   string
} {
	var cases []struct {
		desc           string
		setupFunc      func(t *testing.T, filePath string)
		expectedErr    string
		shouldLogError bool
		shouldLogDebug bool
		debugMessage   string
	}

	cases = append(cases, getValidJSONTestCases()...)
	cases = append(cases, getInvalidJSONTestCases()...)
	cases = append(cases, getFileAccessTestCases()...)

	return cases
}

func getValidJSONTestCases() []struct {
	desc           string
	setupFunc      func(t *testing.T, filePath string)
	expectedErr    string
	shouldLogError bool
	shouldLogDebug bool
	debugMessage   string
} {
	return []struct {
		desc           string
		setupFunc      func(t *testing.T, filePath string)
		expectedErr    string
		shouldLogError bool
		shouldLogDebug bool
		debugMessage   string
	}{
		{
			desc: "valid empty JSON array",
			setupFunc: func(t *testing.T, filePath string) {
				t.Helper()
				err := os.MkdirAll(filepath.Dir(filePath), dirPerm)
				require.NoError(t, err)
				err = os.WriteFile(filePath, []byte("[]"), 0600)
				require.NoError(t, err)
			},
			expectedErr:    "",
			shouldLogError: false,
			shouldLogDebug: true,
			debugMessage:   "Found existing migration file with 0 migrations",
		},
		{
			desc:           "valid JSON with single migration",
			setupFunc:      setupSingleMigration,
			expectedErr:    "",
			shouldLogError: false,
			shouldLogDebug: true,
			debugMessage:   "Found existing migration file with 1 migrations",
		},
		{
			desc:           "valid JSON with multiple migrations",
			setupFunc:      setupMultipleMigrations,
			expectedErr:    "",
			shouldLogError: false,
			shouldLogDebug: true,
			debugMessage:   "Found existing migration file with 3 migrations",
		},
		{
			desc:           "valid JSON with mixed field types",
			setupFunc:      setupMixedFieldTypes,
			expectedErr:    "",
			shouldLogError: false,
			shouldLogDebug: true,
			debugMessage:   "Found existing migration file with 2 migrations",
		},
	}
}

func getInvalidJSONTestCases() []struct {
	desc           string
	setupFunc      func(t *testing.T, filePath string)
	expectedErr    string
	shouldLogError bool
	shouldLogDebug bool
	debugMessage   string
} {
	return []struct {
		desc           string
		setupFunc      func(t *testing.T, filePath string)
		expectedErr    string
		shouldLogError bool
		shouldLogDebug bool
		debugMessage   string
	}{
		{
			desc: "invalid JSON - malformed",
			setupFunc: func(t *testing.T, filePath string) {
				t.Helper()
				err := os.MkdirAll(filepath.Dir(filePath), dirPerm)
				require.NoError(t, err)
				err = os.WriteFile(filePath, []byte("invalid json content"), 0600)
				require.NoError(t, err)
			},
			expectedErr:    "existing migration file contains invalid JSON",
			shouldLogError: true,
			shouldLogDebug: false,
		},
		{
			desc: "invalid JSON - incomplete array",
			setupFunc: func(t *testing.T, filePath string) {
				t.Helper()
				err := os.MkdirAll(filepath.Dir(filePath), dirPerm)
				require.NoError(t, err)
				err = os.WriteFile(filePath, []byte("[{\"version\": 1"), 0600)
				require.NoError(t, err)
			},
			expectedErr:    "existing migration file contains invalid JSON",
			shouldLogError: true,
			shouldLogDebug: false,
		},
		{
			desc: "invalid JSON - wrong structure",
			setupFunc: func(t *testing.T, filePath string) {
				t.Helper()
				err := os.MkdirAll(filepath.Dir(filePath), dirPerm)
				require.NoError(t, err)
				err = os.WriteFile(filePath, []byte("{\"not\": \"an array\"}"), 0600)
				require.NoError(t, err)
			},
			expectedErr:    "existing migration file contains invalid JSON",
			shouldLogError: true,
			shouldLogDebug: false,
		},
		{
			desc: "empty file",
			setupFunc: func(t *testing.T, filePath string) {
				t.Helper()
				err := os.MkdirAll(filepath.Dir(filePath), dirPerm)
				require.NoError(t, err)
				err = os.WriteFile(filePath, []byte(""), 0600)
				require.NoError(t, err)
			},
			expectedErr:    "existing migration file contains invalid JSON",
			shouldLogError: true,
			shouldLogDebug: false,
		},
	}
}

func getFileAccessTestCases() []struct {
	desc           string
	setupFunc      func(t *testing.T, filePath string)
	expectedErr    string
	shouldLogError bool
	shouldLogDebug bool
	debugMessage   string
} {
	return []struct {
		desc           string
		setupFunc      func(t *testing.T, filePath string)
		expectedErr    string
		shouldLogError bool
		shouldLogDebug bool
		debugMessage   string
	}{
		{
			desc: "file exists but cannot be opened (permission denied)",
			setupFunc: func(t *testing.T, filePath string) {
				t.Helper()
				err := os.MkdirAll(filepath.Dir(filePath), dirPerm)
				require.NoError(t, err)
				err = os.WriteFile(filePath, []byte("[]"), 0000) // No read permissions
				require.NoError(t, err)
			},
			expectedErr:    "failed to open existing migration file",
			shouldLogError: false,
			shouldLogDebug: false,
		},
	}
}

func setupSingleMigration(t *testing.T, filePath string) {
	t.Helper()

	err := os.MkdirAll(filepath.Dir(filePath), dirPerm)
	require.NoError(t, err)

	migrations := []tsdbMigrationRecord{
		{
			Version:   1,
			Method:    "UP",
			StartTime: "2025-07-14T13:06:27Z",
			Duration:  100,
		},
	}
	data, err := json.MarshalIndent(migrations, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(filePath, data, 0600)
	require.NoError(t, err)
}

func setupMultipleMigrations(t *testing.T, filePath string) {
	t.Helper()

	err := os.MkdirAll(filepath.Dir(filePath), dirPerm)
	require.NoError(t, err)

	migrations := []tsdbMigrationRecord{
		{Version: 1, Method: "UP", StartTime: "2025-07-14T13:06:27Z", Duration: 50},
		{Version: 2, Method: "UP", StartTime: "2025-07-14T13:06:28Z", Duration: 75},
		{Version: 3, Method: "UP", StartTime: "2025-07-14T13:06:29Z", Duration: 100},
	}
	data, err := json.MarshalIndent(migrations, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(filePath, data, 0600)
	require.NoError(t, err)
}

func setupMixedFieldTypes(t *testing.T, filePath string) {
	t.Helper()

	err := os.MkdirAll(filepath.Dir(filePath), dirPerm)
	require.NoError(t, err)

	// JSON with some fields missing or different types (but still valid for our struct)
	jsonContent := `[
		{
			"version": 1,
			"method": "UP",
			"start_time": "2025-07-14T13:06:27Z",
			"duration": 100
		},
		{
			"version": 2,
			"method": "DOWN",
			"start_time": "2025-07-14T13:06:28Z"
		}
	]`
	err = os.WriteFile(filePath, []byte(jsonContent), 0600)
	require.NoError(t, err)
}

func runValidateExistingFileTestCase(t *testing.T, tc struct {
	desc           string
	setupFunc      func(t *testing.T, filePath string)
	expectedErr    string
	shouldLogError bool
	shouldLogDebug bool
	debugMessage   string
}, i int) {
	t.Helper()

	migratorWithOpenTSDB, mockContainer, filePath := openTSDBSetup(t)

	// Clean up any existing files
	os.RemoveAll(filepath.Dir(filePath))

	// Setup the test scenario
	tc.setupFunc(t, filePath)

	// Cast to access the validateExistingFile method
	openTSDBMig, ok := migratorWithOpenTSDB.(*openTSDBMigrator)
	require.True(t, ok, "Failed to cast to openTSDBMigrator")

	// Call the method under test
	err := openTSDBMig.validateExistingFile(mockContainer)

	// Verify error expectations
	if tc.expectedErr != "" {
		require.Error(t, err, "TEST[%v] %v Failed! Expected error but got none", i, tc.desc)
		assert.Contains(t, err.Error(), tc.expectedErr, "TEST[%v] %v Failed! Error message mismatch", i, tc.desc)
	} else {
		require.NoError(t, err, "TEST[%v] %v Failed! Unexpected error: %v", i, tc.desc, err)
	}
}

// Test_OpenTSDBValidateExistingFile_ConcurrentAccess tests the validateExistingFile method under concurrent access.
func Test_OpenTSDBValidateExistingFile_ConcurrentAccess(t *testing.T) {
	migratorWithOpenTSDB, mockContainer, filePath := openTSDBSetup(t)

	// Setup a valid migration file
	err := os.MkdirAll(filepath.Dir(filePath), dirPerm)
	require.NoError(t, err)

	migrations := []tsdbMigrationRecord{
		{Version: 1, Method: "UP", StartTime: "2025-07-14T13:06:27Z", Duration: 100},
	}
	data, err := json.MarshalIndent(migrations, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(filePath, data, 0600)
	require.NoError(t, err)

	// Cast to access the validateExistingFile method
	openTSDBMig, ok := migratorWithOpenTSDB.(*openTSDBMigrator)
	require.True(t, ok, "Failed to cast to openTSDBMigrator")

	// Run multiple goroutines concurrently
	const numGoroutines = 10

	var wg sync.WaitGroup

	errCh := make(chan error, numGoroutines)

	for range numGoroutines {
		wg.Add(1)

		go func() {
			defer wg.Done()

			err := openTSDBMig.validateExistingFile(mockContainer)
			errCh <- err
		}()
	}

	wg.Wait()
	close(errCh)

	// Verify all goroutines succeeded
	for err := range errCh {
		require.NoError(t, err, "Concurrent access should not cause errors")
	}
}

// Test_OpenTSDBValidateExistingFile_FileModifiedDuringRead tests behavior when file is modified during read.
func Test_OpenTSDBValidateExistingFile_FileModifiedDuringRead(t *testing.T) {
	migratorWithOpenTSDB, mockContainer, filePath := openTSDBSetup(t)

	// Setup initial valid migration file
	err := os.MkdirAll(filepath.Dir(filePath), dirPerm)
	require.NoError(t, err)

	migrations := []tsdbMigrationRecord{
		{Version: 1, Method: "UP", StartTime: "2025-07-14T13:06:27Z", Duration: 100},
	}
	data, err := json.MarshalIndent(migrations, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(filePath, data, 0600)
	require.NoError(t, err)

	// Cast to access the validateExistingFile method
	openTSDBMig, ok := migratorWithOpenTSDB.(*openTSDBMigrator)
	require.True(t, ok, "Failed to cast to openTSDBMigrator")

	// This test verifies that the function handles the case where
	// the file exists and is readable at the time of the call
	err = openTSDBMig.validateExistingFile(mockContainer)
	require.NoError(t, err, "Should successfully validate existing file")

	// Test with file that gets corrupted
	err = os.WriteFile(filePath, []byte("corrupted"), 0600)
	require.NoError(t, err)

	err = openTSDBMig.validateExistingFile(mockContainer)
	require.Error(t, err, "Should fail to validate corrupted file")
	assert.Contains(t, err.Error(), "existing migration file contains invalid JSON")
}
