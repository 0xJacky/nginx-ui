package migrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cert/dns"
	"github.com/0xJacky/Nginx-UI/internal/crypto"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/registration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type legacyDnsCredential struct {
	model.Model
	Name         string      `json:"name"`
	Config       *dns.Config `json:"config,omitempty" gorm:"serializer:json"`
	Provider     string      `json:"provider"`
	ProviderCode string      `json:"provider_code" gorm:"index"`
}

func (legacyDnsCredential) TableName() string {
	return "dns_credentials"
}

type legacyAcmeUser struct {
	model.Model
	Name              string                `json:"name"`
	Email             string                `json:"email"`
	CADir             string                `json:"ca_dir"`
	Registration      registration.Resource `json:"registration" gorm:"serializer:json"`
	Key               model.PrivateKey      `json:"-" gorm:"serializer:json"`
	Proxy             string                `json:"proxy"`
	RegisterOnStartup bool                  `json:"register_on_startup"`
	EABKeyID          string                `json:"eab_key_id"`
	EABHMACKey        string                `json:"eab_hmac_key"`
}

func (legacyAcmeUser) TableName() string {
	return "acme_users"
}

type legacyCert struct {
	model.Model
	Name      string                     `json:"name"`
	Filename  string                     `json:"filename"`
	Resource  *model.CertificateResource `json:"-" gorm:"serializer:json"`
	KeyType   string                     `json:"key_type"`
	AutoCert  int                        `json:"auto_cert"`
	Log       string                     `json:"log"`
	Domains   []string                   `json:"domains" gorm:"serializer:json"`
	Challenge string                     `json:"challenge_method"`
}

func (legacyCert) TableName() string {
	return "certs"
}

func setupSensitiveFieldTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	settings.CryptoSettings.Secret = "test-secret"

	dbName := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	require.NoError(t, err)

	return db
}

func TestEncryptSensitiveJSONFieldsMigratesLegacyPlaintextData(t *testing.T) {
	db := setupSensitiveFieldTestDB(t)

	require.NoError(t, db.AutoMigrate(&legacyDnsCredential{}, &legacyAcmeUser{}, &legacyCert{}))

	legacyCredential := &legacyDnsCredential{
		Name:     "production",
		Provider: "cloudflare",
		Config: &dns.Config{
			Name: "Cloudflare",
			Code: "cloudflare",
			Configuration: &dns.Configuration{
				Credentials: map[string]string{
					"CF_API_TOKEN": "plaintext-token",
				},
			},
		},
	}
	require.NoError(t, db.Create(legacyCredential).Error)

	legacyUser := &legacyAcmeUser{
		Name:  "acme",
		Email: "admin@example.com",
		CADir: "https://acme-v02.api.letsencrypt.org/directory",
		Key: model.PrivateKey{
			X: big.NewInt(11),
			Y: big.NewInt(22),
			D: big.NewInt(33),
		},
	}
	require.NoError(t, db.Create(legacyUser).Error)

	legacyCert := &legacyCert{
		Name:     "example.com",
		Filename: "example.com",
		KeyType:  "2048",
		Resource: &model.CertificateResource{
			Resource: &certificate.Resource{
				Domain:        "example.com",
				CertURL:       "https://acme.example/cert",
				CertStableURL: "https://acme.example/cert/stable",
			},
			PrivateKey:        []byte("legacy-private-key"),
			Certificate:       []byte("legacy-certificate"),
			IssuerCertificate: []byte("legacy-issuer"),
			CSR:               []byte("legacy-csr"),
		},
	}
	require.NoError(t, db.Create(legacyCert).Error)

	require.NoError(t, EncryptSensitiveJSONFields.Migrate(db))

	var credentialRow dnsCredentialConfigRow
	require.NoError(t, db.Table("dns_credentials").Select("id", "config").First(&credentialRow, legacyCredential.ID).Error)
	assert.False(t, bytes.Contains(credentialRow.Config, []byte("plaintext-token")))

	var acmeRow acmeUserKeyRow
	require.NoError(t, db.Table("acme_users").Select("id", "key").First(&acmeRow, legacyUser.ID).Error)
	assert.False(t, bytes.Equal(bytes.TrimSpace(acmeRow.Key), []byte(`{"X":11,"Y":22,"D":33}`)))

	var certRow certResourceRow
	require.NoError(t, db.Table("certs").Select("id", "resource").First(&certRow, legacyCert.ID).Error)
	assert.False(t, bytes.Contains(certRow.Resource, []byte("legacy-private-key")))

	var migratedCredential model.DnsCredential
	require.NoError(t, db.First(&migratedCredential, legacyCredential.ID).Error)
	require.NotNil(t, migratedCredential.Config)
	require.NotNil(t, migratedCredential.Config.Configuration)
	assert.Equal(t, "plaintext-token", migratedCredential.Config.Configuration.Credentials["CF_API_TOKEN"])

	var migratedAcmeUser model.AcmeUser
	require.NoError(t, db.First(&migratedAcmeUser, legacyUser.ID).Error)
	assert.Zero(t, migratedAcmeUser.Key.X.Cmp(big.NewInt(11)))
	assert.Zero(t, migratedAcmeUser.Key.Y.Cmp(big.NewInt(22)))
	assert.Zero(t, migratedAcmeUser.Key.D.Cmp(big.NewInt(33)))

	var migratedCert model.Cert
	require.NoError(t, db.First(&migratedCert, legacyCert.ID).Error)
	require.NotNil(t, migratedCert.Resource)
	assert.Equal(t, []byte("legacy-private-key"), migratedCert.Resource.PrivateKey)
	assert.Equal(t, []byte("legacy-certificate"), migratedCert.Resource.Certificate)
}

