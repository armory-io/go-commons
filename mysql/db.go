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
	"database/sql"
	"fmt"
	"github.com/XSAM/otelsql"
	"github.com/armory-io/go-commons/opentelemetry"
	"go.opentelemetry.io/otel/sdk/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.18.0"
)

func New(
	settings Configuration,
	tracing opentelemetry.Configuration,
	mp *metric.MeterProvider,
) (*sql.DB, error) {
	conn, err := settings.ConnectionUrl(false)
	if err != nil {
		return nil, err
	}

	options := []otelsql.Option{
		otelsql.WithMeterProvider(mp),
	}
	if tracing.Push.Enabled {
		options = append(options,
			otelsql.WithSpanNameFormatter(spanNameFormatter{}),
			otelsql.WithSpanOptions(otelsql.SpanOptions{DisableErrSkip: true}),
			otelsql.WithAttributes(
				semconv.DBSystemMySQL,
			),
		)
	}

	db, err := otelsql.Open("mysql", conn, options...)
	if err != nil {
		return nil, err
	}

	db.SetConnMaxLifetime(settings.MaxLifetime.Duration)
	db.SetMaxOpenConns(settings.MaxOpenConnections)
	db.SetMaxIdleConns(settings.MaxIdleConnections)
	return db, nil
}

type spanNameFormatter struct {
}

func (f spanNameFormatter) Format(ctx context.Context, method otelsql.Method, query string) string {
	return fmt.Sprintf("%s.%s", method, firstN(query, 100))
}

func firstN(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
