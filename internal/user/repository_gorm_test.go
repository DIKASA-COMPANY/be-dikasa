package user

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"
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

func TestGORMRepositoryUpdatesCredentialHashes(t *testing.T) {
	repository, mock := newRepositoryMock(t)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET "pin_hash"=\$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs("pin-hash", sqlmock.AnyArg(), "user-id").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := repository.UpdatePINHash(context.Background(), "user-id", "pin-hash"); err != nil {
		t.Fatalf("update PIN: %v", err)
	}
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET "password_hash"=\$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs("password-hash", sqlmock.AnyArg(), "user-id").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := repository.UpdatePasswordHash(context.Background(), "user-id", "password-hash"); err != nil {
		t.Fatalf("update password: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGORMRepositoryMapsDuplicateProfileFieldsToConflict(t *testing.T) {
	repository, mock := newRepositoryMock(t)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET`).WillReturnError(&pgconn.PgError{Code: "23505", ConstraintName: "users_email_lower_uidx"})
	mock.ExpectRollback()
	err := repository.UpdateProfile(context.Background(), &User{ID: "user-id", FirstName: "Jane", PhoneNumber: "0812345678"})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected profile conflict, got %v", err)
	}
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

func TestTranslatedDuplicateProfileError(t *testing.T) {
	if !errors.Is(normalizeUserWriteError(gorm.ErrDuplicatedKey), ErrConflict) {
		t.Fatal("translated unique constraint must map to conflict")
	}
}
func TestProfileReloadOccursBeforeCommit(t *testing.T) {
	r, m := newRepositoryMock(t)
	m.ExpectBegin()
	m.ExpectQuery(`SELECT \* FROM "users"`).WillReturnRows(sqlmock.NewRows([]string{"id", "first_name", "phone_number"}).AddRow("user-id", "Old", "0812345678"))
	m.ExpectExec(`UPDATE "users" SET`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT \* FROM "users"`).WillReturnError(errors.New("reload failed"))
	m.ExpectRollback()
	_, err := r.PatchProfile(context.Background(), &User{ID: "user-id", FirstName: "New"}, ProfileMutation{Fields: map[string]bool{"first_name": true}}, nil)
	if err == nil {
		t.Fatal("reload failure hidden")
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
