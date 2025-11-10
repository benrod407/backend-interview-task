package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

type DB struct {
	*sql.DB
}

func NewDB(ctx context.Context, dataSourceName string) (*DB, error) {
	db, err := sql.Open("mysql", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	// Check if DB is reachable with ping. The retry limit is 30 seconds
	deadline := time.Now().Add(30 * time.Second)
	for {
		if err = db.PingContext(ctx); err == nil {
			break
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("db ping timeout: %w", err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	log.Print("db successfully initialized")
	return &DB{db}, nil
}
