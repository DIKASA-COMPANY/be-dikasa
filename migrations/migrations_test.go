package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestIdentityMigrationCreatesPostgreSQLTablesAndConstraints(t *testing.T) {
	content, err := os.ReadFile("000001_create_identity_tables.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := strings.ToLower(string(content))
	for _, fragment := range []string{
		"create table roles", "create table genders", "create table outlets", "create table users",
		"password_hash", "foreign key (role_id)", "foreign key (gender_id)", "foreign key (outlet_id)",
		"unique index users_username_lower_uidx", "unique index users_email_lower_uidx",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("migration must contain %q", fragment)
		}
	}
	if strings.Contains(sql, "password varchar") || strings.Contains(sql, "pin varchar") {
		t.Fatal("migration must only persist password_hash and pin_hash")
	}
}

func TestRegistrationOTPMigrationStoresOnlyHashedCodes(t *testing.T) {
	content, err := os.ReadFile("000002_add_registration_otp.up.sql")
	if err != nil {
		t.Fatalf("read registration migration: %v", err)
	}
	sql := strings.ToLower(string(content))
	for _, fragment := range []string{
		"alter table outlets add column email", "create table registration_otps", "code_hash",
		"expires_at", "resend_available_at", "attempts", "verified_at", "foreign key (user_id)",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("registration migration must contain %q", fragment)
		}
	}
	if strings.Contains(sql, " otp varchar") || strings.Contains(sql, " code varchar") {
		t.Fatal("migration must not persist a plaintext OTP")
	}
}

func TestMakefileAppliesAndRollsBackRegistrationMigration(t *testing.T) {
	content, err := os.ReadFile("../Makefile")
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	makefile := string(content)
	for _, migration := range []string{
		"migrations/000002_add_registration_otp.up.sql",
		"migrations/000002_add_registration_otp.down.sql",
	} {
		if !strings.Contains(makefile, migration) {
			t.Errorf("Makefile must run %s", migration)
		}
	}
	if strings.Index(makefile, "000002_add_registration_otp.down.sql") > strings.Index(makefile, "000001_create_identity_tables.down.sql") {
		t.Fatal("migrate-down must roll back registration before identity tables")
	}
}
