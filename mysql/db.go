package mysql

import (
	"database/sql"
)

func New(settings Settings) (*sql.DB, error) {
	var err error
	conn, err := settings.ConnectionUrl(false)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", conn)
	if err != nil {
		return nil, err
	}
	db.SetConnMaxLifetime(settings.MaxLifetime.Duration)
	db.SetMaxOpenConns(settings.MaxOpenConnections)
	db.SetMaxIdleConns(settings.MaxIdleConnections)
	return db, nil
}
