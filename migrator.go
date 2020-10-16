// An utility to apply migrations from applications to database.

package apps

import (
	"github.com/pkg/errors"
	"gorm.io/gorm"
)


type GormMigration interface {
	GetID() string
	Up(db *gorm.DB) error
	Down(db *gorm.DB) error
}

type InlineGormMigration struct {
	ID     string
	OnUp   func(db *gorm.DB) error
	OnDown func(db *gorm.DB) error
}

func (i InlineGormMigration) GetID() string {
	return i.ID
}

func (i InlineGormMigration) Up(db *gorm.DB) error {
	return i.OnUp(db)
}

func (i InlineGormMigration) Down(db *gorm.DB) error {
	return i.OnDown(db)
}

type MigrationRegistry []GormMigration

// Database model
type Migration struct {
	ID          uint `gorm:"primarykey"`
	MigrationID string
	AppName     string
}

// Represents a migration status.
type MigrationStatus struct {
	App       Application
	Migration GormMigration
	Applied   bool
}

// Represents status of several migrations.
type MigrationsStatus []MigrationStatus

// Print status to stdout
func (s MigrationsStatus) Print() {
	// sort by app name
	sorted := map[string]MigrationsStatus{}

	for _, state := range s {
		if _, ok := sorted[state.App.Name()]; !ok {
			sorted[state.App.Name()] = MigrationsStatus{}
		}
		sorted[state.App.Name()] = append(sorted[state.App.Name()], state)
	}

	// print sorted status
	for appName, appStates := range sorted {
		println(appName + ":")

		for _, s := range appStates {
			if s.Applied {
				print("[x]")
			} else {
				print("[ ]")
			}
			println("  " + s.Migration.GetID())
		}
		println("")
	}
}

// automatically migrate the database with known migrations
func Migrate(db *gorm.DB, app Application) (MigrationsStatus, error) {
	var states MigrationsStatus
	_ = app.Migrations()
	logger.Debug("starting migration tool")

	if err := db.AutoMigrate(&Migration{}); err != nil {
		return states, errors.Wrap(err, "cannot migrate Migration model")
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		for _, m := range app.Migrations() {
			// check if migration is applied
			var dbMigration Migration
			result := db.Where(&Migration{MigrationID: m.GetID(), AppName: app.Name()}).First(&dbMigration)
			if IsDBError(result) {
				return errors.Wrap(result.Error, "cannot fetch migration")
			}

			// if migrations not applied
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				if state, err := applyMigration(db, app, m, true); err != nil {
					return errors.Wrap(err, "cannot apply migration: "+m.GetID())
				} else {
					states = append(states, state)
				}
			}
		}
		return nil
	})
	if err != nil {
		return states, errors.Wrap(err, "error in migration transaction")
	}

	return states, nil
}

// Migrate an app to a specified migration ID.
func MigrateTo(db *gorm.DB, app Application, migrationID string) (MigrationsStatus, error) {
	logger.WithField("migration", migrationID).Debug("starting migration tool")
	var states MigrationsStatus

	if err := db.AutoMigrate(&Migration{}); err != nil {
		return states, errors.Wrap(err, "cannot migrate Migration model")
	}

	var dbMigrations []Migration
	result := db.Where(&Migration{AppName: app.Name()}).Find(&dbMigrations)
	if IsDBError(result) {
		return states, errors.Wrap(result.Error, "cannot retrieve migrations")
	}

	// check exists
	exists := false
	for _, m := range app.Migrations() {
		if migrationID == m.GetID() {
			exists = true
			break
		}
	}
	if !exists && migrationID != "zero" {
		return states, errors.New("migration do not exists: " + migrationID)
	}

	// check if a migration is applied
	applied := func(fn string) bool {
		for _, m := range dbMigrations {
			if fn == m.MigrationID {
				return true
			}
		}
		return false
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		// if applied, unapply migration after
		if migrationID == "zero" || applied(migrationID) {
			logger.WithField("migration", migrationID).Debug("migration already applied")

			for i := len(app.Migrations()) - 1; i >= 0; i-- { // go in reverse order
				m := app.Migrations()[i]
				if m.GetID() == migrationID {
					break // we are good
				}

				if applied(m.GetID()) {
					logger.WithField("migration", m.GetID()).Debug("un-applying migration")
					if state, err := applyMigration(db, app, m, false); err != nil {
						return err
					} else {
						states = append(states, state)
					}
				}
			}
		} else {
			logger.WithField("migration", migrationID).Debug("migration is not applied")

			for _, m := range app.Migrations() {
				if !applied(m.GetID()) {
					logger.WithField("migration", m.GetID()).Debug("applying migration")
					if state, err := applyMigration(db, app, m, true); err != nil {
						return err
					} else {
						states = append(states, state)
					}
				}

				if m.GetID() == migrationID {
					break // we are good
				}
			}
		}

		return nil
	})
	if err != nil {
		return states, errors.Wrap(err, "error in migration transaction")
	}

	return states, nil
}

func applyMigration(db *gorm.DB, app Application, migration GormMigration, goUp bool) (MigrationStatus, error) {
	applied := false

	err := db.Transaction(func(tx *gorm.DB) error {
		// run migration
		if goUp {
			if err := migration.Up(db); err != nil {
				return err
			}
			// insert migration row in db
			result := db.Create(&Migration{MigrationID: migration.GetID(), AppName: app.Name()})
			applied = true
			return result.Error
		} else {
			if err := migration.Down(db); err != nil {
				return err
			}

			var dbMigration Migration
			result := db.Where(&Migration{MigrationID: migration.GetID(), AppName: app.Name()}).First(&dbMigration)
			if IsDBError(result) {
				return errors.Wrap(result.Error, "cannot fetch migration")
			}

			db.Delete(&dbMigration)
			if IsDBError(result) {
				return errors.Wrap(result.Error, "cannot delete migration record")
			}
		}
		return nil
	})
	if err != nil {
		return MigrationStatus{}, errors.Wrap(err, "cannot exec transaction")
	}

	return MigrationStatus{
		App:       app,
		Migration: migration,
		Applied:   applied,
	}, nil
}

// show migrations status
func ShowMigrationsForApp(db *gorm.DB, app Application) (MigrationsStatus, error) {
	var states MigrationsStatus

	if err := db.AutoMigrate(&Migration{}); err != nil {
		return states, errors.Wrap(err, "cannot migrate Migration model")
	}

	var dbMigrations []Migration
	result := db.Where(&Migration{AppName: app.Name()}).Find(&dbMigrations)
	if IsDBError(result) {
		return states, errors.Wrap(result.Error, "cannot retrieve migrations")
	}

	// check if a migration is applied
	applied := func(fn string) bool {
		for _, m := range dbMigrations {
			if fn == m.MigrationID {
				return true
			}
		}
		return false
	}

	for _, m := range app.Migrations() {
		states = append(states, MigrationStatus{
			App:       app,
			Migration: m,
			Applied:   applied(m.GetID()),
		})
	}

	return states, nil
}
