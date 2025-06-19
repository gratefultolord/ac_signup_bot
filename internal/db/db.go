package db

import (
	"fmt"
	"time"

	"github.com/gratefultolord/ac_signup_bot/internal/config"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type DB struct {
	Conn *sqlx.DB
}

func New(cfg *config.Config) (*DB, error) {

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s, dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)

	dbConn, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("db.New: cannot connect to database: %w", err)
	}

	dbConn.SetMaxOpenConns(20)
	dbConn.SetMaxIdleConns(5)
	dbConn.SetConnMaxLifetime(60 * time.Minute)

	return &DB{Conn: dbConn}, nil
}

func (db *DB) Close() error {
	return db.Conn.Close()
}
