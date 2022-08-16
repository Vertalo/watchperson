// Copyright 2022 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package database

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lopezator/migrator"
	"github.com/mattn/go-sqlite3"
)

var (
	sqliteMigrations = migrator.Migrations(
		execsql(
			"create_customer_name_watches",
			`create table if not exists customer_name_watches(id primary key, name, webhook, auth_token, created_at datetime, deleted_at datetime);`,
		),
		execsql(
			"create_customer_status",
			`create table if not exists customer_status(customer_id primary key, user_id, note, status, created_at datetime, deleted_at datetime);`,
		),
		execsql(
			"create_customer_watches",
			`create table if not exists customer_watches(id primary key, customer_id, webhook, auth_token, created_at datetime, deleted_at datetime);`,
		),
		execsql(
			"create_company_name_watches",
			`create table if not exists company_name_watches(id primary key, name, webhook, auth_token, created_at datetime, deleted_at datetime);`,
		),
		execsql(
			"create_company_status",
			`create table if not exists company_status(company_id primary key, user_id, note, status, created_at datetime, deleted_at datetime);`,
		),
		execsql(
			"create_company_watches",
			`create table if not exists company_watches(id primary key, company_id, webhook, auth_token, created_at datetime, deleted_at datetime);`,
		),
		execsql(
			"create_ofac_download_stats",
			`create table if not exists ofac_download_stats(downloaded_at datetime, sdns, alt_names, addresses);`,
		),
		execsql(
			"create_webhook_stats",
			`create table if not exists webhook_stats(watch_id string, attempted_at datetime, status);`,
		),
		execsql(
			"add_denied_persons_to_ofac_download_stats",
			"alter table ofac_download_stats add column denied_persons default 0;",
		),
		execsql(
			"rename_ofac_download_stats",
			"alter table ofac_download_stats rename to download_stats",
		),
		execsql(
			"add_sectoral_sanctions_to_download_stats",
			"alter table download_stats add column sectoral_sanctions default 0;",
		),
		execsql(
			"add__bis_entities__to_download_stats",
			"alter table download_stats add column bis_entities default 0;",
		),
	)
)

type sqlite struct {
	path string
	err  error
}

func (s *sqlite) Connect() (*sql.DB, error) {
	if s.err != nil {
		return nil, fmt.Errorf("sqlite had error %v", s.err)
	}

	db, err := sql.Open("sqlite3", s.path)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return db, err
	}

	// Migrate our database
	if m, err := migrator.New(sqliteMigrations); err != nil {
		return db, err
	} else {
		if err := m.Migrate(db); err != nil {
			return db, err
		}
	}

	return db, err
}

func sqliteConnection(path string) *sqlite {
	return &sqlite{
		path: path,
	}
}

func getSqlitePath() string {
	path := os.Getenv("SQLITE_DB_PATH")
	if path == "" || strings.Contains(path, "..") {
		// set default if empty or trying to escape
		// don't filepath.ABS to avoid full-fs reads
		path = "watchman.db"
	}
	return path
}

// TestSQLiteDB is a wrapper around sql.DB for SQLite connections designed for tests to provide
// a clean database for each testcase.  Callers should cleanup with Close() when finished.
type TestSQLiteDB struct {
	DB *sql.DB

	dir string // temp dir created for sqlite files
}

func (r *TestSQLiteDB) Close() error {
	if err := r.DB.Close(); err != nil {
		return err
	}
	return os.RemoveAll(r.dir)
}

// CreateTestSqliteDB returns a TestSQLiteDB which can be used in tests
// as a clean sqlite database. All migrations are ran on the db before.
//
// Callers should call close on the returned *TestSQLiteDB.
func CreateTestSqliteDB(t *testing.T) *TestSQLiteDB {
	dir, err := ioutil.TempDir("", "sqlite")
	if err != nil {
		t.Fatalf("sqlite test: %v", err)
	}

	db, err := sqliteConnection(filepath.Join(dir, "watchman.db")).Connect()
	if err != nil {
		t.Fatalf("sqlite test: %v", err)
	}
	return &TestSQLiteDB{db, dir}
}

// SqliteUniqueViolation returns true when the provided error matches the SQLite error
// for duplicate entries (violating a unique table constraint).
func SqliteUniqueViolation(err error) bool {
	match := strings.Contains(err.Error(), "UNIQUE constraint failed")
	if e, ok := err.(sqlite3.Error); ok {
		return match || e.Code == sqlite3.ErrConstraint
	}
	return match
}
