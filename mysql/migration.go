/*
 * Copyright 2022 Armory, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package mysql

import (
	"context"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/samber/lo"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"time"
)

const defaultMigrationPath = "./db/migrations"

type (
	Migrator struct {
		settings Configuration
		log      *zap.SugaredLogger
	}

	Configuration struct {
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
		// if set to true, no migration will be applied - use for local development only
		SkipMigrations bool `yaml:"skipMigrations"`
	}

	MDuration struct {
		time.Duration
	}

	VersionProvider func() (uint, error)
)

func (d *Configuration) ConnectionUrl(migration bool) (string, error) {
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

func NewMigrator(lc fx.Lifecycle, settings Configuration, log *zap.SugaredLogger) *Migrator {
	m := &Migrator{
		settings: settings,
		log:      log,
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			if !m.settings.SkipMigrations {
				return m.migrate(nil)
			}
			m.log.Warn("migrations disabled")
			return nil
		},
	})

	return m
}

func NewMigratorV2(lc fx.Lifecycle, settings Configuration, versionProvider VersionProvider, log *zap.SugaredLogger) *Migrator {
	m := &Migrator{
		settings: settings,
		log:      log,
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			if version, err := versionProvider(); err != nil {
				m.log.Warnf("eror fetching target db version - %v", err)
				return err
			} else if !m.settings.SkipMigrations {
				m.log.Infof("setting active db version to %d", version)
				return m.migrate(&version)
			} else {
				m.log.Warn("migrations disabled")
				return nil
			}
		},
	})

	return m
}

func (m *Migrator) migrate(version *uint) error {
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
	err = lo.IfF(version == nil, migrationInstance.Up).ElseF(func() error {
		return migrationInstance.Migrate(*version)
	})
	if err == migrate.ErrNoChange {
		m.log.Infof("No change detected.")
		return nil
	}
	return err
}
