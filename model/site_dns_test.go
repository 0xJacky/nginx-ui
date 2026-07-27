package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSiteDNSRecordsJSONSerializer(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:site-dns-records?mode=memory&cache=shared"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Site{}))

	siteModel := &Site{
		Path: "/etc/nginx/sites-available/example",
		DNSRecords: []SiteDNSRecord{
			{ID: "apex", Name: "@", Type: "A", Exists: true},
			{ID: "www", Name: "www", Type: "A", Exists: false},
		},
	}
	require.NoError(t, db.Create(siteModel).Error)

	var savedSite Site
	require.NoError(t, db.First(&savedSite, siteModel.ID).Error)
	assert.Equal(t, siteModel.DNSRecords, savedSite.DNSRecords)
}
