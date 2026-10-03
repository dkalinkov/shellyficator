package accountstore

import (
	"errors"
	"testing"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"shellyficator/internal/accounts"
	"shellyficator/internal/domain"
	_ "shellyficator/migrations"
)

func testAccounts(t *testing.T) (*accounts.Service, core.App) {
	t.Helper()
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := app.ResetBootstrapState(); err != nil {
			t.Error(err)
		}
	})
	return accounts.New(New(app)), app
}

func TestSetupAndSessionUseOperatorAccounts(t *testing.T) {
	service, app := testAccounts(t)
	if needed, err := service.NeedsSetup(); err != nil || !needed {
		t.Fatalf("setup unavailable: %v %v", needed, err)
	}
	const password = "account-test-password"
	if err := service.Setup(" operator@example.com ", password); err != nil {
		t.Fatal(err)
	}
	if needed, err := service.NeedsSetup(); err != nil || needed {
		t.Fatalf("setup stayed open: %v %v", needed, err)
	}
	if err := service.Setup("second@example.com", password); !errors.Is(err, domain.ErrSetupCompleted) {
		t.Fatalf("duplicate setup allowed: %v", err)
	}
	if _, err := service.Authenticate("operator@example.com", "incorrect"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("incorrect password accepted: %v", err)
	}
	session, err := service.Authenticate(" operator@example.com ", password)
	if err != nil || session.Token == "" || session.Operator.Email != "operator@example.com" {
		t.Fatalf("authentication failed: %+v %v", session, err)
	}
	operator, err := service.ValidateSession(session.Token)
	if err != nil || operator.ID != session.Operator.ID {
		t.Fatalf("session failed: %+v %v", operator, err)
	}
	admin, err := app.FindAuthRecordByEmail(core.CollectionNameSuperusers, "operator@example.com")
	if err != nil {
		t.Fatal(err)
	}
	token, err := admin.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ValidateSession(token); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatal("data administrator token accepted as operator session")
	}
}

func TestSetupRollsBackOperatorWhenAdminCreationFails(t *testing.T) {
	service, app := testAccounts(t)
	want := errors.New("admin storage failure")
	app.OnRecordCreate(core.CollectionNameSuperusers).BindFunc(func(e *core.RecordEvent) error { return want })
	if err := service.Setup("rollback@example.com", "account-test-password"); !errors.Is(err, want) {
		t.Fatalf("unexpected error: %v", err)
	}
	if count, err := app.CountRecords("operators"); err != nil || count != 0 {
		t.Fatalf("partial setup persisted: %d %v", count, err)
	}
}

func TestBootstrapRollsBackOnDuplicateAdmin(t *testing.T) {
	service, app := testAccounts(t)
	if err := createVerified(app, core.CollectionNameSuperusers, "existing@example.com", "account-test-password"); err != nil {
		t.Fatal(err)
	}
	if err := service.Bootstrap("existing@example.com", "account-test-password"); err == nil {
		t.Fatal("bootstrap ignored existing administrator")
	}
	if count, err := app.CountRecords("operators"); err != nil || count != 0 {
		t.Fatalf("partial bootstrap persisted: %d %v", count, err)
	}
}
