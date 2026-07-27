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
		Name:                    "203.0.113.8",
		Filename:                "default.conf",
		KeyType:                 certcrypto.RSA2048,
		MustStaple:              true,
		LegoDisableCNAMESupport: true,
		EnableCommonName:        true,
		RevokeOld:               true,
		Profile:                 "shortlived",
	}
	if err := db.Create(certModel).Error; err != nil {
		t.Fatal(err)
	}
	found, err := model.FirstOrCreateCert("default.conf", certcrypto.RSA2048)
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != certModel.ID {
		t.Fatalf("FirstOrCreateCert created duplicate ID %d, want %d", found.ID, certModel.ID)
	}

	err = persistAutoCertOptions(certModel, "default.conf", autoCertRequest{
		Domains:         []string{"203.0.113.8"},
		ChallengeMethod: model.CertChallengeMethodHTTP01,
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
	if got.Profile != "shortlived" {
		t.Fatalf("Profile = %q, want preserved shortlived", got.Profile)
	}
	if got.Name != "203.0.113.8" || got.Filename != "default.conf" {
		t.Fatalf("Name/Filename = %q/%q, want IP/config association", got.Name, got.Filename)
	}
	if got.KeyType != certcrypto.EC256 {
		t.Fatalf("KeyType = %s, want %s", got.KeyType, certcrypto.EC256)
	}
	if len(got.Domains) != 1 || got.Domains[0] != "203.0.113.8" {
		t.Fatalf("Domains = %#v, want IP identifier", got.Domains)
	}
}
