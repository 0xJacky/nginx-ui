package sites

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/clustersync"
	"github.com/0xJacky/Nginx-UI/internal/dns"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	"gorm.io/gorm/clause"
)

// buildProxyTargets processes proxy targets similar to list.go logic
func buildProxyTargets(fileName string) []site.ProxyTarget {
	indexedSite := site.GetIndexedSite(fileName)

	// Convert proxy targets, expanding upstream references
	var proxyTargets []site.ProxyTarget
	upstreamService := upstream.GetUpstreamService()

	for _, target := range indexedSite.ProxyTargets {
		// Check if target.Host is an upstream name
		if upstreamDef, exists := upstreamService.GetUpstreamDefinition(target.Host); exists {
			// Replace with upstream servers
			for _, server := range upstreamDef.Servers {
				proxyTargets = append(proxyTargets, site.ProxyTarget{
					Host: server.Host,
					Port: server.Port,
					Type: server.Type,
				})
			}
		} else {
			// Regular proxy target
			proxyTargets = append(proxyTargets, site.ProxyTarget{
				Host: target.Host,
				Port: target.Port,
				Type: target.Type,
			})
		}
	}

	return proxyTargets
}

// checkDNSRecordsExist verifies all linked records with a single provider request.
func checkDNSRecordsExist(domainID int, records []model.SiteDNSRecord) []model.SiteDNSRecord {
	checkedRecords := append([]model.SiteDNSRecord(nil), records...)
	if domainID == 0 || len(checkedRecords) == 0 {
		return checkedRecords
	}

	svc := dns.NewService()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	providerRecords, err := svc.ListRecords(ctx, uint64(domainID), dns.RecordListOptions{})
	if err != nil {
		logger.Warn("Failed to list DNS records:", err)
		return checkedRecords
	}

	existingRecordIDs := make(map[string]struct{}, len(providerRecords))
	for _, record := range providerRecords {
		existingRecordIDs[record.ID] = struct{}{}
	}
	for i := range checkedRecords {
		_, checkedRecords[i].Exists = existingRecordIDs[checkedRecords[i].ID]
	}

	return checkedRecords
}

func normalizeDNSRecords(records []model.SiteDNSRecord) []model.SiteDNSRecord {
	normalizedRecords := make([]model.SiteDNSRecord, 0, len(records))
	seenRecordIDs := make(map[string]struct{}, len(records))
	for _, record := range records {
		record.ID = strings.TrimSpace(record.ID)
		if record.ID == "" {
			continue
		}
		if _, exists := seenRecordIDs[record.ID]; exists {
			continue
		}
		seenRecordIDs[record.ID] = struct{}{}
		normalizedRecords = append(normalizedRecords, record)
	}
	return normalizedRecords
}

func getSiteDNSRecords(siteModel *model.Site) []model.SiteDNSRecord {
	if len(siteModel.DNSRecords) > 0 {
		return normalizeDNSRecords(siteModel.DNSRecords)
	}
	if siteModel.DNSRecordID == nil || *siteModel.DNSRecordID == "" {
		return nil
	}

	record := model.SiteDNSRecord{ID: *siteModel.DNSRecordID}
	if siteModel.DNSRecordName != nil {
		record.Name = *siteModel.DNSRecordName
	}
	if siteModel.DNSRecordType != nil {
		record.Type = *siteModel.DNSRecordType
	}
	if siteModel.DNSRecordExists != nil {
		record.Exists = *siteModel.DNSRecordExists
	}
	return []model.SiteDNSRecord{record}
}

func setSiteDNSRecords(siteModel *model.Site, domainID *int, records []model.SiteDNSRecord) {
	records = normalizeDNSRecords(records)
	if domainID == nil || len(records) == 0 {
		siteModel.DNSRecords = nil
		siteModel.DNSDomainID = nil
		siteModel.DNSRecordID = nil
		siteModel.DNSRecordName = nil
		siteModel.DNSRecordType = nil
		siteModel.DNSRecordExists = nil
		return
	}

	siteModel.DNSRecords = records
	siteModel.DNSDomainID = domainID
	firstRecord := records[0]
	siteModel.DNSRecordID = &firstRecord.ID
	siteModel.DNSRecordName = &firstRecord.Name
	siteModel.DNSRecordType = &firstRecord.Type
	siteModel.DNSRecordExists = &firstRecord.Exists
}

