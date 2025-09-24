package main

import (
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

func NewDB(config Config) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch config.DbType {
	case "mysql":
		dialector = mysql.Open(config.DbDsn)

	case "sqlite":
		dialector = sqlite.Open(config.DbDsn)

	case "postgres":
		dialector = postgres.Open(config.DbDsn)

	case "sqlserver":
		dialector = sqlserver.Open(config.DbDsn)

	default:
		return nil, fmt.Errorf("unsupported database type: %s", config.DbType)
	}

	return gorm.Open(dialector, &gorm.Config{})
}
