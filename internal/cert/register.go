package cert

import (
	"context"
	"sync"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/pkg/errors"
	"github.com/uozi-tech/cosy/logger"
	"gorm.io/gorm"
)

var (
	// defaultACMEUserMutex serializes the lookup-or-register of the default
	// ACME user, so concurrent callers (boot, install, issuance) register the
	// account with the CA at most once.
	defaultACMEUserMutex sync.Mutex

	// registerACMEUser creates the account at the CA. Test seam.
	registerACMEUser = func(user *model.AcmeUser) error {
		return user.Register()
	}
)

// InitRegister registers the default ACME user (the certificate email and CA
// directory from the settings) unless it already exists. It runs at boot and
// after installation, and is safe to call more than once; failures are
// logged, and issuance registers the user lazily if it is still missing.
func InitRegister(ctx context.Context) {
	InitChallengeEnv()

	if settings.CertSettings.Email == "" {
		return
	}
	if _, err := EnsureDefaultACMEUser(); err != nil {
		logger.Error(err)
	}
}

// EnsureDefaultACMEUser returns the default ACME user for the current
// certificate email and CA directory, registering it with the CA and storing
// it first when it does not exist yet. Safe for concurrent and repeated calls.
func EnsureDefaultACMEUser() (*model.AcmeUser, error) {
	email := settings.CertSettings.Email
	if email == "" {
		return nil, errors.Wrap(gorm.ErrRecordNotFound, "no certificate email is configured for the default ACME user")
	}
	caDir := settings.CertSettings.GetCADir()

	defaultACMEUserMutex.Lock()
	defer defaultACMEUserMutex.Unlock()

	user, err := findACMEUser(email, caDir)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.Wrap(err, "find default ACME user")
	}

	user = &model.AcmeUser{
		Name:  "System Initial User",
		Email: email,
		CADir: caDir,
	}
	if err = registerACMEUser(user); err != nil {
		return nil, errors.Wrap(err, "register default ACME user")
	}
	if err = query.AcmeUser.Create(user); err != nil {
		return nil, errors.Wrap(err, "save default ACME user")
	}

	logger.Info("ACME Default User registered")
	return user, nil
}

// GetDefaultACMEUser returns the ACME user for the certificate email and CA
// directory in the settings. A missing user is registered on the spot, so a
// fresh installation or a changed CA directory does not need a restart.
func GetDefaultACMEUser() (user *model.AcmeUser, err error) {
	email := settings.CertSettings.Email
	caDir := settings.CertSettings.GetCADir()

	user, err = findACMEUser(email, caDir)
	if errors.Is(err, gorm.ErrRecordNotFound) && email != "" {
		user, err = EnsureDefaultACMEUser()
	}
	if err != nil {
		err = errors.Wrap(err, "get default user error")
		return nil, err
	}

	return user, nil
}

func findACMEUser(email, caDir string) (*model.AcmeUser, error) {
	u := query.AcmeUser
	return u.Where(u.Email.Eq(email), u.CADir.Eq(caDir)).First()
}
