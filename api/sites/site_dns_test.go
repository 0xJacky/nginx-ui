package sites

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeDNSRecords(t *testing.T) {
	records := normalizeDNSRecords([]model.SiteDNSRecord{
		{ID: " first ", Name: "@", Type: "A"},
		{ID: "first", Name: "duplicate", Type: "AAAA"},
		{ID: "", Name: "empty", Type: "A"},
		{ID: "second", Name: "www", Type: "A"},
	})

	require.Len(t, records, 2)
	assert.Equal(t, "first", records[0].ID)
	assert.Equal(t, "@", records[0].Name)
	assert.Equal(t, "second", records[1].ID)
}

func TestGetSiteDNSRecordsFallsBackToLegacyRecord(t *testing.T) {
	recordID := "legacy-id"
	recordName := "www"
	recordType := "A"
	recordExists := true
	siteModel := &model.Site{
		DNSRecordID:     &recordID,
		DNSRecordName:   &recordName,
		DNSRecordType:   &recordType,
		DNSRecordExists: &recordExists,
	}

	records := getSiteDNSRecords(siteModel)

	require.Len(t, records, 1)
	assert.Equal(t, model.SiteDNSRecord{
		ID:     recordID,
		Name:   recordName,
		Type:   recordType,
		Exists: true,
	}, records[0])
}

func TestSetSiteDNSRecordsMirrorsFirstRecordAndClearsLinks(t *testing.T) {
	domainID := 42
	siteModel := &model.Site{}
	records := []model.SiteDNSRecord{
		{ID: "apex", Name: "@", Type: "A", Exists: true},
		{ID: "www", Name: "www", Type: "A", Exists: false},
	}

	setSiteDNSRecords(siteModel, &domainID, records)

	assert.Equal(t, &domainID, siteModel.DNSDomainID)
	assert.Equal(t, records, siteModel.DNSRecords)
	require.NotNil(t, siteModel.DNSRecordID)
	assert.Equal(t, "apex", *siteModel.DNSRecordID)
	require.NotNil(t, siteModel.DNSRecordExists)
	assert.True(t, *siteModel.DNSRecordExists)

	setSiteDNSRecords(siteModel, &domainID, nil)

	assert.Nil(t, siteModel.DNSDomainID)
	assert.Nil(t, siteModel.DNSRecords)
	assert.Nil(t, siteModel.DNSRecordID)
	assert.Nil(t, siteModel.DNSRecordName)
	assert.Nil(t, siteModel.DNSRecordType)
	assert.Nil(t, siteModel.DNSRecordExists)
}
