package cluster

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy/logger"
	"gorm.io/gorm"
)

func RegisterPredefinedNodes(ctx context.Context) {
	if len(settings.ClusterSettings.Node) == 0 {
		return
	}

	for _, nodeUrl := range settings.ClusterSettings.Node {
		func() {
			node, err := parseNodeUrl(nodeUrl)
			if err != nil {
				logger.Error(sanitizePredefinedNodeURL(nodeUrl), err)
				return
			}

			if node.Name == "" {
				logger.Error(sanitizePredefinedNodeURL(nodeUrl), "Node name is required")
				return
			}

			if node.URL == "" {
				logger.Error(sanitizePredefinedNodeURL(nodeUrl), "Node URL is required")
				return
			}

			if node.Token == "" {
				logger.Error(sanitizePredefinedNodeURL(nodeUrl), "Node Token is required")
				return
			}
			logger.Warn("[Cluster] Node node_secret query configuration is deprecated; upgrade this relationship to paired authentication")

			if err := registerPredefinedNode(ctx, node); err != nil {
				logger.Error(node.URL, err)
			}
		}()
	}
}

func sanitizePredefinedNodeURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "[invalid cluster node URL redacted]"
	}
	query := parsed.Query()
	if query.Has("node_secret") {
		query.Set("node_secret", "[REDACTED]")
		parsed.RawQuery = query.Encode()
	}
	return parsed.String()
}

func registerPredefinedNode(ctx context.Context, predefined *model.Node) error {
	database := model.UseDB()
	if database == nil {
		return errors.New("database unavailable")
	}
	return database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.Node
		err := tx.Where("url = ?", predefined.URL).First(&existing).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		existing = model.Node{
			Name:             predefined.Name,
			URL:              predefined.URL,
			Enabled:          predefined.Enabled,
			AuthMethod:       model.NodeAuthMethodLegacy,
			CredentialStatus: model.NodeCredentialStatusActive,
		}
		if err := tx.Create(&existing).Error; err != nil {
			return err
		}
		encrypted, err := nodeauth.EncryptPrivateCredential(
			nodeauth.LegacyCredentialPurpose(existing.ID),
			[]byte(predefined.Token),
		)
		if err != nil {
			return err
		}
		return tx.Model(&existing).Update("encrypted_legacy_secret", encrypted).Error
	})
}

func parseNodeUrl(nodeUrl string) (node *model.Node, err error) {
	u, err := url.Parse(nodeUrl)
	if err != nil {
		return
	}
	var sb strings.Builder
	sb.WriteString(u.Scheme)
	sb.WriteString("://")
	sb.WriteString(u.Host)
	sb.WriteString(u.Path)

	node = &model.Node{
		Name:    u.Query().Get("name"),
		URL:     sb.String(),
		Token:   u.Query().Get("node_secret"),
		Enabled: u.Query().Get("enabled") == "true",
	}

	return
}