func GetSite(c *gin.Context) {
	name := helper.UnescapeURL(c.Param("name"))

	path, err := site.ResolveAvailablePath(name)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	file, err := os.Stat(path)
	if os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "file not found",
		})
		return
	}

	s := query.Site
	siteModel, err := s.Where(s.Path.Eq(path)).FirstOrCreate()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	certModel, err := model.FirstCert(name)
	if err != nil {
		logger.Warn(err)
	}

	// Check all DNS record links and migrate legacy single-record links on read.
	linkedRecords := getSiteDNSRecords(siteModel)
	if siteModel.DNSDomainID != nil && len(linkedRecords) > 0 {
		linkedRecords = checkDNSRecordsExist(*siteModel.DNSDomainID, linkedRecords)
		setSiteDNSRecords(siteModel, siteModel.DNSDomainID, linkedRecords)
		// Update in database
		if err := query.Site.Save(siteModel); err != nil {
			logger.Warn("Failed to update DNS record exists status:", err)
		}
	}

	if siteModel.Advanced {
		origContent, err := os.ReadFile(path)
		if err != nil {
			cosy.ErrHandler(c, err)
			return
		}

		c.JSON(http.StatusOK, site.Site{
			ModifiedAt:   file.ModTime(),
			Site:         siteModel,
			Name:         name,
			Config:       string(origContent),
			AutoCert:     certModel.AutoCert == model.AutoCertEnabled,
			Filepath:     path,
			Status:       site.GetSiteStatus(name),
			ProxyTargets: buildProxyTargets(name),
		})
		return
	}

	nginxConfig, err := nginx.ParseNgxConfig(path)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	certInfoMap := make(map[int][]*cert.Info)
	for serverIdx, server := range nginxConfig.Servers {
		for _, directive := range server.Directives {
			if directive.Directive == "ssl_certificate" {
				pubKey, err := cert.GetCertInfo(directive.Params)
				if err != nil {
					logger.Error("Failed to get certificate information", err)
					continue
				}
				certInfoMap[serverIdx] = append(certInfoMap[serverIdx], pubKey)
			}
		}
	}

	c.JSON(http.StatusOK, site.Site{
		Site:         siteModel,
		ModifiedAt:   file.ModTime(),
		Name:         name,
		Config:       nginxConfig.FmtCode(),
		Tokenized:    nginxConfig,
		AutoCert:     certModel.AutoCert == model.AutoCertEnabled,
		CertInfo:     certInfoMap,
		Filepath:     path,
		Status:       site.GetSiteStatus(name),
		ProxyTargets: buildProxyTargets(name),
	})
}

