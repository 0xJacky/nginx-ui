package searcher

import (
	"testing"
	
	"github.com/stretchr/testify/assert"
)

func TestMergeSingleFacet_UniqueTermsCount(t *testing.T) {
	ds := &DistributedSearcher{}
	
	// Create initial facet with some terms from shard 1
	existing := &Facet{
		Field:   "path_exact",
		Total:   3, // Initial count of unique terms
		Missing: 5,
		Other:   10,
		Terms: []*FacetTerm{
			{Term: "/api/users", Count: 100},
			{Term: "/api/posts", Count: 80},
			{Term: "/home", Count: 60},
		},
	}
	
	// Create incoming facet from shard 2
	// Has some overlapping terms and some new terms
	incoming := &Facet{
		Field:   "path_exact",
		Total:   4, // Should NOT be added to existing.Total
		Missing: 3,
		Other:   8,
		Terms: []*FacetTerm{
			{Term: "/api/users", Count: 50},  // Overlapping
			{Term: "/api/posts", Count: 30},  // Overlapping
			{Term: "/login", Count: 40},      // New
			{Term: "/dashboard", Count: 35},  // New
		},
	}
	
	// Merge the facets
	ds.mergeSingleFacet(existing, incoming)
	
	// Verify the results
	assert.Equal(t, "path_exact", existing.Field)
	
	// Total should be the count of unique terms after merging, NOT the sum
	// We have 5 unique terms: /api/users, /api/posts, /home, /login, /dashboard
	assert.Equal(t, 5, existing.Total, "Total should be the count of unique terms, not sum of totals")
	
	// Missing and Other should be summed
	assert.Equal(t, 8, existing.Missing)
	assert.Equal(t, 18, existing.Other)
	
	// Verify term counts are merged correctly
	termMap := make(map[string]int)
	for _, term := range existing.Terms {
		termMap[term.Term] = term.Count
	}
	
	assert.Equal(t, 150, termMap["/api/users"], "Count should be 100+50")
	assert.Equal(t, 110, termMap["/api/posts"], "Count should be 80+30")
	assert.Equal(t, 60, termMap["/home"], "Count should remain 60")
	assert.Equal(t, 40, termMap["/login"], "Count should be 40")
	assert.Equal(t, 35, termMap["/dashboard"], "Count should be 35")
}

func TestMergeSingleFacet_WithLimitAndOther(t *testing.T) {
	// DefaultFacetSize is 10 by default, so we'll use enough terms to exceed it
	ds := &DistributedSearcher{}
	
	existing := &Facet{
		Field:   "path",
		Total:   5,
		Missing: 0,
		Other:   0,
		Terms: []*FacetTerm{
			{Term: "/page1", Count: 1000},
			{Term: "/page2", Count: 900},
			{Term: "/page3", Count: 800},
			{Term: "/page4", Count: 700},
			{Term: "/page5", Count: 600},
		},
	}
	
	incoming := &Facet{
		Field:   "path",
		Total:   8,
		Missing: 0,
		Other:   0,
		Terms: []*FacetTerm{
			{Term: "/page1", Count: 500},
			{Term: "/page6", Count: 550},
			{Term: "/page7", Count: 450},
			{Term: "/page8", Count: 400},
			{Term: "/page9", Count: 350},
			{Term: "/page10", Count: 300},
			{Term: "/page11", Count: 250},
			{Term: "/page12", Count: 200},
		},
	}
	
	ds.mergeSingleFacet(existing, incoming)
	
	// Total should be 12 unique paths (page1-5 from existing, page6-12 from incoming)
	assert.Equal(t, 12, existing.Total, "Should have 12 unique paths")
	
	// DefaultFacetSize is 10, so we should keep top 10 terms
	assert.Equal(t, 10, len(existing.Terms), "Should keep top 10 terms (DefaultFacetSize)")
	
	// Verify top terms are correct
	assert.Equal(t, "/page1", existing.Terms[0].Term)
	assert.Equal(t, 1500, existing.Terms[0].Count) // 1000 + 500
	assert.Equal(t, "/page2", existing.Terms[1].Term)
	assert.Equal(t, 900, existing.Terms[1].Count)
	assert.Equal(t, "/page3", existing.Terms[2].Term)
	assert.Equal(t, 800, existing.Terms[2].Count)
	
	// Other should contain the sum of excluded terms (page11: 250, page12: 200)
	assert.Equal(t, 450, existing.Other, "Other should contain sum of excluded terms")
}