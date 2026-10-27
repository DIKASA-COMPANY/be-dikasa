package user

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

type settingsRepository struct {
	account      *User
	updated      *User
	pinHash      string
	passwordHash string
	err          error
	updateErr    error
}

func (repository *settingsRepository) FindByIdentifier(context.Context, string) (*User, error) {
	return repository.account, repository.err
}
func (repository *settingsRepository) FindByID(context.Context, string) (*User, error) {
	return repository.account, repository.err
}
func (repository *settingsRepository) UpdateProfile(_ context.Context, account *User) error {
	repository.updated = account
	return repository.updateErr
}
func (repository *settingsRepository) UpdatePINHash(_ context.Context, _ string, hash string) error {
	repository.pinHash = hash
	return repository.err
}
func (repository *settingsRepository) UpdatePasswordHash(_ context.Context, _ string, hash string) error {
	repository.passwordHash = hash
	return repository.err
}

type plainCredentialHasher struct{}

func (plainCredentialHasher) Hash(value string) (string, error) { return "hash:" + value, nil }
func (plainCredentialHasher) Compare(hash, value string) error {
	if hash != "hash:"+value {
		return errors.New("mismatch")
	}
	return nil
}

type fakeAvatarStore struct {
	stored  AvatarImage
	deleted []string
	err     error
}

func (store *fakeAvatarStore) Save(context.Context, string, string, *AvatarUpload) (*AvatarImage, error) {
	if store.err != nil {
		return nil, store.err
	}
	return &store.stored, nil
}
func (store *fakeAvatarStore) Delete(_ context.Context, name string) error {
	store.deleted = append(store.deleted, name)
	return store.err
}

func TestUpdateProfileEditsOnlyProvidedFields(t *testing.T) {
	email := "lama@example.com"
	outletID := "outlet-a"
	repository := &settingsRepository{account: &User{ID: "user-a", OutletID: &outletID, FirstName: "Jane", LastName: "Doe", PhoneNumber: "08123456", Email: &email}}
	service := NewService(repository, WithCredentialHasher(plainCredentialHasher{}), WithAvatarStore(NewDisabledAvatarStore()))

	updated, err := service.UpdateProfile(context.Background(), "user-a", ProfileMutation{FirstName: "  Janet ", PhoneNumber: " 08199999 ", Fields: map[string]bool{"first_name": true, "phone_number": true}})
	if err != nil {
		t.Fatalf("update profile: %v", err)
	}
	if updated.FirstName != "Janet" || updated.LastName != "Doe" || updated.PhoneNumber != "08199999" || *updated.Email != email {
		t.Fatalf("omitted profile fields changed: %+v", updated)
	}
}

func TestUpdateProfileCleansAvatarWhenDatabaseFails(t *testing.T) {
	outletID := "outlet-a"
	repository := &settingsRepository{account: &User{ID: "user-a", OutletID: &outletID, FirstName: "Jane", PhoneNumber: "08123456"}, updateErr: errors.New("database unavailable")}
	avatars := &fakeAvatarStore{stored: AvatarImage{URL: "https://new", ObjectName: "new-avatar"}}
	service := NewService(repository, WithCredentialHasher(plainCredentialHasher{}), WithAvatarStore(avatars))

	_, err := service.UpdateProfile(context.Background(), "user-a", ProfileMutation{Fields: map[string]bool{}, Avatar: &AvatarUpload{Reader: bytes.NewReader([]byte("image")), Size: 5}})
	if err == nil || len(avatars.deleted) != 1 || avatars.deleted[0] != "new-avatar" {
		t.Fatalf("expected avatar cleanup, err=%v deleted=%v", err, avatars.deleted)
	}
}

func TestChangePINSupportsFirstSetAndVerifiesExistingPIN(t *testing.T) {
	repository := &settingsRepository{account: &User{ID: "user-a"}}
	service := NewService(repository, WithCredentialHasher(plainCredentialHasher{}))
	if err := service.ChangePIN(context.Background(), "user-a", "", "123456"); err != nil || repository.pinHash != "hash:123456" {
		t.Fatalf("first PIN: hash=%q err=%v", repository.pinHash, err)
	}
	existing := "hash:123456"
	repository.account.PINHash = &existing
	if err := service.ChangePIN(context.Background(), "user-a", "000000", "654321"); !errors.Is(err, ErrInvalidCurrentPIN) {
		t.Fatalf("expected old PIN rejection, got %v", err)
	}
}

func TestChangePasswordVerifiesCurrentAndRejectsReuse(t *testing.T) {
	repository := &settingsRepository{account: &User{ID: "user-a", PasswordHash: "hash:rahasia123"}}
	service := NewService(repository, WithCredentialHasher(plainCredentialHasher{}))
	if err := service.ChangePassword(context.Background(), "user-a", "salah123", "baru12345"); !errors.Is(err, ErrInvalidCurrentPassword) {
		t.Fatalf("expected old password rejection, got %v", err)
	}
	if err := service.ChangePassword(context.Background(), "user-a", "rahasia123", "rahasia123"); !errors.Is(err, ErrCredentialReuse) {
		t.Fatalf("expected reuse rejection, got %v", err)
	}
	if err := service.ChangePassword(context.Background(), "user-a", "rahasia123", "baru12345"); err != nil || repository.passwordHash != "hash:baru12345" {
		t.Fatalf("change password: hash=%q err=%v", repository.passwordHash, err)
	}
}

func TestUncertainProfileCommitDoesNotDeleteReferencedUpload(t *testing.T) {
	out := "outlet"
	object := "new-avatar"
	repo := &settingsRepository{account: &User{ID: "a", OutletID: &out, FirstName: "Jane", PhoneNumber: "0812345678", AvatarObjectName: &object}, updateErr: errors.New("commit reply lost")}
	store := &fakeAvatarStore{stored: AvatarImage{URL: "https://new", ObjectName: object}}
	svc := NewService(repo, WithAvatarStore(store))
	_, err := svc.UpdateProfile(context.Background(), "a", ProfileMutation{Fields: map[string]bool{}, Avatar: &AvatarUpload{Reader: bytes.NewReader([]byte("image")), Size: 5}})
	if err == nil || len(store.deleted) != 0 {
		t.Fatal("potentially committed avatar was deleted", store.deleted)
	}
}
