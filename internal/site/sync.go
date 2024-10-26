package site

import (
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/samber/lo"
	"github.com/uozi-tech/cosy/logger"
)

func getSyncNodes(name string) (nodes []*model.Environment) {
	configFilePath := nginx.GetConfPath("sites-available", name)
	s := query.Site
	site, err := s.Where(s.Path.Eq(configFilePath)).
		Preload(s.SiteCategory).First()
	if err != nil {
		logger.Error(err)
		return
	}

	syncNodeIds := site.SyncNodeIDs
	// inherit sync node ids from site category
	if site.SiteCategory != nil {
		syncNodeIds = append(syncNodeIds, site.SiteCategory.SyncNodeIds...)
	}
	syncNodeIds = lo.Uniq(syncNodeIds)

	e := query.Environment
	nodes, err = e.Where(e.ID.In(syncNodeIds...)).Find()
	if err != nil {
		logger.Error(err)
		return
	}
	return
}
