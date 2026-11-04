package database

import "testing"

func TestOpenPostgresRejectsEmptyDSN(t *testing.T) {
	_, err := OpenPostgres("")
	if err == nil {
		t.Fatal("expected empty DSN error")
	}
}
