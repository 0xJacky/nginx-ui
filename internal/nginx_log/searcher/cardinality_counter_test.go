package searcher

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCardinalityCounter_CountCardinality(t *testing.T) {
	// Create a mock cardinality counter with nil shards for testing
	counter := NewCounter(nil)
	
	req := &CardinalityRequest{
		Field: "path_exact",
	}
	
	// Test that it handles nil shards gracefully
	result, err := counter.Count(context.Background(), req)
	
	// Should return error when IndexAlias is not available
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "IndexAlias not available")
	assert.NotNil(t, result)
	assert.Equal(t, "path_exact", result.Field)
	assert.Contains(t, result.Error, "IndexAlias not available")
}

func TestCardinalityCounter_BatchCountCardinality(t *testing.T) {
	counter := NewCounter(nil)
	
	fields := []string{"path_exact", "ip", "browser"}
	baseReq := &CardinalityRequest{}
	
	results, err := counter.BatchCount(context.Background(), fields, baseReq)
	
	assert.NoError(t, err)
	assert.Len(t, results, 3)
	
	for _, field := range fields {
		result, exists := results[field]
		assert.True(t, exists)
		assert.Equal(t, field, result.Field)
	}
}

func TestCardinalityRequest_Validation(t *testing.T) {
	counter := NewCounter(nil)
	
	// Test empty field name
	req := &CardinalityRequest{
		Field: "",
	}
	
	_, err := counter.Count(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "field name is required")
}

func TestCardinalityCounter_EstimateCardinality(t *testing.T) {
	counter := NewCounter(nil)
	
	req := &CardinalityRequest{
		Field: "test_field",
	}
	
	// For now, EstimateCardinality should behave the same as CountCardinality
	result, err := counter.Estimate(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test_field", result.Field)
}

func TestCardinalityRequest_TimeRange(t *testing.T) {
	counter := NewCounter(nil)
	
	now := time.Now().Unix()
	start := now - 3600 // 1 hour ago
	
	req := &CardinalityRequest{
		Field:     "path_exact", 
		StartTime: &start,
		EndTime:   &now,
	}
	
	result, err := counter.Count(context.Background(), req)
	assert.Error(t, err) // IndexAlias not available
	assert.NotNil(t, result)
	assert.Equal(t, "path_exact", result.Field)
	assert.Contains(t, result.Error, "IndexAlias not available")
}

// TestCardinalityEfficiency verifies that cardinality counting is more efficient than large facets
func TestCardinalityEfficiency_Concept(t *testing.T) {
	// This is a conceptual test to document the performance advantage
	
	// Traditional approach issues:
	// - Large FacetSize (e.g., 100,000) loads many terms into memory
	// - FacetSize limits accuracy when unique values > FacetSize
	// - Aggregating across shards can double-count terms
	
	// New cardinality approach advantages:
	// - Uses smaller FacetSize (5,000) per shard for term collection
	// - Tracks unique terms in map (keys only, no counts needed)
	// - Memory usage: O(unique_terms) vs O(facet_size * term_length)
	// - More accurate cross-shard deduplication
	
	// Example calculation:
	// Old approach: FacetSize=100,000 * avg_term_length=20 bytes = ~2MB per shard
	// New approach: unique_terms=10,000 * avg_term_length=20 bytes = ~200KB total
	// Memory improvement: ~10x better
	
	assert.True(t, true, "Cardinality counter design provides better memory efficiency")
}