func SaveSite(c *gin.Context) {
	name := helper.UnescapeURL(c.Param("name"))

	var json struct {
		Content       string                 `json:"content" binding:"required"`
		NamespaceID   uint64                 `json:"namespace_id"`
		Namespace     string                 `json:"namespace"`
		SyncNodeIDs   []uint64               `json:"sync_node_ids"`
		Overwrite     bool                   `json:"overwrite"`
		PostAction    string                 `json:"post_action"`
		DNSDomainID   *int                   `json:"dns_domain_id"`
		DNSRecordID   *string                `json:"dns_record_id"`
		DNSRecordName *string                `json:"dns_record_name"`
		DNSRecordType *string                `json:"dns_record_type"`
		DNSRecords    *[]model.SiteDNSRecord `json:"dns_records"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	// A sync from another node identifies the namespace by name so both sides
	// group the site the same way even though their ids differ.
	namespaceID := json.NamespaceID
	if json.Namespace != "" {
		namespaceID = clustersync.ResolveNamespaceIDByName(json.Namespace)
	}

	err := site.Save(name, json.Content, json.Overwrite, namespaceID, json.SyncNodeIDs, json.PostAction)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Update DNS link information after file is saved
	path, err := site.ResolveAvailablePath(name)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	s := query.Site
	siteModel, err := s.Where(s.Path.Eq(path)).FirstOrCreate()
	if err != nil {
		logger.Warn("Failed to find or create site for DNS update:", err)
	} else {
		var linkedRecords []model.SiteDNSRecord
		if json.DNSRecords != nil {
			linkedRecords = normalizeDNSRecords(*json.DNSRecords)
		} else if json.DNSRecordID != nil {
			legacyRecord := model.SiteDNSRecord{ID: *json.DNSRecordID}
			if json.DNSRecordName != nil {
				legacyRecord.Name = *json.DNSRecordName
			}
			if json.DNSRecordType != nil {
				legacyRecord.Type = *json.DNSRecordType
			}
			linkedRecords = []model.SiteDNSRecord{legacyRecord}
		}

		if json.DNSDomainID != nil {
			linkedRecords = checkDNSRecordsExist(*json.DNSDomainID, linkedRecords)
		}
		setSiteDNSRecords(siteModel, json.DNSDomainID, linkedRecords)

		if err := s.Save(siteModel); err != nil {
			logger.Warn("Failed to save DNS link information:", err)
		}
	}

	GetSite(c)
}

func RenameSite(c *gin.Context) {
	oldName := helper.UnescapeURL(c.Param("name"))
	var json struct {
		NewName string `json:"new_name"`
	}
	if !cosy.BindAndValid(c, &json) {
		return
	}

	err := site.Rename(oldName, json.NewName)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func disableMaintenanceIfExists(name string) error {
	// Check if the site is in maintenance mode, if yes, disable maintenance mode first
	maintenanceConfigPath, err := site.ResolveEnabledPath(name + site.MaintenanceSuffix)
	if err != nil {
		return err
	}

	if _, err := os.Stat(maintenanceConfigPath); err == nil {
		// Site is in maintenance mode, disable it first
		err := site.DisableMaintenance(name)
		if err != nil {
			return err
		}
	}

	return nil
}

func enableSiteByName(name string) error {
	if err := disableMaintenanceIfExists(name); err != nil {
		return err
	}

	return site.Enable(name)
}

func disableSiteByName(name string) error {
	if err := disableMaintenanceIfExists(name); err != nil {
		return err
	}

	return site.Disable(name)
}

func EnableSite(c *gin.Context) {
	name := helper.UnescapeURL(c.Param("name"))

	err := enableSiteByName(name)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func DisableSite(c *gin.Context) {
	name := helper.UnescapeURL(c.Param("name"))

	err := disableSiteByName(name)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

type batchSiteNamesRequest struct {
	Names []string `json:"names" binding:"required,min=1"`
}

func BatchEnableSites(c *gin.Context) {
	var json batchSiteNamesRequest
	if !cosy.BindAndValid(c, &json) {
		return
	}

	for _, name := range json.Names {
		if err := enableSiteByName(name); err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func BatchDisableSites(c *gin.Context) {
	var json batchSiteNamesRequest
	if !cosy.BindAndValid(c, &json) {
		return
	}

	for _, name := range json.Names {
		if err := disableSiteByName(name); err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func DeleteSite(c *gin.Context) {
	err := site.Delete(helper.UnescapeURL(c.Param("name")))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}

func BatchUpdateSites(c *gin.Context) {
	cosy.Core[model.Site](c).SetValidRules(gin.H{
		"namespace_id": "required",
	}).SetItemKey("path").
		BeforeExecuteHook(func(ctx *cosy.Ctx[model.Site]) {
			effectedPath := make([]string, len(ctx.BatchEffectedIDs))
			var sites []*model.Site
			for i, name := range ctx.BatchEffectedIDs {
				path, err := site.ResolveAvailablePath(name)
				if err != nil {
					ctx.AbortWithError(err)
					return
				}

				effectedPath[i] = path
				sites = append(sites, &model.Site{
					Path: path,
				})
			}
			s := query.Site
			err := s.Clauses(clause.OnConflict{
				DoNothing: true,
			}).Create(sites...)
			if err != nil {
				ctx.AbortWithError(err)
				return
			}
			ctx.BatchEffectedIDs = effectedPath
		}).BatchModify()
}

func isInvalidSiteName(name string) bool {
	return name == "" || name == "." ||
		strings.ContainsAny(name, `/\`) || strings.Contains(name, "..")
}

func EnableMaintenanceSite(c *gin.Context) {
	name := helper.UnescapeURL(c.Param("name"))
	if isInvalidSiteName(name) {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "invalid site name",
		})
		return
	}

	// If site is already enabled, disable the normal site first
	enabledConfigPath, err := site.ResolveEnabledPath(name)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	if _, err := os.Stat(enabledConfigPath); err == nil {
		// Site is already enabled, disable normal site first
		err := site.Disable(name)
		if err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}

	// Then enable maintenance mode
	err = site.EnableMaintenance(name)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
