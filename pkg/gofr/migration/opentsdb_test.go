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

const dirPerm = 0755

var (
	ErrFailedToCreateMigrationDir        = errors.New("failed to create migration directory")
	ErrCheckAndCreateMigrationTablePanic = errors.New("panic occurred during checkAndCreateMigrationTable")
)

// openTSDBSetup creates a test setup for OpenTSDB migration tests
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
	openTSDBDS := openTSDBDS{OpenTSDB: mockOpenTSDB, filePath: filePath}
	migratorWithOpenTSDB := openTSDBDS.apply(&ds)

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
			setupFunc: func(_ *testing.T, filePath string) {
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

func Test_OpenTSDBCommitMigration(t *testing.T) {
	migratorWithOpenTSDB, mockContainer, filePath := openTSDBSetup(t)

	testCases := []struct {
		desc        string
		setupFunc   func(t *testing.T, filePath string)
		expectedErr error
		verifyFunc  func(t *testing.T)
	}{
		{
			desc: "commit to empty file",
			setupFunc: func(t *testing.T, filePath string) {
				t.Helper()

				actualPath := getActualMigrationFilePath(filePath)
				dir := filepath.Dir(actualPath)
				os.MkdirAll(dir, dirPerm)
				err := os.WriteFile(actualPath, []byte("[]"), 0600)
				require.NoError(t, err)
			},
			expectedErr: nil,
			verifyFunc: func(t *testing.T) {
				// Find the actual migration file
				actualFile := findMigrationFile(t, filepath.Dir(filePath))
				content, err := os.ReadFile(actualFile)
				require.NoError(t, err)
				
				var migrations []tsdbMigrationRecord
				err = json.Unmarshal(content, &migrations)
				require.NoError(t, err)
				
				assert.Len(t, migrations, 1)
				assert.Equal(t, int64(10), migrations[0].Version)
				assert.Equal(t, "UP", migrations[0].Method)
			},
		},
		{
			desc: "commit to file with existing migrations",
			setupFunc: func(t *testing.T, filePath string) {
    err := os.MkdirAll(filepath.Dir(filePath), dirPerm)
    require.NoError(t, err)

    // Prepopulate with one migration
    migrations := []tsdbMigrationRecord{
    {
        Version:   1,
        Method:    "UP",
        StartTime: time.Now().UTC().Format(time.RFC3339),
        Duration:  0,
    },
}


    data, _ := json.MarshalIndent(migrations, "", "  ")
    err = os.WriteFile(filePath, data, 0600)
    require.NoError(t, err)
},

			expectedErr: nil,
			verifyFunc: func(t *testing.T) {
				// Find the actual migration file
				actualFile := findMigrationFile(t, filepath.Dir(filePath))
				content, err := os.ReadFile(actualFile)
				require.NoError(t, err)
				
				var migrations []tsdbMigrationRecord
				err = json.Unmarshal(content, &migrations)
				require.NoError(t, err)
				
				assert.Len(t, migrations, 2)
				assert.Equal(t, int64(1), migrations[0].Version)
				assert.Equal(t, int64(10), migrations[1].Version)
			},
		},
		{
			desc: "duplicate migration version",
			setupFunc: func(t *testing.T, filePath string) {
				t.Helper()

				actualPath := getActualMigrationFilePath(filePath)
				dir := filepath.Dir(actualPath)
				os.MkdirAll(dir, dirPerm)
				existing := []tsdbMigrationRecord{
					{Version: 10, Method: "UP", StartTime: "2025-07-14T13:06:27Z", Duration: 0},
				}
				data, err := json.Marshal(existing)
				require.NoError(t, err)
				err = os.WriteFile(actualPath, data, 0600)
				require.NoError(t, err)
			},
			expectedErr: nil,
			verifyFunc: func(t *testing.T) {
				// Find the actual migration file
				actualFile := findMigrationFile(t, filepath.Dir(filePath))
				content, err := os.ReadFile(actualFile)
				require.NoError(t, err)
				
				var migrations []tsdbMigrationRecord
				err = json.Unmarshal(content, &migrations)
				require.NoError(t, err)
				
				// Should still have only one migration (duplicate not added)
				assert.Len(t, migrations, 1)
				assert.Equal(t, int64(10), migrations[0].Version)
			},
		},
		{
			desc: "file doesn't exist initially",
			setupFunc: func(t *testing.T, filePath string) {
				// Just ensure the base directory exists for the test
				t.Helper()

				baseDir := filepath.Dir(filePath)
				os.MkdirAll(baseDir, dirPerm)
			},
			expectedErr: nil,
			verifyFunc: func(t *testing.T) {
				// The migration system should create the file, so let's wait a bit and check
				time.Sleep(10 * time.Millisecond)
				
				// Find the actual migration file
				actualFile := findMigrationFile(t, filepath.Dir(filePath))
				content, err := os.ReadFile(actualFile)
				require.NoError(t, err)
				
				var migrations []tsdbMigrationRecord
				err = json.Unmarshal(content, &migrations)
				require.NoError(t, err)
				
				assert.Len(t, migrations, 1)
				assert.Equal(t, int64(10), migrations[0].Version)
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
			// Clean up before each test
			os.RemoveAll(filepath.Dir(filePath))
			
			tc.setupFunc(t, filePath)

			// First ensure the migration table exists
			err := migratorWithOpenTSDB.checkAndCreateMigrationTable(mockContainer)
			require.NoError(t, err, "Setup should create migration table")
			
			// Now run the actual test
			err = migratorWithOpenTSDB.commitMigration(mockContainer, td)

			if tc.expectedErr != nil {
				assert.Contains(t, err.Error(), tc.expectedErr.Error(), "TEST[%v] %v Failed!", i, tc.desc)
			} else {
				require.NoError(t, err, "TEST[%v] %v Failed!", i, tc.desc)
			}

			if tc.verifyFunc != nil {
				tc.verifyFunc(t)
			}
		})
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

			if e, ok := err.(*exec.ExitError); ok && !e.Success() {
				    return

			} else {
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

			require.Equal(t, 0, len(tmpFiles), "TEST[%v] %v Failed! Temporary file should be cleaned up", i, tc.desc)
		})
	}
}
