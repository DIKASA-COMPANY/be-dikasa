package user

import (
	"context"
	"errors"
	"testing"
)

type stubRepository struct {
	user *User
	err  error
}

func (stub stubRepository) FindByIdentifier(context.Context, string) (*User, error) {
	return stub.user, stub.err
}

func (stub stubRepository) FindByID(context.Context, string) (*User, error) {
	return stub.user, stub.err
}

func TestServiceFindByIdentifier(t *testing.T) {
	expected := &User{ID: "user-id", Username: "kasir"}
	service := NewService(stubRepository{user: expected})

	actual, err := service.FindByIdentifier(context.Background(), "kasir")

	if err != nil {
		t.Fatalf("find user: %v", err)
	}
	if actual.ID != expected.ID {
		t.Fatalf("expected user %q, got %q", expected.ID, actual.ID)
	}
}

func TestServicePropagatesRepositoryError(t *testing.T) {
	expected := errors.New("database failure")
	service := NewService(stubRepository{err: expected})

	_, err := service.FindByIdentifier(context.Background(), "kasir")

	if !errors.Is(err, expected) {
		t.Fatalf("expected %v, got %v", expected, err)
	}
}
