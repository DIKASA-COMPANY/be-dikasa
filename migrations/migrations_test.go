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

func TestCashSessionMigrationStoresOpeningCashPerOutlet(t *testing.T) {
	content, err := os.ReadFile("000003_create_cash_sessions.up.sql")
	if err != nil {
		t.Fatalf("read cash session migration: %v", err)
	}
	sql := strings.ToLower(string(content))
	for _, fragment := range []string{
		"create table cash_sessions", "outlet_id uuid not null", "user_id uuid not null",
		"opening_cash bigint not null", "check (opening_cash >= 0", "foreign key (outlet_id)",
		"foreign key (user_id)", "cash_sessions_outlet_open_uidx", "where status = 'open'",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("cash session migration must contain %q", fragment)
		}
	}
}

func TestCatalogMigrationCreatesOutletScopedMenuAndDefaults(t *testing.T) {
	content, err := os.ReadFile("000004_create_catalog.up.sql")
	if err != nil {
		t.Fatalf("read catalog migration: %v", err)
	}
	sql := strings.ToLower(string(content))
	for _, fragment := range []string{
		"create table product_types", "create table product_categories", "create table products", "create table order_types",
		"product_type_id uuid not null", "category_id uuid not null", "outlet_id uuid not null", "price bigint not null",
		"image_url text", "image_object_name text", "foreign key (category_id, outlet_id)",
		"insert into product_types", "'food'", "'drink'", "'additional'",
		"'free_table'", "'table_order'", "'takeaway'", "'delivery'", "'reservation'", "'online_delivery'",
		"update users", "lower(r.name) = 'admin'", "lower(m.name) = 'manager'",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("catalog migration must contain %q", fragment)
		}
	}
}

func TestUserProfileMigrationAddsAvatarStorageReferences(t *testing.T) {
	content, err := os.ReadFile("000005_add_user_profile_avatar.up.sql")
	if err != nil {
		t.Fatalf("read user profile migration: %v", err)
	}
	sql := strings.ToLower(string(content))
	for _, fragment := range []string{"alter table users", "avatar_url text", "avatar_object_name text"} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("user profile migration must contain %q", fragment)
		}
	}
}

func TestReservationAndProductCommerceMigrationMatchesPOSForms(t *testing.T) {
	content, err := os.ReadFile("000006_add_reservations_and_product_commerce.up.sql")
	if err != nil {
		t.Fatalf("read reservation migration: %v", err)
	}
	sql := strings.ToLower(string(content))
	for _, fragment := range []string{
		"alter table products", "stock_status", "online_selling_price", "cost_price", "online_cost_price",
		"create table product_variants", "create table reservation_types", "create table restaurant_tables",
		"create table reservations", "customer_name", "customer_contact", "reserved_at",
		"print_reservation", "send_reservation", "created_by", "reservations_table_time_uidx",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("reservation/product migration must contain %q", fragment)
		}
	}
}

func TestMakefileAppliesAndRollsBackMigrationsInOrder(t *testing.T) {
	content, err := os.ReadFile("../Makefile")
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	makefile := string(content)
	for _, migration := range []string{
		"migrations/000002_add_registration_otp.up.sql",
		"migrations/000002_add_registration_otp.down.sql",
		"migrations/000003_create_cash_sessions.up.sql",
		"migrations/000003_create_cash_sessions.down.sql",
		"migrations/000004_create_catalog.up.sql",
		"migrations/000004_create_catalog.down.sql",
		"migrations/000005_add_user_profile_avatar.up.sql",
		"migrations/000005_add_user_profile_avatar.down.sql",
		"migrations/000006_add_reservations_and_product_commerce.up.sql",
		"migrations/000006_add_reservations_and_product_commerce.down.sql",
	} {
		if !strings.Contains(makefile, migration) {
			t.Errorf("Makefile must run %s", migration)
		}
	}
	if strings.Index(makefile, "000002_add_registration_otp.down.sql") > strings.Index(makefile, "000001_create_identity_tables.down.sql") {
		t.Fatal("migrate-down must roll back registration before identity tables")
	}
	if strings.Index(makefile, "000003_create_cash_sessions.down.sql") > strings.Index(makefile, "000002_add_registration_otp.down.sql") {
		t.Fatal("migrate-down must roll back cash sessions before registration")
	}
	if strings.Index(makefile, "000004_create_catalog.down.sql") > strings.Index(makefile, "000003_create_cash_sessions.down.sql") {
		t.Fatal("migrate-down must roll back catalog before cash sessions")
	}
	if strings.Index(makefile, "000005_add_user_profile_avatar.down.sql") > strings.Index(makefile, "000004_create_catalog.down.sql") {
		t.Fatal("migrate-down must roll back user profile before catalog")
	}
	if strings.Index(makefile, "000006_add_reservations_and_product_commerce.down.sql") > strings.Index(makefile, "000005_add_user_profile_avatar.down.sql") {
		t.Fatal("migrate-down must roll back reservations before user profile")
	}
}
