package celeritas

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgconn" // need this and next two for pgx
	_ "github.com/jackc/pgx/v4"
	_ "github.com/jackc/pgx/v4/stdlib"
)

// OpenDB opens a connection to a sql database. It requires a database type
// (mysql or pgx), and a complete connection string.
func (c *Celeritas) OpenDB(dbType, dsn string) (*sql.DB, error) {
	if dbType == "postgres" {
		dbType = "pgx"
	}
	db, err := sql.Open(dbType, dsn)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return db, nil
}