func TestSensitiveModelsPersistEncryptedJSON(t *testing.T) {
	db := setupSensitiveFieldTestDB(t)

	require.NoError(t, db.AutoMigrate(&model.DnsCredential{}, &model.AcmeUser{}, &model.Cert{}))

	credential := &model.DnsCredential{
		Name:     "production",
		Provider: "cloudflare",
		Config: &dns.Config{
			Name: "Cloudflare",
			Code: "cloudflare",
			Configuration: &dns.Configuration{
				Credentials: map[string]string{
					"CF_API_TOKEN": "new-token",
				},
			},
		},
	}
	require.NoError(t, db.Create(credential).Error)

	acmeUser := &model.AcmeUser{
		Name:  "acme",
		Email: "admin@example.com",
		CADir: "https://acme-v02.api.letsencrypt.org/directory",
		Key: model.PrivateKey{
			X: big.NewInt(101),
			Y: big.NewInt(202),
			D: big.NewInt(303),
		},
	}
	require.NoError(t, db.Create(acmeUser).Error)

	certModel := &model.Cert{
		Name:     "example.com",
		Filename: "example.com",
		KeyType:  "2048",
		Resource: &model.CertificateResource{
			Resource: &certificate.Resource{
				Domain:        "example.com",
				CertURL:       "https://acme.example/cert",
				CertStableURL: "https://acme.example/cert/stable",
			},
			PrivateKey:        []byte("new-private-key"),
			Certificate:       []byte("new-certificate"),
			IssuerCertificate: []byte("new-issuer"),
			CSR:               []byte("new-csr"),
		},
	}
	require.NoError(t, db.Create(certModel).Error)

	var credentialRow dnsCredentialConfigRow
	require.NoError(t, db.Table("dns_credentials").Select("id", "config").First(&credentialRow, credential.ID).Error)
	plainCredential, err := json.Marshal(credential.Config)
	require.NoError(t, err)
	assert.False(t, bytes.Equal(bytes.TrimSpace(credentialRow.Config), plainCredential))

	decryptedCredential, err := crypto.AesDecrypt(append([]byte(nil), credentialRow.Config...))
	require.NoError(t, err)
	var storedCredential dns.Config
	require.NoError(t, json.Unmarshal(decryptedCredential, &storedCredential))
	require.NotNil(t, storedCredential.Configuration)
	assert.Equal(t, "new-token", storedCredential.Configuration.Credentials["CF_API_TOKEN"])

	var acmeRow acmeUserKeyRow
	require.NoError(t, db.Table("acme_users").Select("id", "key").First(&acmeRow, acmeUser.ID).Error)
	plainKey, err := json.Marshal(acmeUser.Key)
	require.NoError(t, err)
	assert.False(t, bytes.Equal(bytes.TrimSpace(acmeRow.Key), plainKey))

	decryptedKey, err := crypto.AesDecrypt(append([]byte(nil), acmeRow.Key...))
	require.NoError(t, err)
	var storedKey model.PrivateKey
	require.NoError(t, json.Unmarshal(decryptedKey, &storedKey))
	assert.Zero(t, storedKey.X.Cmp(big.NewInt(101)))
	assert.Zero(t, storedKey.Y.Cmp(big.NewInt(202)))
	assert.Zero(t, storedKey.D.Cmp(big.NewInt(303)))

	var certRow certResourceRow
	require.NoError(t, db.Table("certs").Select("id", "resource").First(&certRow, certModel.ID).Error)
	plainResource, err := json.Marshal(certModel.Resource)
	require.NoError(t, err)
	assert.False(t, bytes.Equal(bytes.TrimSpace(certRow.Resource), plainResource))

	decryptedResource, err := crypto.AesDecrypt(append([]byte(nil), certRow.Resource...))
	require.NoError(t, err)
	var storedResource model.CertificateResource
	require.NoError(t, json.Unmarshal(decryptedResource, &storedResource))
	assert.Equal(t, []byte("new-private-key"), storedResource.PrivateKey)
	assert.Equal(t, []byte("new-certificate"), storedResource.Certificate)
}
