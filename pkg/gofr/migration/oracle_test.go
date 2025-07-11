package migration

import (
    
    "database/sql"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "go.uber.org/mock/gomock"

    "gofr.dev/pkg/gofr/container"

	"bytes"
    "io"
    "os"
)

func oracleSetup(t *testing.T) (migrator, *MockOracle, *container.Container) {
    t.Helper()
    ctrl := gomock.NewController(t)
    mockContainer, _ := container.NewMockContainer(t)
    mockOracle := NewMockOracle(ctrl)
    ds := Datasource{Oracle: mockOracle}
    od := oracleDS{Oracle: mockOracle}
    mg := od.apply(&ds)
    mockContainer.Oracle = mockOracle
    return mg, mockOracle, mockContainer
}

func Test_OracleCheckAndCreateMigrationTable(t *testing.T) {
    mg, mockOracle, mockContainer := oracleSetup(t)
    testCases := []struct {
        desc string
        err  error
    }{
        {"no error", nil},
        {"connection failed", sql.ErrConnDone},
    }
    for i, tc := range testCases {
        mockOracle.EXPECT().Exec(gomock.Any(), CheckAndCreateOracleMigrationTable).Return(tc.err)
        err := mg.checkAndCreateMigrationTable(mockContainer)
        assert.Equal(t, tc.err, err, "TEST[%v]\n %v Failed! ", i, tc.desc)
    }
}

func Test_OracleGetLastMigration(t *testing.T) {
    mg, mockOracle, mockContainer := oracleSetup(t)
    testCases := []struct {
        desc string
        err  error
        resp int64
    }{
        {"no error", nil, 0},
        {"connection failed", sql.ErrConnDone, 0},
    }
    for i, tc := range testCases {
        mockOracle.EXPECT().Select(gomock.Any(), gomock.Any(), getLastOracleGoFrMigration).Return(tc.err)
        resp := mg.getLastMigration(mockContainer)
        assert.Equal(t, tc.resp, resp, "TEST[%v]\n %v Failed! ", i, tc.desc)
    }
}

func Test_OracleCommitMigration(t *testing.T) {
    mg, mockOracle, mockContainer := oracleSetup(t)
    timeNow := time.Now()
    td := transactionData{
        StartTime:      timeNow,
        MigrationNumber: 10,
    }
    testCases := []struct {
        desc string
        err  error
    }{
        {"no error", nil},
        {"connection failed", sql.ErrConnDone},
    }
    for i, tc := range testCases {
        mockOracle.EXPECT().Exec(gomock.Any(), insertOracleGoFrMigrationRow, td.MigrationNumber,
            "UP", td.StartTime, gomock.Any()).Return(tc.err)
        err := mg.commitMigration(mockContainer, td)
        assert.Equal(t, tc.err, err, "TEST[%v]\n %v Failed! ", i, tc.desc)
    }
}

func captureStdout(f func()) string {
    old := os.Stdout
    r, w, _ := os.Pipe()
    os.Stdout = w

    f()

    w.Close()
    var buf bytes.Buffer
    io.Copy(&buf, r)
    os.Stdout = old
    return buf.String()
}

func Test_OracleBeginTransaction(t *testing.T) {
    logs := captureStdout(func() {
        mg, _, mockContainer := oracleSetup(t)
        mg.beginTransaction(mockContainer)
    })
    assert.Contains(t, logs, "OracleDB Migrator begin successfully")
}

