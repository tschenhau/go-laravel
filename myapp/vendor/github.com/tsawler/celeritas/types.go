package celeritas

import (
	"database/sql"
)

// Server holds details about the server
type Server struct {
	ServerName string
	Port       string
	Secure     bool
}

// redisConfig holds redis config values
type redisConfig struct {
	host     string
	password string
	prefix   string
}

// databaseConfig holds database config values
type databaseConfig struct {
	dsn      string
	database string
}

// cookieConfig holds cookie config values
type cookieConfig struct {
	name     string
	lifetime string
	persist  string
	secure   string
	domain   string
}

// Database holds the database type and connection pool
type Database struct {
	DatabaseType string
	Pool         *sql.DB
}

// initPaths is used when initializing the application. It holds the root
// path for the application, and a slice of strings with the names of
// folders that the application expects to find.
type initPaths struct {
	rootPath    string
	folderNames []string
}
