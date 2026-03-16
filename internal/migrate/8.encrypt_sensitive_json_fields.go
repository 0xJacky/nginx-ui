package migrate

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/0xJacky/Nginx-UI/internal/crypto"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var EncryptSensitiveJSONFields = &gormigrate.Migration{
	ID: "20260316000001",
	Migrate: func(tx *gorm.DB) error {
		if err := migrateDnsCredentialConfig(tx); err != nil {
			return err
		}

		if err := migrateAcmeUserKey(tx); err != nil {
			return err
		}

		if err := migrateCertResource(tx); err != nil {
			return err
		}

		return nil
	},
}

type dnsCredentialConfigRow struct {
	ID     uint64         `gorm:"column:id"`
	Config datatypes.JSON `gorm:"column:config"`
}

type acmeUserKeyRow struct {
	ID  uint64         `gorm:"column:id"`
	Key datatypes.JSON `gorm:"column:key"`
}

type certResourceRow struct {
	ID       uint64         `gorm:"column:id"`
	Resource datatypes.JSON `gorm:"column:resource"`
}

func migrateDnsCredentialConfig(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("dns_credentials") || !tx.Migrator().HasColumn("dns_credentials", "config") {
		return nil
	}

	var rows []dnsCredentialConfigRow
	if err := tx.Table("dns_credentials").Select("id", "config").Find(&rows).Error; err != nil {
		return err
	}

	for _, row := range rows {
		encrypted, changed, err := encryptJSONValueIfNeeded(row.Config)
		if err != nil {
			return fmt.Errorf("migrate dns_credentials.config for id %d: %w", row.ID, err)
		}
		if !changed {
			continue
		}

		if err := tx.Table("dns_credentials").
			Where("id = ?", row.ID).
			Update("config", string(encrypted)).Error; err != nil {
			return err
		}
	}

	return nil
}

func migrateAcmeUserKey(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("acme_users") || !tx.Migrator().HasColumn("acme_users", "key") {
		return nil
	}

	var rows []acmeUserKeyRow
	if err := tx.Table("acme_users").Select("id", "key").Find(&rows).Error; err != nil {
		return err
	}

	for _, row := range rows {
		encrypted, changed, err := encryptJSONValueIfNeeded(row.Key)
		if err != nil {
			return fmt.Errorf("migrate acme_users.key for id %d: %w", row.ID, err)
		}
		if !changed {
			continue
		}

		if err := tx.Table("acme_users").
			Where("id = ?", row.ID).
			Update("key", string(encrypted)).Error; err != nil {
			return err
		}
	}

	return nil
}

func migrateCertResource(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("certs") || !tx.Migrator().HasColumn("certs", "resource") {
		return nil
	}

	var rows []certResourceRow
	if err := tx.Table("certs").Select("id", "resource").Find(&rows).Error; err != nil {
		return err
	}

	for _, row := range rows {
		encrypted, changed, err := encryptJSONValueIfNeeded(row.Resource)
		if err != nil {
			return fmt.Errorf("migrate certs.resource for id %d: %w", row.ID, err)
		}
		if !changed {
			continue
		}

		if err := tx.Table("certs").
			Where("id = ?", row.ID).
			Update("resource", string(encrypted)).Error; err != nil {
			return err
		}
	}

	return nil
}

func encryptJSONValueIfNeeded(value []byte) ([]byte, bool, error) {
	trimmed := bytes.TrimSpace(value)
	if len(trimmed) == 0 {
		return nil, false, nil
	}

	if json.Valid(trimmed) {
		encrypted, err := crypto.AesEncrypt(value)
		if err != nil {
			return nil, false, err
		}
		return encrypted, true, nil
	}

	decrypted, err := crypto.AesDecrypt(append([]byte(nil), value...))
	if err == nil && json.Valid(bytes.TrimSpace(decrypted)) {
		return nil, false, nil
	}

	return nil, false, fmt.Errorf("value is neither plaintext JSON nor encrypted JSON")
}
