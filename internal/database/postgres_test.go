package database

import "testing"

func TestOpenPostgresRejectsEmptyDSN(t *testing.T) {
	_, err := OpenPostgres("")
	if err == nil {
		t.Fatal("expected empty DSN error")
	}
}

func TestOpenFirebaseRejectsEmptyProjectID(t *testing.T) {
	_, err := OpenFirebase(t.Context(), "")
	if err == nil {
		t.Fatal("expected empty Firebase project ID error")
	}
}
