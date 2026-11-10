package accountstore

import (
	"database/sql"
	"errors"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"shellyficator/internal/domain"
)

type Store struct{ app core.App }

func New(app core.App) *Store { return &Store{app: app} }

func (s *Store) NeedsSetup() (bool, error) {
	count, err := s.app.CountRecords("operators")
	return count == 0, err
}

func createVerified(app core.App, collectionName, email, password string) error {
	collection, err := app.FindCollectionByNameOrId(collectionName)
	if err != nil {
		return err
	}
	record := core.NewRecord(collection)
	record.Set("email", email)
	record.SetPassword(password)
	record.Set("verified", true)
	return app.Save(record)
}

func (s *Store) Setup(email, password string) error {
	return s.app.RunInTransaction(func(app core.App) error {
		count, err := app.CountRecords("operators")
		if err != nil {
			return err
		}
		if count > 0 {
			return domain.ErrSetupCompleted
		}
		if err := createVerified(app, "operators", email, password); err != nil {
			return err
		}
		count, err = app.CountRecords(core.CollectionNameSuperusers, dbx.Not(dbx.HashExp{"email": core.DefaultInstallerEmail}))
		if err != nil {
			return err
		}
		if count == 0 {
			if err := createVerified(app, core.CollectionNameSuperusers, email, password); err != nil {
				return err
			}
			installer, err := app.FindAuthRecordByEmail(core.CollectionNameSuperusers, core.DefaultInstallerEmail)
			if err == nil {
				return app.Delete(installer)
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
		return nil
	})
}

func (s *Store) Bootstrap(email, password string) error {
	return s.app.RunInTransaction(func(app core.App) error {
		for _, name := range []string{"operators", core.CollectionNameSuperusers} {
			if _, err := app.FindAuthRecordByEmail(name, email); err == nil {
				return errors.New("account already exists; use the data admin interface to manage it")
			} else if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if err := createVerified(app, name, email, password); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) Authenticate(email, password string) (domain.Session, error) {
	record, err := s.app.FindAuthRecordByEmail("operators", email)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Session{}, domain.ErrInvalidCredentials
	}
	if err != nil {
		return domain.Session{}, err
	}
	if !record.ValidatePassword(password) {
		return domain.Session{}, domain.ErrInvalidCredentials
	}
	return newSession(record)
}

func newSession(record *core.Record) (domain.Session, error) {
	token, err := record.NewAuthToken()
	if err != nil {
		return domain.Session{}, err
	}
	return domain.Session{Operator: domain.Operator{ID: record.Id, Email: record.Email()}, Token: token}, nil
}

// AuthenticateIdentity links only an existing verified operator. Once linked,
// issuer and subject are authoritative, including after an email address change.
// It never provisions an operator or grants access to the superuser collection.
func (s *Store) AuthenticateIdentity(identity domain.ExternalIdentity) (domain.Session, error) {
	if identity.Issuer == "" || identity.Subject == "" || identity.Email == "" || !identity.EmailVerified {
		return domain.Session{}, domain.ErrInvalidCredentials
	}
	var session domain.Session
	err := s.app.RunInTransaction(func(app core.App) error {
		record, err := app.FindFirstRecordByFilter("operators", "oidc_issuer = {:issuer} && oidc_subject = {:subject}", dbx.Params{"issuer": identity.Issuer, "subject": identity.Subject})
		if errors.Is(err, sql.ErrNoRows) {
			record, err = app.FindAuthRecordByEmail("operators", identity.Email)
			if errors.Is(err, sql.ErrNoRows) {
				return domain.ErrInvalidCredentials
			}
			if err != nil {
				return err
			}
			if !record.Verified() || record.GetString("oidc_issuer") != "" || record.GetString("oidc_subject") != "" {
				return domain.ErrInvalidCredentials
			}
			record.Set("oidc_issuer", identity.Issuer)
			record.Set("oidc_subject", identity.Subject)
			if err := app.Save(record); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		session, err = newSession(record)
		return err
	})
	return session, err
}

func (s *Store) ValidateSession(token string) (domain.Operator, error) {
	record, err := s.app.FindAuthRecordByToken(token, core.TokenTypeAuth)
	if err != nil || record.Collection().Name != "operators" {
		return domain.Operator{}, domain.ErrInvalidCredentials
	}
	return domain.Operator{ID: record.Id, Email: record.Email()}, nil
}
