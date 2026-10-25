package devseed

import (
	"database/sql/driver"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type bcryptHash struct{}

func (bcryptHash) Match(value driver.Value) bool {
	hash, ok := value.(string)
	return ok && bcrypt.CompareHashAndPassword([]byte(hash), []byte("dikasa123")) == nil
}

func TestSeedLeavesExistingUserUntouchedAndRejectsWeakPassword(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := Run(db, "short"); err == nil {
		t.Fatal("accepted weak password")
	}
	mock.ExpectBegin()
	mock.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS (SELECT 1 FROM users WHERE LOWER(username) = 'admin')")).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectCommit()
	if err := Run(db, "dikasa123"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSeedWritesBcryptUserAndOutletAtomically(t *testing.T) {
	sqlDB, mock, _ := sqlmock.New()
	defer sqlDB.Close()
	db, _ := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	mock.ExpectBegin()
	mock.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec("WITH new_outlet AS").WithArgs(bcryptHash{}).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := Run(db, "dikasa123"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
