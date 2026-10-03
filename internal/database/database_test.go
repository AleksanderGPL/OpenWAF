package database

import (
	"path/filepath"
	"testing"
)

func TestOpenFilePath(t *testing.T) {
	// Spaces, #, and % must remain filename characters in the SQLite URI.
	path := filepath.Join(t.TempDir(), "data with spaces", "open#waf%.db")
	for i := 0; i < 2; i++ {
		db, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatal(err)
		}
		var foreignKeys int
		err = sqlDB.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys)
		if err != nil || foreignKeys != 1 {
			sqlDB.Close()
			t.Fatalf("foreign_keys = %d, error = %v", foreignKeys, err)
		}
		if err := sqlDB.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
