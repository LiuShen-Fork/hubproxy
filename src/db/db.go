package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

var (
	DB   *sql.DB
	once sync.Once
)

func Init(path string) error {
	var initErr error
	once.Do(func() {
		if path == "" {
			path = "data/hubproxy.db"
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			initErr = fmt.Errorf("create db dir: %w", err)
			return
		}

		dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)"
		database, err := sql.Open("sqlite", dsn)
		if err != nil {
			initErr = fmt.Errorf("open sqlite: %w", err)
			return
		}
		database.SetMaxOpenConns(1)
		database.SetMaxIdleConns(1)
		database.SetConnMaxLifetime(0)

		if err := database.Ping(); err != nil {
			_ = database.Close()
			initErr = fmt.Errorf("ping sqlite: %w", err)
			return
		}

		DB = database
		if err := migrate(); err != nil {
			initErr = err
			return
		}
	})
	return initErr
}

func Close() error {
	if DB == nil {
		return nil
	}
	return DB.Close()
}

// migrate is implemented in migrate_extra.go

func Now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func ParseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, s)
}
