// Package database menginisialisasi PostgreSQL dan Firebase untuk aplikasi.
package database

import (
	"errors"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// OpenPostgres membuka koneksi GORM ke PostgreSQL dan mengatur connection pool.
func OpenPostgres(dsn string) (*gorm.DB, error) {
	if dsn == "" {
		return nil, errors.New("DATABASE_URL wajib diatur")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{PrepareStmt: true, TranslateError: true})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	return db, nil
}

// ClosePostgres menutup connection pool PostgreSQL.
func ClosePostgres(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
