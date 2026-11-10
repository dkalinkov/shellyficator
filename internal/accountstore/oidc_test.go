package accountstore

import (
	"errors"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"shellyficator/internal/domain"
)

func TestOIDCLinksExistingOperatorAndPreservesPassword(t *testing.T) {
	service, app := testAccounts(t)
	const email, password = "operator@example.com", "account-test-password"
	if err := service.Setup(email, password); err != nil {
		t.Fatal(err)
	}
	passwordSession, err := service.Authenticate(email, password)
	if err != nil {
		t.Fatal(err)
	}
	identity := domain.ExternalIdentity{Issuer: "https://auth.example.com", Subject: "stable-subject", Email: email, EmailVerified: true}
	session, err := service.AuthenticateIdentity(identity)
	if err != nil || session.Operator.ID != passwordSession.Operator.ID {
		t.Fatalf("OIDC changed operator: %v", err)
	}
	if operator, err := service.ValidateSession(session.Token); err != nil || operator.ID != passwordSession.Operator.ID {
		t.Fatalf("invalid OIDC session: %v", err)
	}
	identity.Email = "changed@example.com"
	if session, err := service.AuthenticateIdentity(identity); err != nil || session.Operator.ID != passwordSession.Operator.ID {
		t.Fatalf("stable identity was lost after email change: %v", err)
	}
	if session, err := service.Authenticate(email, password); err != nil || session.Operator.ID != passwordSession.Operator.ID {
		t.Fatal("OIDC changed password authentication")
	}
	if count, err := app.CountRecords("operators"); err != nil || count != 1 {
		t.Fatal("OIDC created an operator")
	}
	if count, err := app.CountRecords(core.CollectionNameSuperusers); err != nil || count != 1 {
		t.Fatal("OIDC changed data administrators")
	}
}

func TestOIDCRejectsUnknownUnverifiedAndConflictingIdentities(t *testing.T) {
	service, app := testAccounts(t)
	if err := service.Setup("operator@example.com", "account-test-password"); err != nil {
		t.Fatal(err)
	}
	base := domain.ExternalIdentity{Issuer: "https://auth.example.com", Subject: "subject", Email: "operator@example.com", EmailVerified: true}
	for _, change := range []func(*domain.ExternalIdentity){
		func(i *domain.ExternalIdentity) { i.EmailVerified = false },
		func(i *domain.ExternalIdentity) { i.Email = "unknown@example.com" },
		func(i *domain.ExternalIdentity) { i.Subject = "" },
		func(i *domain.ExternalIdentity) { i.Issuer = "" },
	} {
		identity := base
		change(&identity)
		if _, err := service.AuthenticateIdentity(identity); !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Fatalf("untrusted identity accepted: %v", err)
		}
	}
	if err := createVerified(app, core.CollectionNameSuperusers, "admin-only@example.com", "account-test-password"); err != nil {
		t.Fatal(err)
	}
	adminIdentity := base
	adminIdentity.Email = "admin-only@example.com"
	if _, err := service.AuthenticateIdentity(adminIdentity); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatal("superuser was admitted as operator")
	}
	if _, err := service.AuthenticateIdentity(base); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*domain.ExternalIdentity){
		func(i *domain.ExternalIdentity) { i.Subject = "replacement-subject" },
		func(i *domain.ExternalIdentity) { i.Issuer = "https://other.example.com" },
	} {
		identity := base
		change(&identity)
		if _, err := service.AuthenticateIdentity(identity); !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Fatal("existing identity link was overwritten")
		}
	}
	unverified, err := app.FindAuthRecordByEmail("operators", "operator@example.com")
	if err != nil {
		t.Fatal(err)
	}
	unverified.Set("oidc_issuer", "")
	unverified.Set("oidc_subject", "")
	unverified.Set("verified", false)
	if err := app.Save(unverified); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AuthenticateIdentity(base); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatal("unverified local operator was linked")
	}
}
