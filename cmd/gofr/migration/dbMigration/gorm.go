package dbmigration

import (
	"strconv"
	"time"

	"gorm.io/gorm"

	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/log"
)

type GORM struct {
	db  *gorm.DB
	txn *gorm.DB
}

type gofrMigration struct {
	App       string    `gorm:"primary_key"`
	Version   int64     `gorm:"primary_key;auto_increment:false"`
	StartTime time.Time `gorm:"autoCreateTime"`
	EndTime   time.Time `gorm:"default:NULL"`
	Method    string    `gorm:"primary_key"`
}

// NewGorm return an new GORM instance
func NewGorm(d *gorm.DB) *GORM {
	return &GORM{db: d}
}

// Run executes a migration
func (g *GORM) Run(m Migrator, app, name, methods string, logger log.Logger) error {
	if g.db == nil {
		return errors.DataStoreNotInitialized{DBName: datastore.SQLStore}
	}

	g.txn = g.db.Begin()

	err := g.preRun(app, methods, name)
	if err != nil {
		if g.txn != nil {
			g.rollBack()
		}

		return err
	}

	ds := &datastore.DataStore{ORM: g.db}

	if methods == UP {
		err = m.Up(ds, logger)
	} else {
		err = m.Down(ds, logger)
	}

	if err != nil {
		g.rollBack()
		return &errors.Response{Reason: "error encountered in running the migration", Detail: err}
	}

	err = g.postRun(app, methods, name)
	if err != nil {
		g.rollBack()
		return err
	}

	g.commit()

	return nil
}

func (g *GORM) preRun(app, method, name string) error {
	if !g.db.Migrator().HasTable(&gofrMigration{}) {
		err := g.db.Migrator().CreateTable(&gofrMigration{})
		if err != nil {
			return &errors.Response{Reason: "unable to create gofr_migrations table", Detail: err.Error()}
		}
	}

	if g.isDirty(app) {
		return &errors.Response{Reason: "dirty migration check failed"}
	}

	ver, _ := strconv.Atoi(name)

	err := g.txn.Create(&gofrMigration{App: app, Version: int64(ver), StartTime: time.Now(), Method: method}).Error
	if err != nil {
		return &errors.Response{Reason: "unable to insert values into  gofr_migrations table.", Detail: err.Error()}
	}

	return nil
}

func (g *GORM) isDirty(app string) bool {
	var val int64

	err := g.txn.Table("gofr_migrations").Where("app = ? AND end_time is null", app).Count(&val).Error
	if err != nil || val > 0 {
		return true
	}

	return false
}

func (g *GORM) postRun(app, method, name string) error {
	// finish the migration
	err := g.txn.Table("gofr_migrations").Where("app = ? AND version = ? AND method = ?", app, name, method).
		Update(`end_time`, time.Now()).Error

	return err
}

// LastRunVersion retrieves the last run migration version
func (g *GORM) LastRunVersion(app, method string) (lv int) {
	if g.db == nil {
		return -1
	}

	row := g.db.Table("gofr_migrations").Where("app = ? AND method = ?", app, method).
		Select("COALESCE(MAX(version),0) as version").Row()

	_ = row.Scan(&lv)

	return
}

// GetAllMigrations retrieves all migrations
func (g *GORM) GetAllMigrations(app string) (upMigration, downMigration []int) {
	if g.db == nil {
		return []int{-1}, nil
	}

	rows, err := g.db.Table("gofr_migrations").Where("app = ?", app).Select("version, method").Rows()
	if err != nil {
		return nil, nil
	}

	defer func() {
		_ = rows.Close()
		_ = rows.Err()
	}()

	for rows.Next() {
		var (
			i int
			v string
		)

		_ = rows.Scan(&i, &v)

		if v == UP {
			upMigration = append(upMigration, i)
		} else {
			downMigration = append(downMigration, i)
		}
	}

	return upMigration, downMigration
}

// FinishMigration completes the migration
func (g *GORM) FinishMigration() error {
	// this method is no longer needed since individual
	// migrations are committed instantly after completion
	return nil
}

func (g *GORM) rollBack() {
	g.txn.Rollback()
}

func (g *GORM) commit() {
	g.txn.Commit()
}
