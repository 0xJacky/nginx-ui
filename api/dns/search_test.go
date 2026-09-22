package dns

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/0xJacky/Nginx-UI/model"
)

func TestExpandDomainSearchTerms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			// The list renders the decoded spelling, so searching it has to match the
			// punycode that is actually stored.
			name:  "unicode term also matches the stored punycode",
			input: []string{"例.example.com"},
			want:  []string{"例.example.com", "xn--fsq.example.com"},
		},
		{
			name:  "partial unicode label still converts",
			input: []string{"例"},
			want:  []string{"例", "xn--fsq"},
		},
		{
			name:  "punycode term is not duplicated",
			input: []string{"xn--fsq.example.com"},
			want:  []string{"xn--fsq.example.com"},
		},
		{
			name:  "ascii term is left alone",
			input: []string{"example.com"},
			want:  []string{"example.com"},
		},
		{
			name:  "uppercase ascii keeps the typed form and adds the lowered one",
			input: []string{"EXAMPLE"},
			want:  []string{"EXAMPLE", "example"},
		},
		{
			name:  "blanks are dropped",
			input: []string{"", "example.com", ""},
			want:  []string{"example.com"},
		},
		{
			name:  "repeated terms collapse",
			input: []string{"例", "例"},
			want:  []string{"例", "xn--fsq"},
		},
		{
			name:  "no terms yields nothing",
			input: nil,
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, expandDomainSearchTerms(tt.input))
		})
	}
}

func newSearchTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.DnsDomain{}))

	for _, domain := range []string{"xn--fsq.example.com", "example.com", "other.test"} {
		require.NoError(t, db.Create(&model.DnsDomain{Domain: domain, DnsCredentialID: 1}).Error)
	}

	return db
}

func newSearchContext(t *testing.T, rawQuery string) *gin.Context {
	t.Helper()

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/domains?"+rawQuery, nil)
	return c
}

func searchDomains(t *testing.T, db *gorm.DB, rawQuery string) []string {
	t.Helper()

	var found []model.DnsDomain
	err := db.Model(&model.DnsDomain{}).
		Scopes(domainSearchScope(newSearchContext(t, rawQuery))).
		Find(&found).Error
	require.NoError(t, err)

	domains := make([]string, 0, len(found))
	for _, item := range found {
		domains = append(domains, item.Domain)
	}
	return domains
}

// The scope has to produce working SQL, not just the right terms, so this drives it
// against a real database.
func TestDomainSearchScopeFindsPunycodeFromUnicodeQuery(t *testing.T) {
	db := newSearchTestDB(t)

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{"full unicode spelling", "domain=" + url.QueryEscape("例.example.com"), []string{"xn--fsq.example.com"}},
		{"partial unicode label", "domain=" + url.QueryEscape("例"), []string{"xn--fsq.example.com"}},
		{"stored punycode spelling", "domain=xn--fsq", []string{"xn--fsq.example.com"}},
		{"ascii term still works", "domain=other", []string{"other.test"}},
		{"shared ascii suffix matches both", "domain=example.com", []string{"xn--fsq.example.com", "example.com"}},
		{"repeated query key", "domain[]=" + url.QueryEscape("例") + "&domain[]=other", []string{"xn--fsq.example.com", "other.test"}},
		{"no term returns everything", "", []string{"xn--fsq.example.com", "example.com", "other.test"}},
		{"blank term returns everything", "domain=", []string{"xn--fsq.example.com", "example.com", "other.test"}},
		{"unmatched term returns nothing", "domain=nosuchzone", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ElementsMatch(t, tt.want, searchDomains(t, db, tt.query))
		})
	}
}

// The domain filter must not swallow other conditions on the same query.
func TestDomainSearchScopeComposesWithOtherConditions(t *testing.T) {
	db := newSearchTestDB(t)

	var found []model.DnsDomain
	err := db.Model(&model.DnsDomain{}).
		Where("domain LIKE ?", "%test%").
		Scopes(domainSearchScope(newSearchContext(t, "domain="+url.QueryEscape("例")))).
		Find(&found).Error
	require.NoError(t, err)
	require.Empty(t, found, "the two filters must AND together, not OR")
}
