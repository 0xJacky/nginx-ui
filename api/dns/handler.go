package dns

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"

	"github.com/0xJacky/Nginx-UI/internal/cron"
	dnsService "github.com/0xJacky/Nginx-UI/internal/dns"
	"github.com/0xJacky/Nginx-UI/model"
)

func ListDomains(c *gin.Context) {
	cosy.Core[model.DnsDomain](c).
		SetPreloads("DnsCredential").
		SetFussy("domain", "description").
		PagingList()
}

func GetDomain(c *gin.Context) {
	cosy.Core[model.DnsDomain](c).
		SetPreloads("DnsCredential").
		Get()
}

func CreateDomain(c *gin.Context) {
	cosy.Core[model.DnsDomain](c).
		SetValidRules(gin.H{
			"domain":            "required",
			"description":       "omitempty",
			"dns_credential_id": "required",
		}).
		BeforeExecuteHook(domainMutationHook(dnsService.NewService(), false)).
		Create()
}

func UpdateDomain(c *gin.Context) {
	cosy.Core[model.DnsDomain](c).
		SetValidRules(gin.H{
			"domain":            "required",
			"description":       "omitempty",
			"dns_credential_id": "required",
		}).
		BeforeExecuteHook(domainMutationHook(dnsService.NewService(), true)).
		Modify()
}

func DeleteDomain(c *gin.Context) {
	cosy.Core[model.DnsDomain](c).Destroy()
}

func ListRecords(c *gin.Context) {
	domainID := cast.ToUint64(c.Param("id"))
	var params recordListQuery
	_ = c.ShouldBindQuery(&params)

	svc := dnsService.NewService()
	records, err := svc.ListRecords(
		c.Request.Context(),
		domainID,
		dnsService.RecordListOptions{
			Filter: dnsService.RecordFilter{
				Type: params.Type,
				Name: params.Name,
			},
		},
	)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	page := lo.If(params.Page < 1, 1).Else(params.Page)
	perPage := lo.If(params.PerPage <= 0, 50).Else(params.PerPage)

	total := len(records)
	start := max((page-1)*perPage, 0)
	end := min(start+perPage, total)

	var pagedRecords []dnsService.Record
	if total == 0 || start >= total {
		pagedRecords = []dnsService.Record{}
	} else {
		pagedRecords = records[start:end]
	}

	c.JSON(http.StatusOK, gin.H{
		"data":       pagedRecords,
		"pagination": buildPagination(page, perPage, int64(total)),
	})
}

func CreateRecord(c *gin.Context) {
	domainID := cast.ToUint64(c.Param("id"))
	var payload recordRequest
	if !cosy.BindAndValid(c, &payload) {
		return
	}

	svc := dnsService.NewService()
	record, err := svc.CreateRecord(c.Request.Context(), domainID, toRecordInput(payload))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusCreated, record)
}

func UpdateRecord(c *gin.Context) {
	domainID := cast.ToUint64(c.Param("id"))
	recordID := c.Param("record_id")

	var payload recordRequest
	if !cosy.BindAndValid(c, &payload) {
		return
	}

	svc := dnsService.NewService()
	record, err := svc.UpdateRecord(c.Request.Context(), domainID, recordID, toRecordInput(payload))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, record)
}

func DeleteRecord(c *gin.Context) {
	domainID := cast.ToUint64(c.Param("id"))
	recordID := c.Param("record_id")

	svc := dnsService.NewService()
	if err := svc.DeleteRecord(c.Request.Context(), domainID, recordID); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// GetDDNSConfig returns the DDNS configuration for a domain.
func GetDDNSConfig(c *gin.Context) {
	domainID := cast.ToUint64(c.Param("id"))
	svc := dnsService.NewService()

	cfg, err := svc.GetDDNSConfig(c.Request.Context(), domainID)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, toDDNSResponse(cfg))
}

// ListDDNSConfig returns DDNS overview for all domains.
func ListDDNSConfig(c *gin.Context) {
	ctx := c.Request.Context()
	domains, err := dnsService.ListDDNSDomains(ctx)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	items := make([]ddnsDomainItem, 0, len(domains))
	for _, domain := range domains {
		cfg := domain.DDNSConfig
		if cfg == nil {
			cfg = &model.DDNSConfig{
				Enabled:         false,
				IntervalSeconds: dnsService.DefaultDDNSInterval(),
				Targets:         []model.DDNSRecordTarget{},
			}
		} else if cfg.IntervalSeconds <= 0 {
			cfg.IntervalSeconds = dnsService.DefaultDDNSInterval()
		}

		credName := ""
		credProvider := ""
		if domain.DnsCredential != nil {
			credName = domain.DnsCredential.Name
			credProvider = domain.DnsCredential.Provider
		}

		item := ddnsDomainItem{
			ID:                 domain.ID,
			Domain:             domain.Domain,
			CredentialName:     credName,
			CredentialProvider: credProvider,
			Config:             toDDNSResponse(cfg),
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"data": items,
	})
}

// UpdateDDNSConfig updates DDNS settings for a domain and restarts its schedule.
func UpdateDDNSConfig(c *gin.Context) {
	domainID := cast.ToUint64(c.Param("id"))

	var payload ddnsConfigRequest
	if !cosy.BindAndValid(c, &payload) {
		return
	}

	if payload.Enabled && len(payload.RecordIDs) == 0 {
		cosy.ErrHandler(c, dnsService.ErrDDNSTargetRequired)
		return
	}

	svc := dnsService.NewService()
	cfg, err := svc.UpdateDDNSConfig(c.Request.Context(), domainID, dnsService.DDNSUpdateInput{
		Enabled:         payload.Enabled,
		IntervalSeconds: payload.IntervalSeconds,
		RecordIDs:       payload.RecordIDs,
	})
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	if cfg.Enabled {
		if err := cron.AddOrUpdateDDNSJob(domainID, cfg.IntervalSeconds); err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	} else {
		if err := cron.RemoveDDNSJob(domainID); err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}

	c.JSON(http.StatusOK, toDDNSResponse(cfg))
}

func buildPagination(page, perPage int, total int64) model.Pagination {
	page = lo.If(page < 1, 1).Else(page)
	perPage = lo.If(perPage <= 0, 50).Else(perPage)

	totalPages := total / cast.ToInt64(perPage)
	if total%cast.ToInt64(perPage) != 0 {
		totalPages++
	}

	return model.Pagination{
		Total:       total,
		PerPage:     perPage,
		CurrentPage: page,
		TotalPages:  totalPages,
	}
}

func domainMutationHook(svc *dnsService.Service, isUpdate bool) func(ctx *cosy.Ctx[model.DnsDomain]) {
	return func(ctx *cosy.Ctx[model.DnsDomain]) {
		normalized, err := dnsService.NormalizeDomain(ctx.Model.Domain)
		if err != nil {
			ctx.AbortWithError(err)
			return
		}

		credential, err := dnsService.LoadCredential(ctx.Request.Context(), ctx.Model.DnsCredentialID)
		if err != nil {
			ctx.AbortWithError(err)
			return
		}

		excludeID := uint64(0)
		if isUpdate {
			if ctx.ID == 0 {
				ctx.ID = ctx.GetParamID()
			}
			excludeID = ctx.ID
		}

		if err := dnsService.EnsureDomainUnique(ctx.Request.Context(), normalized, credential.ID, excludeID); err != nil {
			ctx.AbortWithError(err)
			return
		}

		ctx.Model.Domain = normalized
		ctx.Model.Description = strings.TrimSpace(ctx.Model.Description)
		ctx.Model.DnsCredentialID = credential.ID
	}
}
