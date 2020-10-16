package apps

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"testing"
)

type User struct {
	ID        uint   `gorm:"primarykey"`
	Email     string `gorm:"uniqueIndex"`
	FirstName string
}

// An app to manage users
type UserApp struct{}

func (a *UserApp) Name() string {
	return "users"
}

func (a *UserApp) Migrations() []GormMigration {
	return []GormMigration{
		InlineGormMigration{
			ID: "0001_add_user",
			OnUp: func(db *gorm.DB) error {
				// Here we are not using db.Migrator().CreateTable(&User{}) because when resetting
				// migrations to zero, it will create field which would not have existed before
				if err := db.Exec("CREATE TABLE `users` (`id` integer,`email` text,PRIMARY KEY (`id`))").Error; err != nil {
					return err
				}
				if err := db.Exec("CREATE UNIQUE INDEX `idx_users_email` ON `users`(`email`)").Error; err != nil {
					return err
				}
				return nil
			},
			OnDown: func(db *gorm.DB) error {
				return db.Migrator().DropTable(&User{})
			},
		},
		InlineGormMigration{
			ID: "0002_add_user_first_name",
			OnUp: func(db *gorm.DB) error {
				return db.Migrator().AddColumn(&User{}, "FirstName")
			},
			OnDown: func(db *gorm.DB) error {
				return db.Migrator().DropColumn(&User{}, "FirstName")
			},
		},
	}
}

func TestMigrate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Error(err)
	}

	app := &UserApp{}
	if s, err := Migrate(db.Debug(), app); err != nil {
		t.Error(err)
	} else {
		s.Print()
	}

	if s, err := MigrateTo(db.Debug(), app, "zero"); err != nil {
		t.Error(err)
	} else {
		s.Print()
	}
}
