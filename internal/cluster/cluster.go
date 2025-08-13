package cluster

import (
	"context"
	"net/url"

	"strings"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy/logger"
	"gorm.io/gen/field"
)

func RegisterPredefinedNodes(ctx context.Context) {
	if len(settings.ClusterSettings.Node) == 0 {
		return
	}

	q := query.Node
	for _, nodeUrl := range settings.ClusterSettings.Node {
		func() {
			node, err := parseNodeUrl(nodeUrl)
			if err != nil {
				logger.Error(nodeUrl, err)
				return
			}

			if node.Name == "" {
				logger.Error(nodeUrl, "Node name is required")
				return
			}

			if node.URL == "" {
				logger.Error(nodeUrl, "Node URL is required")
				return
			}

			if node.Token == "" {
				logger.Error(nodeUrl, "Node Token is required")
				return
			}

			_, err = q.Where(q.URL.Eq(node.URL)).
				Attrs(field.Attrs(node)).
				FirstOrCreate()
			if err != nil {
				logger.Error(node.URL, err)
			}
		}()
	}
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
