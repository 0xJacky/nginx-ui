package sites

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-acme/lego/v5/certcrypto"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAutoCertTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Cert{}); err != nil {
		t.Fatal(err)
	}
	model.Use(db)
	t.Cleanup(func() { model.Use(nil) })

	return db
}

func TestPersistAutoCertOptionsWritesCommonNameAndClearsBooleans(t *testing.T) {
	db := setupAutoCertTestDB(t)
	certModel := &model.Cert{
		Name:                    "example.com",
		Filename:                "example.com",
		KeyType:                 certcrypto.RSA2048,
		MustStaple:              true,
		LegoDisableCNAMESupport: true,
		EnableCommonName:        true,
		RevokeOld:               true,
	}
	if err := db.Create(certModel).Error; err != nil {
		t.Fatal(err)
	}

	err := persistAutoCertOptions(certModel, "example.com", autoCertRequest{
		Domains:         []string{"example.com", "www.example.com"},
		ChallengeMethod: model.CertChallengeMethodDNS01,
		KeyType:         certcrypto.EC256,
		DnsCredentialID: 10,
		ACMEUserID:      20,
	})
	if err != nil {
		t.Fatal(err)
	}

	var got model.Cert
	if err := db.First(&got, certModel.ID).Error; err != nil {
		t.Fatal(err)
	}

	if got.EnableCommonName {
		t.Fatalf("EnableCommonName = true, want false")
	}
	if got.MustStaple {
		t.Fatalf("MustStaple = true, want false")
	}
	if got.LegoDisableCNAMESupport {
		t.Fatalf("LegoDisableCNAMESupport = true, want false")
	}
	if got.RevokeOld {
		t.Fatalf("RevokeOld = true, want false")
	}
	if got.KeyType != certcrypto.EC256 {
		t.Fatalf("KeyType = %s, want %s", got.KeyType, certcrypto.EC256)
	}
	if len(got.Domains) != 2 || got.Domains[0] != "example.com" || got.Domains[1] != "www.example.com" {
		t.Fatalf("Domains = %#v, want example.com and www.example.com", got.Domains)
	}
}
