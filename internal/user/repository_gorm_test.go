package user

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newRepositoryMock(t *testing.T) (*GORMRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sql mock: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB, PreferSimpleProtocol: true}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open gorm: %v", err)
	}
	return NewGORMRepository(db), mock
}

func TestGORMRepositoryFindByIdentifier(t *testing.T) {
	repository, mock := newRepositoryMock(t)
	query := regexp.QuoteMeta(`SELECT * FROM "users" WHERE LOWER(username) = LOWER($1) OR phone_number = $2 OR LOWER(email) = LOWER($3) ORDER BY "users"."id" LIMIT $4`)
	mock.ExpectQuery(query).
		WithArgs("kasir", "kasir", "kasir", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "role_id", "username", "phone_number", "password_hash", "first_name", "last_name", "is_active"}).
			AddRow("user-id", "role-id", "kasir", "08123456789", "hash", "Budi", "Kasir", true))

	account, err := repository.FindByIdentifier(context.Background(), "kasir")

	if err != nil {
		t.Fatalf("find user: %v", err)
	}
	if account.ID != "user-id" || account.Username != "kasir" {
		t.Fatalf("unexpected user: %+v", account)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestGORMRepositoryMapsRecordNotFound(t *testing.T) {
	repository, mock := newRepositoryMock(t)
	mock.ExpectQuery(`SELECT \* FROM "users"`).
		WithArgs("unknown", "unknown", "unknown", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, err := repository.FindByIdentifier(context.Background(), "unknown")

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
