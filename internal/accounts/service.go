package accounts

import (
	"errors"
	"strings"

	"shellyficator/internal/domain"
)

// Repository keeps account storage and PocketBase token handling in an adapter.
type Repository interface {
	NeedsSetup() (bool, error)
	Setup(email, password string) error
	Bootstrap(email, password string) error
	Authenticate(email, password string) (domain.Session, error)
	AuthenticateIdentity(domain.ExternalIdentity) (domain.Session, error)
	ValidateSession(token string) (domain.Operator, error)
}

type Service struct{ repository Repository }

func New(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) NeedsSetup() (bool, error) { return s.repository.NeedsSetup() }

func validate(email, password string) (string, error) {
	email = strings.TrimSpace(email)
	if email == "" || len(password) < 12 {
		return "", errors.New("provide an email and a password with at least 12 characters")
	}
	return email, nil
}

func (s *Service) Setup(email, password string) error {
	email, err := validate(email, password)
	if err != nil {
		return err
	}
	return s.repository.Setup(email, password)
}

// Bootstrap preserves the CLI's explicit creation of both initial accounts.
func (s *Service) Bootstrap(email, password string) error {
	email, err := validate(email, password)
	if err != nil {
		return err
	}
	return s.repository.Bootstrap(email, password)
}

func (s *Service) Authenticate(email, password string) (domain.Session, error) {
	return s.repository.Authenticate(strings.TrimSpace(email), password)
}

func (s *Service) ValidateSession(token string) (domain.Operator, error) {
	return s.repository.ValidateSession(token)
}

func (s *Service) AuthenticateIdentity(identity domain.ExternalIdentity) (domain.Session, error) {
	identity.Email = strings.TrimSpace(identity.Email)
	if identity.Issuer == "" || identity.Subject == "" || identity.Email == "" || !identity.EmailVerified {
		return domain.Session{}, domain.ErrInvalidCredentials
	}
	return s.repository.AuthenticateIdentity(identity)
}
