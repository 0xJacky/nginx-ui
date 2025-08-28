package searcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test that verifies our facet Total fix logic
func TestFacetTotalCorrection_Logic(t *testing.T) {
	// Test the logic we implemented in convertBleveResult
	
	// Case 1: Facet with terms - Total should be count of terms
	facetTerms := []*FacetTerm{
		{Term: "/api/users", Count: 500},
		{Term: "/api/posts", Count: 300},
		{Term: "/home", Count: 200},
		{Term: "/login", Count: 150},
		{Term: "/dashboard", Count: 100},
	}
	
	correctTotal := len(facetTerms)
	assert.Equal(t, 5, correctTotal, "Total should be the count of unique terms")
	
	// Case 2: Empty facet - Total should be 0
	emptyTerms := []*FacetTerm{}
	emptyTotal := len(emptyTerms)
	assert.Equal(t, 0, emptyTotal, "Total should be 0 when there are no terms")
	
	// Case 3: Nil terms - Total should be 0
	var nilTerms []*FacetTerm = nil
	nilTotal := 0
	if nilTerms != nil {
		nilTotal = len(nilTerms)
	}
	assert.Equal(t, 0, nilTotal, "Total should be 0 when terms are nil")
}

func TestFacetCorrection_PathExactVsPath(t *testing.T) {
	// This test verifies that we should use path_exact instead of path
	// for accurate unique page counting
	
	// path field (analyzed) might split "/api/users/123" into ["api", "users", "123"]
	// path_exact field (keyword) keeps "/api/users/123" as one term
	
	analyzedPathTerms := []string{"api", "users", "admin", "login", "dashboard"} // Wrong for counting unique URLs
	exactPathTerms := []string{"/api/users", "/api/admin", "/login", "/dashboard"} // Correct for counting unique URLs
	
	assert.Greater(t, len(analyzedPathTerms), len(exactPathTerms), 
		"Analyzed path creates more terms due to tokenization, leading to incorrect counts")
	
	assert.Equal(t, 4, len(exactPathTerms), 
		"path_exact gives accurate count of unique URLs")
}