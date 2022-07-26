package mysql

import (
	"context"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	log "github.com/sirupsen/logrus"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"time"
)

const defaultMigrationPath = "./db/migrations"

type (
	Migrator struct {
		settings Settings
		log      *zap.SugaredLogger
	}

	Settings struct {
		Connection string `yaml:"connection"`
		// User can also be specified separately from the connection string
		User string `yaml:"user"`
		// Password can also be specified separately from the connection string
		Password string `yaml:"password"`
		// User can also be specified separately from the connection string
		MigrateUser string `yaml:"migrateUser"`
		// Password can also be specified separately from the connection string
		MigratePassword string `yaml:"migratePassword"`
		// MaxLifetime is the maximum lifetime of a connection
		// from time.ParseDuration:
		// A duration string is a possibly signed sequence of
		// decimal numbers, each with optional fraction and a unit suffix,
		// such as "300ms", "-1.5h" or "2h45m".
		// Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".
		MaxLifetime        MDuration `yaml:"maxLifetime"`
		MaxOpenConnections int       `yaml:"maxOpenConnections"`
		MaxIdleConnections int       `yaml:"maxIdleConnections"`
		MigrationPath      string    `yaml:"migrationPath"`
	}

	MDuration struct {
		time.Duration
	}
)

func (d *Settings) ConnectionUrl(migration bool) (string, error) {
	cfg, err := mysql.ParseDSN(d.Connection)
	if err != nil {
		return "", err
	}
	if migration {
		cfg.User = d.MigrateUser
		cfg.Passwd = d.MigratePassword

	} else {
		cfg.User = d.User
		cfg.Passwd = d.Password
	}
	if migration {
		return fmt.Sprintf("mysql://%s", cfg.FormatDSN()), nil
	}
	cfg.ParseTime = true
	return cfg.FormatDSN(), nil
}

func (d *MDuration) UnmarshalJSON(data []byte) error {
	s := string(data)
	if len(s) > 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}

	// remove quotes
	var err error
	d.Duration, err = time.ParseDuration(s)
	if err != nil {
		return err
	}
	return nil
}

func NewMigrator(lc fx.Lifecycle, settings Settings, log *zap.SugaredLogger) *Migrator {
	m := &Migrator{
		settings: settings,
		log:      log,
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			return m.migrate()
		},
	})

	return m
}

func (m *Migrator) migrate() error {
	databaseConfig := m.settings

	c, err := databaseConfig.ConnectionUrl(true)
	if err != nil {
		return err
	}

	migrationPath := databaseConfig.MigrationPath
	if migrationPath == "" {
		m.log.Infof("No database.migrationPath configured, defaulting to: %s", defaultMigrationPath)
		migrationPath = defaultMigrationPath
	}
	migrationInstance, err := migrate.New(fmt.Sprintf("file://%s", migrationPath), c)
	if err != nil {
		return err
	}
	err = migrationInstance.Up()
	if err == migrate.ErrNoChange {
		log.Infof("No change detected.")
		return nil
	}
	return err
}
