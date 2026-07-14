package database

import "testing"

func TestOpenMySQLRejectsEmptyDSN(t *testing.T) {
	if _, err := OpenMySQL(""); err == nil {
		t.Fatalf("OpenMySQL() error = nil, want configuration error")
	}
}
