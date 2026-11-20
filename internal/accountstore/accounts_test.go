package accountstore

import (
	"errors"
	"testing"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"shellyficator/internal/domain"
	_ "shellyficator/migrations"
)

func testAccounts(t *testing.T) (*Store, core.App) {
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
	return New(app), app
}

func TestSetupAndSessionUseOperatorAccounts(t *testing.T) {
	service, app := testAccounts(t)
	if needed, err := service.NeedsSetup(); err != nil || !needed {
		t.Fatalf("setup unavailable: %v %v", needed, err)
	}
	const password = " account-test-password "
	for _, create := range []func(string, string) error{service.Setup, service.Bootstrap} {
		for _, input := range [][2]string{{" ", password}, {"operator@example.com", "too-short!"}} {
			if err := create(input[0], input[1]); err == nil {
				t.Fatal("invalid setup credentials were accepted")
			}
		}
	}
	if err := service.Setup(" operator@example.com ", password); err != nil {
		t.Fatal(err)
	}
	if needed, err := service.NeedsSetup(); err != nil || needed {
		t.Fatalf("setup stayed open: %v %v", needed, err)
	}
	if err := service.Setup("second@example.com", password); !errors.Is(err, domain.ErrSetupCompleted) {
		t.Fatalf("duplicate setup allowed: %v", err)
	}
	if _, err := service.Authenticate("operator@example.com", "account-test-password"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("password whitespace was ignored: %v", err)
	}
	session, err := service.Authenticate(" operator@example.com ", password)
	if err != nil || session == "" {
		t.Fatalf("authentication failed: %v", err)
	}
	operator, err := app.FindAuthRecordByEmail("operators", "operator@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if id, err := service.ValidateSession(session); err != nil || id != operator.Id {
		t.Fatalf("session failed: %v", err)
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

func TestDashboardSignInDoesNotBypassOperatorAuthenticationPolicies(t *testing.T) {
	for _, test := range []struct {
		name      string
		configure func(*core.Collection)
		oidc      bool
	}{
		{"password disabled", func(c *core.Collection) { c.PasswordAuth.Enabled = false }, true},
		{"authentication locked", func(c *core.Collection) { c.AuthRule = nil }, false},
		{"custom authentication rule", func(c *core.Collection) { rule := "verified = true"; c.AuthRule = &rule }, false},
		{"MFA required", func(c *core.Collection) { c.OTP.Enabled = true; c.MFA.Enabled = true }, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, app := testAccounts(t)
			const email, password = "operator@example.com", "account-test-password"
			if err := service.Setup(email, password); err != nil {
				t.Fatal(err)
			}
			collection, err := app.FindCollectionByNameOrId("operators")
			if err != nil {
				t.Fatal(err)
			}
			test.configure(collection)
			if err := app.Save(collection); err != nil {
				t.Fatal(err)
			}
			if _, err := service.Authenticate(email, password); !errors.Is(err, domain.ErrInvalidCredentials) {
				t.Fatalf("password login bypassed authentication policy: %v", err)
			}
			identity := domain.ExternalIdentity{Issuer: "https://auth.example.com", Subject: "test-subject", Email: email, EmailVerified: true}
			session, err := service.AuthenticateIdentity(identity)
			if test.oidc {
				if err != nil || session == "" {
					t.Fatalf("disabling password authentication blocked OIDC: %v", err)
				}
				return
			}
			if !errors.Is(err, domain.ErrInvalidCredentials) {
				t.Fatalf("OIDC bypassed authentication policy: %v", err)
			}
			operator, err := app.FindAuthRecordByEmail("operators", email)
			if err != nil || operator.GetString("oidc_subject") != "" {
				t.Fatal("rejected OIDC login persisted an identity link")
			}
		})
	}
}
