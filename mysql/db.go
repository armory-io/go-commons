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
	"database/sql"
	"github.com/XSAM/otelsql"
	"github.com/armory-io/go-commons/tracing"
)

func New(settings Configuration, tracing tracing.Configuration) (*sql.DB, error) {
	conn, err := settings.ConnectionUrl(false)
	if err != nil {
		return nil, err
	}

	var db *sql.DB
	if tracing.Push.Enabled {
		db, err = otelsql.Open("mysql", conn, otelsql.WithSpanOptions(otelsql.SpanOptions{DisableErrSkip: true}))
	} else {
		db, err = sql.Open("mysql", conn)
	}

	if err != nil {
		return nil, err
	}

	db.SetConnMaxLifetime(settings.MaxLifetime.Duration)
	db.SetMaxOpenConns(settings.MaxOpenConnections)
	db.SetMaxIdleConns(settings.MaxIdleConnections)
	return db, nil
}
