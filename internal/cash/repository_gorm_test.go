package cash

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newCashRepositoryMock(t *testing.T) (*GORMRepository, sqlmock.Sqlmock) {
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

func TestGORMRepositoryCreatesCashSession(t *testing.T) {
	repository, mock := newCashRepositoryMock(t)
	openedAt := time.Date(2026, time.August, 19, 1, 30, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "cash_sessions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("session-id"))
	mock.ExpectCommit()
	session := &Session{OutletID: "outlet-id", UserID: "user-id", OpeningCash: 500_000, Status: StatusOpen, OpenedAt: openedAt}

	err := repository.Create(context.Background(), session)

	if err != nil {
		t.Fatalf("create cash session: %v", err)
	}
	if session.ID != "session-id" {
		t.Fatalf("expected generated id, got %q", session.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestGORMRepositoryMapsConcurrentOpenSessionConflict(t *testing.T) {
	repository, mock := newCashRepositoryMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "cash_sessions"`).
		WillReturnError(&pgconn.PgError{Code: "23505", ConstraintName: "cash_sessions_outlet_open_uidx"})
	mock.ExpectRollback()

	err := repository.Create(context.Background(), &Session{OutletID: "outlet-id", UserID: "user-id", Status: StatusOpen})

	if !errors.Is(err, ErrActiveSessionExists) {
		t.Fatalf("expected ErrActiveSessionExists, got %v", err)
	}
}

func TestGORMRepositoryFindsOpenSessionByOutlet(t *testing.T) {
	repository, mock := newCashRepositoryMock(t)
	openedAt := time.Date(2026, time.August, 19, 1, 30, 0, 0, time.UTC)
	mock.ExpectQuery(`SELECT \* FROM "cash_sessions" WHERE outlet_id = \$1 AND status = \$2 ORDER BY opened_at DESC,"cash_sessions"\."id" LIMIT \$3`).
		WithArgs("outlet-id", StatusOpen, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "outlet_id", "user_id", "opening_cash", "status", "opened_at"}).
			AddRow("session-id", "outlet-id", "user-id", int64(500_000), StatusOpen, openedAt))

	session, err := repository.FindOpenByOutletID(context.Background(), "outlet-id")

	if err != nil {
		t.Fatalf("find open cash session: %v", err)
	}
	if session.ID != "session-id" || session.OpeningCash != 500_000 {
		t.Fatalf("unexpected cash session: %+v", session)
	}
}

func TestGORMRepositoryMapsMissingOpenSession(t *testing.T) {
	repository, mock := newCashRepositoryMock(t)
	mock.ExpectQuery(`SELECT \* FROM "cash_sessions"`).
		WithArgs("outlet-id", StatusOpen, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, err := repository.FindOpenByOutletID(context.Background(), "outlet-id")

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
