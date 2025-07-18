package cache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/lang/en"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/uozi-tech/cosy/logger"
)

// SearchDocument represents a document in the search index
type SearchDocument struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`    // "site", "stream", or "config"
	Name      string    `json:"name"`    // extracted from filename
	Path      string    `json:"path"`    // file path
	Content   string    `json:"content"` // file content
	UpdatedAt time.Time `json:"updated_at"`
}

// SearchResult represents a search result
type SearchResult struct {
	Document SearchDocument `json:"document"`
	Score    float64        `json:"score"`
}

// SearchIndexer manages the Bleve search index
type SearchIndexer struct {
	index       bleve.Index
	indexPath   string
	indexMutex  sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	cleanupOnce sync.Once
}

var (
	searchIndexer     *SearchIndexer
	searchIndexerOnce sync.Once
)

// GetSearchIndexer returns the singleton search indexer instance
func GetSearchIndexer() *SearchIndexer {
	searchIndexerOnce.Do(func() {
		// Create a temporary directory for the index
		tempDir, err := os.MkdirTemp("", "nginx-ui-search-index-*")
		if err != nil {
			logger.Fatalf("Failed to create temp directory for search index: %v", err)
		}

		searchIndexer = &SearchIndexer{
			indexPath: tempDir,
		}
	})
	return searchIndexer
}

// InitSearchIndex initializes the search index
func InitSearchIndex(ctx context.Context) error {
	indexer := GetSearchIndexer()
	return indexer.Initialize(ctx)
}

// Initialize sets up the Bleve search index
func (si *SearchIndexer) Initialize(ctx context.Context) error {
	si.indexMutex.Lock()
	defer si.indexMutex.Unlock()

	// Create a derived context for cleanup
	si.ctx, si.cancel = context.WithCancel(ctx)

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Try to open existing index, create new if it fails
	var err error
	si.index, err = bleve.Open(si.indexPath)
	if err != nil {
		// Check context again before creating new index
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		logger.Info("Creating new search index at:", si.indexPath)
		si.index, err = bleve.New(si.indexPath, si.createIndexMapping())
		if err != nil {
			return fmt.Errorf("failed to create search index: %w", err)
		}
	}

	// Register callback for config scanning
	RegisterCallback(si.handleConfigScan)

	// Start cleanup goroutine
	go si.watchContext()

	logger.Info("Search index initialized successfully")
	return nil
}

// watchContext monitors the context and cleans up when it's cancelled
func (si *SearchIndexer) watchContext() {
	<-si.ctx.Done()
	si.cleanup()
}

// cleanup closes the index and removes the temporary directory
func (si *SearchIndexer) cleanup() {
	si.cleanupOnce.Do(func() {
		logger.Info("Cleaning up search index...")

		si.indexMutex.Lock()
		defer si.indexMutex.Unlock()

		if si.index != nil {
			si.index.Close()
			si.index = nil
		}

		// Remove the temporary directory
		if err := os.RemoveAll(si.indexPath); err != nil {
			logger.Error("Failed to remove search index directory:", err)
		} else {
			logger.Info("Search index directory removed successfully")
		}
	})
}

// createIndexMapping creates the mapping for the search index
func (si *SearchIndexer) createIndexMapping() mapping.IndexMapping {
	docMapping := bleve.NewDocumentMapping()

	// Text fields with standard analyzer
	textField := bleve.NewTextFieldMapping()
	textField.Analyzer = en.AnalyzerName
	textField.Store = true
	textField.Index = true

	// Keyword fields for exact match
	keywordField := bleve.NewKeywordFieldMapping()
	keywordField.Store = true
	keywordField.Index = true

	// Date field
	dateField := bleve.NewDateTimeFieldMapping()
	dateField.Store = true
	dateField.Index = true

	// Map fields to types
	fieldMappings := map[string]*mapping.FieldMapping{
		"id":         keywordField,
		"type":       keywordField,
		"path":       keywordField,
		"name":       textField,
		"content":    textField,
		"updated_at": dateField,
	}

	for field, fieldMapping := range fieldMappings {
		docMapping.AddFieldMappingsAt(field, fieldMapping)
	}

	indexMapping := bleve.NewIndexMapping()
	indexMapping.DefaultMapping = docMapping
	indexMapping.DefaultAnalyzer = en.AnalyzerName

	return indexMapping
}

// handleConfigScan processes scanned config files and indexes them
func (si *SearchIndexer) handleConfigScan(configPath string, content []byte) (err error) {
	// Add panic recovery to prevent the entire application from crashing
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic during config scan: %v", r)
			logger.Error("Panic occurred while scanning config", "config_path", configPath, "content_size", len(content), "error", err)
		}
	}()

	// File size limit: 10MB to prevent memory overflow
	const maxFileSize = 10 * 1024 * 1024 // 10MB
	if len(content) > maxFileSize {
		logger.Warn("Skipping file due to size limit", "path", configPath, "size", len(content), "limit", maxFileSize)
		return nil
	}

	// Skip empty files
	if len(content) == 0 {
		return nil
	}

	// Basic content validation: check if it's text content
	if !isTextContent(content) {
		logger.Warn("Skipping non-text file", "path", configPath)
		return nil
	}

	docType := si.determineConfigType(configPath)
	if docType == "" {
		return nil // Skip unsupported file types
	}

	doc := SearchDocument{
		ID:        configPath,
		Type:      docType,
		Name:      filepath.Base(configPath),
		Path:      configPath,
		Content:   string(content),
		UpdatedAt: time.Now(),
	}
	return si.IndexDocument(doc)
}

// determineConfigType determines the type of config file based on path
func (si *SearchIndexer) determineConfigType(configPath string) string {
	normalizedPath := filepath.ToSlash(configPath)

	switch {
	case strings.Contains(normalizedPath, "sites-available") || strings.Contains(normalizedPath, "sites-enabled"):
		return "site"
	case strings.Contains(normalizedPath, "streams-available") || strings.Contains(normalizedPath, "streams-enabled"):
		return "stream"
	default:
		return "config"
	}
}

// IndexDocument indexes a single document
func (si *SearchIndexer) IndexDocument(doc SearchDocument) (err error) {
	// Add panic recovery to prevent the entire application from crashing
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic during indexing: %v", r)
			logger.Error("Panic occurred while indexing document", "document_id", doc.ID, "error", err)
		}
	}()

	si.indexMutex.RLock()
	defer si.indexMutex.RUnlock()

	if si.index == nil {
		return fmt.Errorf("search index not initialized")
	}

	// Additional size check as a safety measure
	if len(doc.Content) > 50*1024*1024 { // 50MB absolute limit
		return fmt.Errorf("document content too large: %d bytes", len(doc.Content))
	}

	// logger.Debugf("Indexing document: ID=%s, Type=%s, Name=%s, Path=%s",
	// 	doc.ID, doc.Type, doc.Name, doc.Path)

	return si.index.Index(doc.ID, doc)
}

// Search performs a search query
func (si *SearchIndexer) Search(ctx context.Context, queryStr string, limit int) ([]SearchResult, error) {
	return si.searchWithType(ctx, queryStr, "", limit)
}

// SearchByType performs a search filtered by document type
func (si *SearchIndexer) SearchByType(ctx context.Context, queryStr string, docType string, limit int) ([]SearchResult, error) {
	return si.searchWithType(ctx, queryStr, docType, limit)
}

// searchWithType performs the actual search with optional type filtering
func (si *SearchIndexer) searchWithType(ctx context.Context, queryStr string, docType string, limit int) ([]SearchResult, error) {
	si.indexMutex.RLock()
	defer si.indexMutex.RUnlock()

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if si.index == nil {
		return nil, fmt.Errorf("search index not initialized")
	}

	if limit <= 0 {
		limit = 500 // Increase default limit to handle more results
	}

	query := si.buildQuery(queryStr, docType)
	searchRequest := bleve.NewSearchRequest(query)
	searchRequest.Size = limit
	searchRequest.Fields = []string{"*"}

	// Use a channel to handle search with context cancellation
	type searchResult struct {
		result *bleve.SearchResult
		err    error
	}

	resultChan := make(chan searchResult, 1)
	go func() {
		result, err := si.index.Search(searchRequest)
		resultChan <- searchResult{result: result, err: err}
	}()

	// Wait for search result or context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-resultChan:
		if res.err != nil {
			return nil, fmt.Errorf("search execution failed: %w", res.err)
		}
		results := si.convertResults(res.result)

		// Debug log the search execution
		logger.Debugf("Search index query '%s' (type: %s, limit: %d) returned %d results",
			queryStr, docType, limit, len(results))

		return results, nil
	}
}

// buildQuery builds a search query with optional type filtering
func (si *SearchIndexer) buildQuery(queryStr string, docType string) query.Query {
	mainQuery := bleve.NewBooleanQuery()

	// Add type filter if specified
	if docType != "" {
		typeQuery := bleve.NewTermQuery(docType)
		typeQuery.SetField("type")
		mainQuery.AddMust(typeQuery)
	}

	// Add text search across name and content fields only
	textQuery := bleve.NewBooleanQuery()
	searchFields := []string{"name", "content"}

	for _, field := range searchFields {
		// Create a boolean query for this field to combine multiple query types
		fieldQuery := bleve.NewBooleanQuery()

		// 1. Exact match query (highest priority)
		matchQuery := bleve.NewMatchQuery(queryStr)
		matchQuery.SetField(field)
		matchQuery.SetBoost(3.0) // Higher boost for exact matches
		fieldQuery.AddShould(matchQuery)

		// 2. Prefix query for partial matches (e.g., "access" matches "access_log")
		prefixQuery := bleve.NewPrefixQuery(queryStr)
		prefixQuery.SetField(field)
		prefixQuery.SetBoost(2.0) // Medium boost for prefix matches
		fieldQuery.AddShould(prefixQuery)

		// 3. Wildcard query for more flexible matching
		wildcardQuery := bleve.NewWildcardQuery("*" + queryStr + "*")
		wildcardQuery.SetField(field)
		wildcardQuery.SetBoost(1.5) // Lower boost for wildcard matches
		fieldQuery.AddShould(wildcardQuery)

		// 4. Fuzzy match query (allows 1 character difference)
		fuzzyQuery := bleve.NewFuzzyQuery(queryStr)
		fuzzyQuery.SetField(field)
		fuzzyQuery.SetFuzziness(1)
		fuzzyQuery.SetBoost(1.0) // Lowest boost for fuzzy matches
		fieldQuery.AddShould(fuzzyQuery)

		textQuery.AddShould(fieldQuery)
	}

	if docType != "" {
		mainQuery.AddMust(textQuery)
	} else {
		return textQuery
	}

	return mainQuery
}

// convertResults converts Bleve search results to our SearchResult format
func (si *SearchIndexer) convertResults(searchResult *bleve.SearchResult) []SearchResult {
	results := make([]SearchResult, 0, len(searchResult.Hits))

	for _, hit := range searchResult.Hits {
		doc := SearchDocument{
			ID:      si.getStringField(hit.Fields, "id"),
			Type:    si.getStringField(hit.Fields, "type"),
			Name:    si.getStringField(hit.Fields, "name"),
			Path:    si.getStringField(hit.Fields, "path"),
			Content: si.getStringField(hit.Fields, "content"),
		}

		// Parse updated_at if present
		if updatedAtStr := si.getStringField(hit.Fields, "updated_at"); updatedAtStr != "" {
			if updatedAt, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
				doc.UpdatedAt = updatedAt
			}
		}

		results = append(results, SearchResult{
			Document: doc,
			Score:    hit.Score,
		})
	}

	return results
}

// getStringField safely gets a string field from search results
func (si *SearchIndexer) getStringField(fields map[string]interface{}, fieldName string) string {
	if value, ok := fields[fieldName]; ok {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

// DeleteDocument removes a document from the index
func (si *SearchIndexer) DeleteDocument(docID string) error {
	si.indexMutex.RLock()
	defer si.indexMutex.RUnlock()

	if si.index == nil {
		return fmt.Errorf("search index not initialized")
	}

	return si.index.Delete(docID)
}

// RebuildIndex rebuilds the entire search index
func (si *SearchIndexer) RebuildIndex(ctx context.Context) error {
	si.indexMutex.Lock()
	defer si.indexMutex.Unlock()

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if si.index != nil {
		si.index.Close()
	}

	// Check context before removing old index
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Remove old index
	if err := os.RemoveAll(si.indexPath); err != nil {
		logger.Error("Failed to remove old index:", err)
	}

	// Check context before creating new index
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Create new index
	var err error
	si.index, err = bleve.New(si.indexPath, si.createIndexMapping())
	if err != nil {
		return fmt.Errorf("failed to create new index: %w", err)
	}

	logger.Info("Search index rebuilt successfully")
	return nil
}

// GetIndexStats returns statistics about the search index
func (si *SearchIndexer) GetIndexStats() (map[string]interface{}, error) {
	si.indexMutex.RLock()
	defer si.indexMutex.RUnlock()

	if si.index == nil {
		return nil, fmt.Errorf("search index not initialized")
	}

	docCount, err := si.index.DocCount()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"document_count": docCount,
		"index_path":     si.indexPath,
	}, nil
}

// Close closes the search index and triggers cleanup
func (si *SearchIndexer) Close() error {
	if si.cancel != nil {
		si.cancel()
	}

	si.cleanup()
	return nil
}

// Convenience functions for different search types

// SearchSites searches only site configurations
func SearchSites(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	return GetSearchIndexer().SearchByType(ctx, query, "site", limit)
}

// SearchStreams searches only stream configurations
func SearchStreams(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	return GetSearchIndexer().SearchByType(ctx, query, "stream", limit)
}

// SearchConfigs searches only general configurations
func SearchConfigs(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	return GetSearchIndexer().SearchByType(ctx, query, "config", limit)
}

// SearchAll searches across all configuration types
func SearchAll(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	return GetSearchIndexer().Search(ctx, query, limit)
}

// isTextContent checks if the content appears to be text-based
// This helps prevent indexing binary files that might have been misidentified
func isTextContent(content []byte) bool {
	if len(content) == 0 {
		return true // Empty content is considered text
	}

	// Check for common binary file signatures
	if len(content) >= 4 {
		// Check for some common binary file headers
		switch {
		case content[0] == 0x7F && content[1] == 0x45 && content[2] == 0x4C && content[3] == 0x46: // ELF
			return false
		case content[0] == 0x89 && content[1] == 0x50 && content[2] == 0x4E && content[3] == 0x47: // PNG
			return false
		case content[0] == 0xFF && content[1] == 0xD8 && content[2] == 0xFF: // JPEG
			return false
		case content[0] == 0x50 && content[1] == 0x4B && content[2] == 0x03 && content[3] == 0x04: // ZIP
			return false
		case content[0] == 0x50 && content[1] == 0x4B && content[2] == 0x05 && content[3] == 0x06: // ZIP (empty)
			return false
		case content[0] == 0x50 && content[1] == 0x4B && content[2] == 0x07 && content[3] == 0x08: // ZIP (spanned)
			return false
		}
	}

	// Check if the first part of the content contains mostly printable characters
	// Sample up to 8KB for performance
	sampleSize := len(content)
	if sampleSize > 8192 {
		sampleSize = 8192
	}

	nonPrintableCount := 0
	for i := 0; i < sampleSize; i++ {
		b := content[i]
		// Allow printable ASCII characters, newlines, tabs, and carriage returns
		if (b < 32 && b != 9 && b != 10 && b != 13) || b > 126 {
			nonPrintableCount++
		}
	}

	// If more than 30% of the sampled content is non-printable, consider it binary
	threshold := float64(sampleSize) * 0.3
	return float64(nonPrintableCount) <= threshold
}
