package dns

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"

	dnsService "github.com/0xJacky/Nginx-UI/internal/dns"
	"github.com/0xJacky/Nginx-UI/model"
)

func ListDomains(c *gin.Context) {
	var params domainListQuery
	_ = c.ShouldBindQuery(&params)

	svc := dnsService.NewService()
	domains, total, err := svc.ListDomains(
		c.Request.Context(),
		dnsService.DomainListOptions{
			Page:          params.Page,
			PerPage:       params.PerPage,
			Keyword:       params.Keyword,
			DnsCredential: params.CredentialID,
		},
	)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	responses := lo.Map(domains, func(domain *model.DnsDomain, _ int) domainResponse {
		return newDomainResponse(domain)
	})

	c.JSON(http.StatusOK, gin.H{
		"data":       responses,
		"pagination": buildPagination(params.Page, params.PerPage, total),
	})
}

func GetDomain(c *gin.Context) {
	id := cast.ToUint64(c.Param("id"))
	svc := dnsService.NewService()

	domain, err := svc.GetDomain(c.Request.Context(), id)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, newDomainResponse(domain))
}

func CreateDomain(c *gin.Context) {
	var payload domainRequest
	if !cosy.BindAndValid(c, &payload) {
		return
	}

	svc := dnsService.NewService()
	domain, err := svc.CreateDomain(c.Request.Context(), dnsService.DomainInput{
		Domain:          payload.Domain,
		Description:     payload.Description,
		DnsCredentialID: payload.DnsCredentialID,
	})
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusCreated, newDomainResponse(domain))
}

func UpdateDomain(c *gin.Context) {
	var payload domainRequest
	if !cosy.BindAndValid(c, &payload) {
		return
	}

	id := cast.ToUint64(c.Param("id"))
	svc := dnsService.NewService()

	domain, err := svc.UpdateDomain(c.Request.Context(), id, dnsService.DomainInput{
		Domain:          payload.Domain,
		Description:     payload.Description,
		DnsCredentialID: payload.DnsCredentialID,
	})
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, newDomainResponse(domain))
}

func DeleteDomain(c *gin.Context) {
	id := cast.ToUint64(c.Param("id"))
	svc := dnsService.NewService()
	if err := svc.DeleteDomain(c.Request.Context(), id); err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.Status(http.StatusNoContent)
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
	start := (page - 1) * perPage
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}

	end := start + perPage
	if end > total {
		end = total
	}

	var pagedRecords []dnsService.Record
	if total == 0 {
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
