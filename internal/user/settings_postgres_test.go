package user_test

import (
	"be-dikasa/internal/devseed"
	"be-dikasa/internal/user"
	"context"
	"errors"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
)

func accountDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DIKASA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("opt in with DIKASA_TEST_DATABASE_URL")
	}
	u, e := url.Parse(dsn)
	if e != nil || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1") {
		t.Fatal("requires loopback PostgreSQL")
	}
	for k := range u.Query() {
		if k != "sslmode" && k != "connect_timeout" {
			t.Fatal("unsupported database URL option")
		}
	}
	base, e := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	schema := "dikasa_account_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if e = base.Exec("CREATE SCHEMA " + schema).Error; e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := base.Exec("DROP SCHEMA " + schema + " CASCADE").Error; e != nil {
			t.Error(e)
		}
		sql, _ := base.DB()
		sql.Close()
	})
	q := u.Query()
	q.Set("search_path", schema+",public")
	u.RawQuery = q.Encode()
	db, e := gorm.Open(postgres.New(postgres.Config{DSN: u.String(), PreferSimpleProtocol: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { sql, _ := db.DB(); sql.Close() })
	files, _ := filepath.Glob("../../migrations/*.up.sql")
	sort.Strings(files)
	for _, f := range files {
		b, e := os.ReadFile(f)
		if e != nil {
			t.Fatal(e)
		}
		if e = db.Exec(string(b)).Error; e != nil {
			t.Fatalf("%s: %v", filepath.Base(f), e)
		}
	}
	if e = devseed.Run(db, "integration-only-password"); e != nil {
		t.Fatal(e)
	}
	return db
}

func TestAccountSettingsPostgres(t *testing.T) {
	db := accountDB(t)
	ctx := context.Background()
	var a user.User
	if e := db.Where("username = ?", "admin").First(&a).Error; e != nil {
		t.Fatal(e)
	}
	repo := user.NewGORMRepository(db)
	svc := user.NewService(repo)
	var out user.Outlet
	db.First(&out, "id = ?", *a.OutletID)
	address := out.Address
	choices, e := svc.ListGenders(ctx)
	if e != nil || len(choices) == 0 {
		t.Fatal("gender reference", e)
	}
	p, e := svc.UpdateProfile(ctx, a.ID, user.ProfileMutation{PersonalAddress: "Personal Home", DateOfBirth: "1990-08-10", GenderID: choices[0].ID, Fields: map[string]bool{"personal_address": true, "date_of_birth": true, "gender_id": true}})
	if e != nil || p.PersonalAddress == nil || p.Gender == nil || p.Outlet == nil || p.Role.Name == "" {
		t.Fatal("profile mapping", e)
	}
	_, e = svc.UpdateProfile(ctx, a.ID, user.ProfileMutation{GenderID: uuid.NewString(), Fields: map[string]bool{"gender_id": true}})
	if !errors.Is(e, user.ErrInvalidProfile) {
		t.Fatal("unknown gender", e)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, m := range []user.ProfileMutation{{FirstName: "Edited", Fields: map[string]bool{"first_name": true}}, {Email: "changed@example.com", Fields: map[string]bool{"email": true}}} {
		wg.Add(1)
		go func(m user.ProfileMutation) { defer wg.Done(); _, e := svc.UpdateProfile(ctx, a.ID, m); errs <- e }(m)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	p, e = svc.FindByID(ctx, a.ID)
	if e != nil || p.FirstName != "Edited" || p.Email == nil || *p.Email != "changed@example.com" {
		t.Fatal("disjoint updates lost", e)
	}
	db.First(&out, "id = ?", *a.OutletID)
	if out.Address != address {
		t.Fatal("outlet address changed")
	}
	old := p.PasswordHash
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, next := range []string{"next-a", "next-b"} {
		go func(next string) { <-start; results <- repo.ReplaceCredential(ctx, a.ID, "password_hash", &old, next) }(next)
	}
	close(start)
	wins := 0
	for i := 0; i < 2; i++ {
		e := <-results
		if e == nil {
			wins++
		} else if !errors.Is(e, user.ErrConflict) {
			t.Fatal(e)
		}
	}
	if wins != 1 {
		t.Fatal("credential CAS winners", wins)
	}
	// A stale photo mutation must neither overwrite the winning reference nor lose disjoint text.
	photo := "https://project.supabase.co/storage/v1/object/public/dikasa-images/avatar.png"
	db.Model(&user.User{}).Where("id = ?", a.ID).Update("avatar_url", photo)
	_, e = repo.PatchProfile(ctx, p, user.ProfileMutation{RemoveAvatar: true, Fields: map[string]bool{"remove_avatar": true}}, nil)
	if !errors.Is(e, user.ErrConflict) {
		t.Fatal("stale avatar mutation accepted", e)
	}
	p, e = svc.UpdateProfile(ctx, a.ID, user.ProfileMutation{Fields: map[string]bool{"personal_address": true, "date_of_birth": true, "gender_id": true}})
	if e != nil || p.PersonalAddress != nil || p.DateOfBirth != nil || p.GenderID != nil {
		t.Fatal("optional clearing", e)
	}
}
