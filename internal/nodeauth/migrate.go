package nodeauth

import (
	"fmt"

	"github.com/0xJacky/Nginx-UI/model"
	"gorm.io/gorm"
)

// MigrateLegacyNodeCredentials moves legacy controller secrets out of the
// plaintext nodes.token column after the authenticated-encryption key exists.
func MigrateLegacyNodeCredentials(database *gorm.DB) error {
	if database == nil {
		return nil
	}

	var nodes []model.Node
	if err := database.Unscoped().Where("token <> ''").Find(&nodes).Error; err != nil {
		return err
	}

	return database.Transaction(func(tx *gorm.DB) error {
		for _, node := range nodes {
			encrypted, err := EncryptPrivateCredential(LegacyCredentialPurpose(node.ID), []byte(node.Token))
			if err != nil {
				return fmt.Errorf("encrypt legacy credential for node %d: %w", node.ID, err)
			}
			if err := tx.Unscoped().Model(&model.Node{}).
				Where("id = ?", node.ID).
				Updates(map[string]any{
					"token":                   "",
					"encrypted_legacy_secret": encrypted,
					"auth_method":             model.NodeAuthMethodLegacy,
					"credential_status":       model.NodeCredentialStatusActive,
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
