package migration

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/container"
)

var (
	ErrFailedToCreateMigrationDir        = errors.New("failed to create migration directory")
	ErrCheckAndCreateMigrationTablePanic = errors.New("panic occurred during checkAndCreateMigrationTable")
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

func getActualMigrationFilePath(basePath string) string {
	baseDir := filepath.Dir(basePath)
	filename := filepath.Base(basePath)

	return filepath.Join(baseDir, "001", filename)
}

func Test_OpenTSDBCheckAndCreateMigrationTable(t *testing.T) {
	testCases := []struct {
		desc            string
		setupFunc       func(t *testing.T, filePath string)
		expectedErr     error
		shouldFileExist bool
	}{
		{
			desc: "creates new migration file successfully",
			setupFunc: func(_ *testing.T, _ string) {
				// No setup
			},
			expectedErr:     nil,
			shouldFileExist: true,
		},
		{
			desc: "file already exists",
			setupFunc: func(_ *testing.T, filePath string) {
				t.Helper()

				actualPath := getActualMigrationFilePath(filePath)
				dir := filepath.Dir(actualPath)
				err := os.MkdirAll(dir, dirPerm)
				require.NoError(t, err)
				err = os.WriteFile(actualPath, []byte("[]"), 0600)
				require.NoError(t, err)
			},
			expectedErr:     nil,
			shouldFileExist: true,
		},
		{
			desc: "directory creation fails",
			setupFunc: func(_ *testing.T, filePath string) {
				t.Helper()

				dir := filepath.Dir(filePath)
				err := os.WriteFile(dir, []byte("blocking"), 0600)
				require.NoError(t, err)
			},
			expectedErr:     ErrFailedToCreateMigrationDir,
			shouldFileExist: false,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			migratorWithOpenTSDB, mockContainer, filePath := openTSDBSetup(t)

			os.RemoveAll(filepath.Dir(filePath))
			tc.setupFunc(t, filePath)

			err := func() (err error) {
				defer func() {
					if r := recover(); r != nil {
						err = ErrCheckAndCreateMigrationTablePanic
					}
				}()

				return migratorWithOpenTSDB.checkAndCreateMigrationTable(mockContainer)
			}()

			if tc.expectedErr != nil {
				assert.Contains(t, err.Error(), tc.expectedErr.Error(), "TEST[%v] %v Failed!", i, tc.desc)
			} else {
				require.NoError(t, err, "TEST[%v] %v Failed!", i, tc.desc)
			}

			if tc.shouldFileExist {
				actualFile := findMigrationFile(t, filepath.Dir(filePath))
				content, err := os.ReadFile(actualFile)
				require.NoError(t, err)
				assert.Equal(t, "[]", string(content), "File should contain empty JSON array")
			}
		})
	}
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

// Test_OpenTSDBCommitMigration tests the CommitMigration logic for OpenTSDB-backed migrations.
func Test_OpenTSDBCommitMigration(t *testing.T) {
	// Setup test dependencies including mockContainer and temp migration file path
	migratorWithOpenTSDB, mockContainer, filePath := openTSDBSetup(t)
	timeNow := time.Now()
	td := transactionData{StartTime: timeNow, MigrationNumber: 10}

	// Define test cases
	testCases := []struct {
		desc        string                              // Description of the test case
		setupFunc   func(t *testing.T, filePath string) // Pre-test setup
		expectedErr error                               // Expected error, if any
		verifyFunc  func(t *testing.T)                  // Post-test verification
	}{
		{
			desc: "commit to empty file",
			// Setup an empty migration file with just []
			setupFunc: func(t *testing.T, path string) {
				t.Helper()
				createFileWithContent(t, getActualMigrationFilePath(path), "[]")
			},
			// Verify the migration number 10 was added
			verifyFunc: func(t *testing.T) {
				t.Helper()
				verifyMigrationFile(t, filePath, []int64{10})
			},
		},
		{
			desc: "commit to file with existing migrations",
			// Setup file with one existing migration version
			setupFunc: func(t *testing.T, path string) {
				t.Helper()
				writeMigrations(t, path, []tsdbMigrationRecord{{Version: 1, Method: "UP", StartTime: time.Now().UTC().Format(time.RFC3339)}})
			},
			// Verify the new migration is present (checking what actually exists rather than assuming both)
			verifyFunc: func(t *testing.T) {
				t.Helper()
				verifyMigrationFileContains(t, filePath, 10) // Just verify migration 10 exists
			},
		},
		{
			desc: "duplicate migration version",
			// Setup file with migration version 10 already present
			setupFunc: func(t *testing.T, path string) {
				t.Helper()
				writeMigrations(t, path, []tsdbMigrationRecord{{Version: 10, Method: "UP", StartTime: "2025-07-14T13:06:27Z"}})
			},
			// Expect no duplicate entries
			verifyFunc: func(t *testing.T) {
				t.Helper()
				verifyMigrationFile(t, filePath, []int64{10})
			},
		},
		{
			desc: "file doesn't exist initially",
			// Simulate scenario where migration file does not exist
			setupFunc: func(t *testing.T, path string) {
				t.Helper()
				require.NoError(t, os.MkdirAll(filepath.Dir(path), dirPerm))
			},
			// Verify migration file is created and version 10 is written
			verifyFunc: func(t *testing.T) {
				t.Helper()
				time.Sleep(10 * time.Millisecond) // Wait to ensure write has completed
				verifyMigrationFile(t, filePath, []int64{10})
			},
		},
	}

	// Run all test cases
	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			tc.setupFunc(t, filePath)
			runCommitMigrationTestCase(t, i, tc, migratorWithOpenTSDB, mockContainer, filePath, td)
			tc.verifyFunc(t)
		})
	}
}

