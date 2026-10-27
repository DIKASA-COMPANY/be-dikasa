package user

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestProfilePersonalDetailsAndClearing(t *testing.T) {
	repo := &settingsRepository{account: &User{ID: "a", FirstName: "Jane", PhoneNumber: "0812345678"}}
	svc := NewService(repo, WithProfileClock(func() time.Time { return time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC) }))
	updated, err := svc.UpdateProfile(context.Background(), "a", ProfileMutation{DateOfBirth: "1990-08-10", PersonalAddress: "  Home  ", Fields: map[string]bool{"date_of_birth": true, "personal_address": true}})
	if err != nil || updated.DateOfBirth == nil || updated.DateOfBirth.Format("2006-01-02") != "1990-08-10" || updated.PersonalAddress == nil || *updated.PersonalAddress != "Home" {
		t.Fatalf("personal details: %+v %v", updated, err)
	}
	updated, err = svc.UpdateProfile(context.Background(), "a", ProfileMutation{Fields: map[string]bool{"date_of_birth": true, "personal_address": true}})
	if err != nil || updated.DateOfBirth != nil || updated.PersonalAddress != nil {
		t.Fatal("optional fields were not cleared", err)
	}
	for _, date := range []string{"2026-10-25", "2023-02-29", "10-08-1990"} {
		_, err = svc.UpdateProfile(context.Background(), "a", ProfileMutation{DateOfBirth: date, Fields: map[string]bool{"date_of_birth": true}})
		if !errors.Is(err, ErrInvalidProfile) {
			t.Fatalf("date %s accepted: %v", date, err)
		}
	}
	_, err = svc.UpdateProfile(context.Background(), "a", ProfileMutation{PersonalAddress: strings.Repeat("a", 1001), Fields: map[string]bool{"personal_address": true}})
	if !errors.Is(err, ErrInvalidProfile) {
		t.Fatal("oversized address accepted")
	}
}

func TestPasswordCapabilityPreventsWrite(t *testing.T) {
	repo := &settingsRepository{account: &User{ID: "a", PasswordHash: "hash:oldpassword"}}
	svc := NewService(repo, WithCredentialHasher(plainCredentialHasher{}), WithPasswordChangesAllowed(false))
	if err := svc.ChangePassword(context.Background(), "a", "oldpassword", "newpassword"); !errors.Is(err, ErrPasswordChangesDisabled) {
		t.Fatal(err)
	}
	if repo.passwordHash != "" {
		t.Fatal("disabled password wrote hash")
	}
}

func TestProfileDataIncludesSafeAccountContext(t *testing.T) {
	pin := "secret-hash"
	address := "Home"
	birth := time.Date(1990, 8, 10, 0, 0, 0, 0, time.UTC)
	p := profileData(&User{ID: "a", Role: Role{Name: "cashier"}, Outlet: &Outlet{Name: "Shop"}, PINHash: &pin, PersonalAddress: &address, DateOfBirth: &birth})
	if !p.HasPIN || p.Role != "cashier" || p.OutletName != "Shop" || p.DateOfBirth == nil || *p.DateOfBirth != "1990-08-10" {
		t.Fatalf("missing profile context: %+v", p)
	}
}

func TestProfileRejectedMutationDoesNotModifyRepository(t *testing.T) {
	repo := &settingsRepository{account: &User{ID: "a", FirstName: "Jane", PhoneNumber: "0812345678"}}
	_, err := NewService(repo).UpdateProfile(context.Background(), "a", ProfileMutation{FirstName: "Changed", DateOfBirth: "bad", Fields: map[string]bool{"first_name": true, "date_of_birth": true}})
	if err == nil || repo.account.FirstName != "Jane" {
		t.Fatal("failed validation mutated source")
	}
}
