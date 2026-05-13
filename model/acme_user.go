package model

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"math/big"

	"github.com/0xJacky/Nginx-UI/internal/transport"
	"github.com/go-acme/lego/v5/acme"
	"github.com/go-acme/lego/v5/lego"
	"github.com/go-acme/lego/v5/registration"
)

type PrivateKey struct {
	X, Y *big.Int
	D    *big.Int
}

type AcmeRegistration struct {
	Body acme.Account `json:"body"`
	URI  string       `json:"uri,omitempty"`
}

type AcmeUser struct {
	Model
	Name              string           `json:"name"`
	Email             string           `json:"email"`
	CADir             string           `json:"ca_dir"`
	Registration      AcmeRegistration `json:"registration" gorm:"serializer:json"`
	Key               PrivateKey       `json:"-" gorm:"serializer:json[aes]"`
	Proxy             string           `json:"proxy"`
	RegisterOnStartup bool             `json:"register_on_startup"`
	EABKeyID          string           `json:"eab_key_id"`
	EABHMACKey        string           `json:"eab_hmac_key"`
}

func (u *AcmeUser) GetEmail() string {
	return u.Email
}

func (u *AcmeUser) GetRegistration() *acme.ExtendedAccount {
	return &acme.ExtendedAccount{
		Account:  u.Registration.Body,
		Location: u.Registration.URI,
	}
}

func (u *AcmeUser) GetPrivateKey() crypto.Signer {
	return &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: elliptic.P256(),
			X:     u.Key.X,
			Y:     u.Key.Y,
		},
		D: u.Key.D,
	}
}
func (u *AcmeUser) Register() error {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	u.Key = PrivateKey{
		X: privateKey.PublicKey.X,
		Y: privateKey.PublicKey.Y,
		D: privateKey.D,
	}

	config := lego.NewConfig(u)
	config.CADirURL = u.CADir
	u.Registration = AcmeRegistration{}

	// Skip TLS check
	if config.HTTPClient != nil {
		t, err := transport.NewTransport(
			transport.WithProxy(u.Proxy))
		if err != nil {
			return err
		}
		config.HTTPClient.Transport = t
	}

	client, err := lego.NewClient(config)
	if err != nil {
		return err
	}

	// New users will need to register
	var reg *acme.ExtendedAccount
	ctx := context.Background()

	// Check if EAB credentials are provided
	if u.EABKeyID != "" && u.EABHMACKey != "" {
		// Register with External Account Binding
		reg, err = client.Registration.RegisterWithExternalAccountBinding(ctx, registration.RegisterEABOptions{
			TermsOfServiceAgreed: true,
			Kid:                  u.EABKeyID,
			HmacEncoded:          u.EABHMACKey,
		})
	} else {
		// Register without EAB
		reg, err = client.Registration.Register(ctx, registration.RegisterOptions{TermsOfServiceAgreed: true})
	}

	if err != nil {
		return err
	}

	u.Registration = AcmeRegistration{
		Body: reg.Account,
		URI:  reg.Location,
	}

	return nil
}