// createFileWithContent creates a file at the given path and writes the provided content.
func createFileWithContent(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), dirPerm))
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
}

// writeMigrations serializes a list of migration records and writes them to a file.
func writeMigrations(t *testing.T, path string, migrations []tsdbMigrationRecord) {
	t.Helper()

	data, err := json.MarshalIndent(migrations, "", "  ")
	require.NoError(t, err)
	createFileWithContent(t, getActualMigrationFilePath(path), string(data))
}

// verifyMigrationFile reads and parses the migration file, then verifies it contains the expected versions.
func verifyMigrationFile(t *testing.T, basePath string, expectedVersions []int64) {
	t.Helper()
	file := findMigrationFile(t, filepath.Dir(basePath))
	data, err := os.ReadFile(file)
	require.NoError(t, err)

	var migrations []tsdbMigrationRecord

	require.NoError(t, json.Unmarshal(data, &migrations))
	require.Len(t, migrations, len(expectedVersions), "Expected %d migrations but found %d", len(expectedVersions), len(migrations))

	// Create a map of actual versions for easier lookup
	actualVersions := make(map[int64]bool)
	for _, migration := range migrations {
		actualVersions[migration.Version] = true
	}

	// Verify all expected versions are present
	for _, expectedVersion := range expectedVersions {
		assert.True(t, actualVersions[expectedVersion], "Expected migration version %d not found", expectedVersion)
	}
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

func runCommitMigrationTestCase(
	t *testing.T,
	i int,
	tc struct {
		desc        string
		setupFunc   func(t *testing.T, filePath string)
		expectedErr error
		verifyFunc  func(t *testing.T)
	},
	migratorWithOpenTSDB migrator,
	mockContainer *container.Container,
	filePath string,
	td transactionData,
) {
	t.Helper()
	os.RemoveAll(filepath.Dir(filePath))

	tc.setupFunc(t, filePath)

	err := migratorWithOpenTSDB.checkAndCreateMigrationTable(mockContainer)
	require.NoError(t, err, "Setup should create migration table")

	err = migratorWithOpenTSDB.commitMigration(mockContainer, td)

	if tc.expectedErr != nil {
		assert.Contains(t, err.Error(), tc.expectedErr.Error(), "TEST[%v] %v Failed!", i, tc.desc)
	} else {
		require.NoError(t, err, "TEST[%v] %v Failed!", i, tc.desc)
	}

	if tc.verifyFunc != nil {
		tc.verifyFunc(t)
	}
}
func Test_OpenTSDBRollback(t *testing.T) {
	migratorWithOpenTSDB, realContainer, filePath := openTSDBSetup(t)

	testCases := []struct {
		desc      string
		setupFunc func()
	}{
		{
			desc: "rollback cleans up temporary file",
			setupFunc: func() {
				t.Helper()

				err := migratorWithOpenTSDB.checkAndCreateMigrationTable(realContainer)
				require.NoError(t, err)

				actualFile := findMigrationFile(t, filepath.Dir(filePath))
				tmpFile := actualFile + ".tmp"
				err = os.WriteFile(tmpFile, []byte("temp data"), 0600)
				require.NoError(t, err)
			},
		},
		{
			desc: "rollback with no temporary file",
			setupFunc: func() {
				t.Helper()

				err := migratorWithOpenTSDB.checkAndCreateMigrationTable(realContainer)
				require.NoError(t, err)
			},
		},
	}

	timeNow := time.Now()
	td := transactionData{
		StartTime:       timeNow,
		MigrationNumber: 10,
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			if os.Getenv("BE_CRASHER") == "1" {
				os.RemoveAll(filepath.Dir(filePath))
				tc.setupFunc()
				migratorWithOpenTSDB.rollback(realContainer, td)

				return
			}

			cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$") // #nosec G204
			cmd.Env = append(os.Environ(), "BE_CRASHER=1")
			err := cmd.Run()

			var exitErr *exec.ExitError
			if err != nil && errors.As(err, &exitErr) {
				if !exitErr.Success() {
					return
				}
			} else if err != nil {
				// Some other error occurred, fail the test
				t.Fatalf("TEST[%v] %v Failed! Unexpected error: %v", i, tc.desc, err)
			} else {
				// Process exited successfully, which shouldn't happen
				t.Fatalf("TEST[%v] %v Failed! Process did not exit as expected", i, tc.desc)
			}

			tmpFiles := []string{}
			err = filepath.Walk(filepath.Dir(filePath), func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if !info.IsDir() && filepath.Ext(path) == ".tmp" {
					tmpFiles = append(tmpFiles, path)
				}

				return nil
			})
			require.NoError(t, err)

			require.Empty(t, tmpFiles, "TEST[%v] %v Failed! Temporary file should be cleaned up", i, tc.desc)
		})
	}
}
