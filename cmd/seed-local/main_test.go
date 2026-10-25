package main

import "testing"

func TestSeedTargetRejectsRemoteHostsAndConnectionOverrides(t *testing.T) {
	for _, dsn := range []string{
		"postgres://user:password@remote.example/dikasa",
		"postgres://user:password@localhost/dikasa?host=remote.example",
		"postgres://user:password@localhost/dikasa?hostaddr=192.0.2.1",
		"postgres://user:password@localhost/dikasa?service=production",
		"postgres://user:password@localhost/dikasa?%68ost=remote.example",
		"host=localhost dbname=dikasa",
	} {
		if err := validateSeedTarget("local", dsn); err == nil {
			t.Fatalf("accepted unsafe DSN: %s", dsn)
		}
	}
	if err := validateSeedTarget("firebase", "postgres://localhost/dikasa"); err == nil {
		t.Fatal("accepted firebase mode")
	}
	for _, host := range []string{"localhost", "127.0.0.1", "[::1]"} {
		if err := validateSeedTarget("local", "postgres://"+host+"/dikasa?sslmode=disable"); err != nil {
			t.Fatal(err)
		}
	}
}
