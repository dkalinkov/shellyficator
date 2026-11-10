package accounts

import (
	"testing"

	"shellyficator/internal/domain"
)

type recordingRepository struct {
	email, password string
	calls           int
}

func (r *recordingRepository) NeedsSetup() (bool, error) { return true, nil }
func (r *recordingRepository) Setup(email, password string) error {
	r.email, r.password = email, password
	r.calls++
	return nil
}
func (r *recordingRepository) Bootstrap(email, password string) error {
	return r.Setup(email, password)
}
func (r *recordingRepository) Authenticate(email, password string) (domain.Session, error) {
	r.email, r.password = email, password
	return domain.Session{}, nil
}
func (*recordingRepository) ValidateSession(string) (domain.Operator, error) {
	return domain.Operator{}, nil
}

func (*recordingRepository) AuthenticateIdentity(domain.ExternalIdentity) (domain.Session, error) {
	return domain.Session{}, nil
}

func TestAccountServiceValidatesBeforePersistence(t *testing.T) {
	repository := &recordingRepository{}
	service := New(repository)
	for _, command := range []func(string, string) error{service.Setup, service.Bootstrap} {
		for _, input := range [][2]string{{" ", "account-test-password"}, {"operator@example.com", "short"}} {
			if err := command(input[0], input[1]); err == nil {
				t.Fatal("invalid account command accepted")
			}
		}
	}
	if repository.calls != 0 {
		t.Fatal("invalid command reached storage")
	}
	if err := service.Setup(" operator@example.com ", "account-test-password"); err != nil {
		t.Fatal(err)
	}
	if repository.email != "operator@example.com" || repository.calls != 1 {
		t.Fatal("service did not normalize setup identity")
	}
	if _, err := service.Authenticate(" operator@example.com ", " password with spaces "); err != nil {
		t.Fatal(err)
	}
	if repository.email != "operator@example.com" || repository.password != " password with spaces " {
		t.Fatal("authentication changed the password or failed to normalize email")
	}
}
