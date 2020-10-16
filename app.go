package apps

// Represents an application
type Application interface {
	Name() string
	Migrations() []GormMigration
}
