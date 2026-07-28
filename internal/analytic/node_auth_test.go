package analytic

import (
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupLegacyNodeAuthForTest(t *testing.T, node *model.Node, secret string) {
	t.Helper()
	originalDatabase := model.UseDB()
	originalCryptoSecret := settings.CryptoSettings.Secret
	originalInstanceID := settings.NodeSettings.InstanceID
	t.Cleanup(func() {
		model.Use(originalDatabase)
		settings.CryptoSettings.Secret = originalCryptoSecret
		settings.NodeSettings.InstanceID = originalInstanceID
	})

	settings.CryptoSettings.Secret = "analytic-node-auth-test-root"
	settings.NodeSettings.InstanceID = "11111111-1111-4111-8111-111111111111"
	database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "node-auth.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.Node{}))
	model.Use(database)

	encryptedSecret, err := nodeauth.EncryptPrivateCredential(
		nodeauth.LegacyCredentialPurpose(node.ID),
		[]byte(secret),
	)
	require.NoError(t, err)
	node.Token = ""
	node.EncryptedLegacySecret = encryptedSecret
	node.AuthMethod = model.NodeAuthMethodLegacy
	node.CredentialStatus = model.NodeCredentialStatusActive
	require.NoError(t, database.Create(node).Error)
}
