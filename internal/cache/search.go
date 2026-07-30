package cache

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
	indexapi "github.com/blevesearch/bleve_index_api"
	"github.com/gabriel-vasile/mimetype"
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

	// Memory management
	totalContentSize int64
	documentCount    int64
	maxMemoryUsage   int64
	documentSizes    map[string]int64
	memoryMutex      sync.RWMutex
}

var (
	searchIndexer     *SearchIndexer
	searchIndexerOnce sync.Once
)

// GetSearchIndexer returns the singleton search indexer instance
func GetSearchIndexer() *SearchIndexer {
	searchIndexerOnce.Do(func() {
		searchIndexer = &SearchIndexer{
			indexPath:      "memory",
			maxMemoryUsage: 100 * 1024 * 1024, // 100MB memory limit for indexed content
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

	var err error
	logger.Info("Creating in-memory search index")
	si.index, err = bleve.NewMemOnly(si.createIndexMapping())
	if err != nil {
		return fmt.Errorf("failed to create in-memory search index: %w", err)
	}
	si.resetMemoryUsage()

	// Register callback for config scanning
	RegisterCallback("search.handleConfigScan", si.handleConfigScan)

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

// cleanup closes the in-memory index and resets memory accounting.
func (si *SearchIndexer) cleanup() {
	si.cleanupOnce.Do(func() {
		logger.Info("Cleaning up search index...")

		si.indexMutex.Lock()
		defer si.indexMutex.Unlock()

		if si.index != nil {
			si.index.Close()
			si.index = nil
		}

		// Reset memory tracking
		si.memoryMutex.Lock()
		si.totalContentSize = 0
		si.documentCount = 0
		si.documentSizes = nil
		si.memoryMutex.Unlock()
	})
}

// createIndexMapping creates the mapping for the search index
func (si *SearchIndexer) createIndexMapping() mapping.IndexMapping {
	docMapping := bleve.NewDocumentMapping()
	docMapping.Dynamic = false

	textField := bleve.NewTextFieldMapping()
	textField.Analyzer = "standard"
	textField.Store = true
	textField.Index = true
	textField.DocValues = false
	textField.IncludeTermVectors = false
	textField.IncludeInAll = false

	keywordField := bleve.NewKeywordFieldMapping()
	keywordField.Store = true
	keywordField.Index = true
	keywordField.DocValues = false
	keywordField.IncludeTermVectors = false
	keywordField.IncludeInAll = false

	storedKeywordField := bleve.NewKeywordFieldMapping()
	storedKeywordField.Store = true
	storedKeywordField.Index = false
	storedKeywordField.DocValues = false
	storedKeywordField.IncludeTermVectors = false
	storedKeywordField.IncludeInAll = false

	idField := bleve.NewKeywordFieldMapping()
	idField.Store = false
	idField.Index = false
	idField.DocValues = false
	idField.IncludeTermVectors = false
	idField.IncludeInAll = false

	dateField := bleve.NewDateTimeFieldMapping()
	dateField.Store = true
	dateField.Index = false
	dateField.DocValues = false
	dateField.IncludeTermVectors = false
	dateField.IncludeInAll = false

	fieldMappings := map[string]*mapping.FieldMapping{
		"id":         idField,
		"type":       keywordField,
		"path":       storedKeywordField,
		"name":       textField,
		"content":    textField,
		"updated_at": dateField,
	}

	for field, fieldMapping := range fieldMappings {
		docMapping.AddFieldMappingsAt(field, fieldMapping)
	}

	indexMapping := bleve.NewIndexMapping()
	indexMapping.DefaultMapping = docMapping
	indexMapping.DefaultAnalyzer = "standard"
	indexMapping.DefaultField = "content"
	indexMapping.IndexDynamic = false
	indexMapping.StoreDynamic = false
	indexMapping.DocValuesDynamic = false

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

	// File size limit: 1MB to prevent memory overflow and improve performance
	const maxFileSize = 1024 * 1024 // 1MB
	if len(content) > maxFileSize {
		return nil
	}

	// Empty content is emitted by the scanner when a config is removed.
	if len(content) == 0 {
		return si.DeleteDocument(configPath)
	}

	// Basic content validation: check if it's a configuration file
	if !isConfigFile(content) {
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

	// Additional size check as a safety measure
	if len(doc.Content) > 2*1024*1024 { // 2MB absolute limit
		return fmt.Errorf("document content too large: %d bytes", len(doc.Content))
	}

	si.indexMutex.Lock()
	defer si.indexMutex.Unlock()

	if si.index == nil {
		return fmt.Errorf("search index not initialized")
	}

	// Check if document already exists in the index
	contentSize := int64(len(doc.Content))
	existingDoc, err := si.index.Document(doc.ID)
	isNewDocument := err != nil || existingDoc == nil
	if !isNewDocument {
		if existingContent, ok := documentStringField(existingDoc, "content"); ok && existingContent == doc.Content {
			return nil
		}
	}

	si.memoryMutex.Lock()
	defer si.memoryMutex.Unlock()
	if si.documentSizes == nil {
		si.documentSizes = make(map[string]int64)
	}
	previousSize := si.documentSizes[doc.ID]
	newTotalSize := si.totalContentSize - previousSize + contentSize
	newDocumentCount := si.documentCount
	if isNewDocument {
		newDocumentCount++
	}
	if newTotalSize > si.maxMemoryUsage || newDocumentCount > 1000 {
		logger.Warn("Skipping document due to content budget",
			"document_id", doc.ID,
			"content_size", contentSize,
			"content_budget", si.maxMemoryUsage)
		return nil
	}

	// Index the document (this will update existing or create new)
	err = si.index.Index(doc.ID, doc)
	if err != nil {
		return err
	}

	si.totalContentSize = newTotalSize
	si.documentCount = newDocumentCount
	si.documentSizes[doc.ID] = contentSize

	return nil
}

func documentStringField(doc indexapi.Document, name string) (string, bool) {
	if doc == nil {
		return "", false
	}

	var value string
	var found bool
	doc.VisitFields(func(field indexapi.Field) {
		if found {
			return
		}
		if field.Name() == name {
			value = string(field.Value())
			found = true
		}
	})

	return value, found
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

		// log the search execution
		logger.Debugf("Search index query '%s' (type: %s, limit: %d) returned %d results",
			queryStr, docType, limit, len(results))

		return results, nil
	}
}

// isNumericQuery checks if the query string is primarily numeric
// This helps us apply different search strategies for numbers vs text
func isNumericQuery(queryStr string) bool {
	if len(queryStr) == 0 {
		return false
	}

	// Count numeric characters
	numericCount := 0
	for _, ch := range queryStr {
		if ch >= '0' && ch <= '9' {
			numericCount++
		}
	}

	// If more than 50% of characters are digits, treat as numeric query
	// This handles cases like "9005", "port:9005", "192.168.1.1", etc.
	return float64(numericCount)/float64(len(queryStr)) > 0.5
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

	// Determine if this is a numeric query
	isNumeric := isNumericQuery(queryStr)

	// Add text search across name and content fields only
	textQuery := bleve.NewBooleanQuery()
	searchFields := []string{"name", "content"}

	for _, field := range searchFields {
		// Create a boolean query for this field to combine multiple query types
		fieldQuery := bleve.NewBooleanQuery()

		if isNumeric {
			// Numeric query strategy: prioritize exact matches and prefix matches
			// Avoid fuzzy matching to prevent false positives

			// 1. Term query for exact token match (highest priority for numbers)
			termQuery := bleve.NewTermQuery(queryStr)
			termQuery.SetField(field)
			termQuery.SetBoost(10.0) // Highest boost for exact term matches
			fieldQuery.AddShould(termQuery)

			// 2. Prefix query for partial matches (e.g., "9005" matches "90051234")
			prefixQuery := bleve.NewPrefixQuery(queryStr)
			prefixQuery.SetField(field)
			prefixQuery.SetBoost(5.0) // High boost for prefix matches
			fieldQuery.AddShould(prefixQuery)

			// 3. Wildcard query for substring matching (e.g., "9005" in "listen 9005;")
			wildcardQuery := bleve.NewWildcardQuery("*" + queryStr + "*")
			wildcardQuery.SetField(field)
			wildcardQuery.SetBoost(2.0) // Lower boost for wildcard matches
			fieldQuery.AddShould(wildcardQuery)

		} else {
			// Text query strategy: more flexible matching with fuzzy support

			// 1. Term query for exact token match (highest priority)
			termQuery := bleve.NewTermQuery(strings.ToLower(queryStr))
			termQuery.SetField(field)
			termQuery.SetBoost(8.0) // High boost for exact matches
			fieldQuery.AddShould(termQuery)

			// 2. Match query for analyzed text search (handles case-insensitive, etc.)
			matchQuery := bleve.NewMatchQuery(queryStr)
			matchQuery.SetField(field)
			matchQuery.SetBoost(4.0) // Medium-high boost for match queries
			fieldQuery.AddShould(matchQuery)

			// 3. Prefix query for partial matches (e.g., "access" matches "access_log")
			prefixQuery := bleve.NewPrefixQuery(strings.ToLower(queryStr))
			prefixQuery.SetField(field)
			prefixQuery.SetBoost(3.0) // Medium boost for prefix matches
			fieldQuery.AddShould(prefixQuery)

			// 4. Wildcard query for more flexible matching
			wildcardQuery := bleve.NewWildcardQuery("*" + strings.ToLower(queryStr) + "*")
			wildcardQuery.SetField(field)
			wildcardQuery.SetBoost(2.0) // Lower boost for wildcard matches
			fieldQuery.AddShould(wildcardQuery)

			// 5. Fuzzy match query (allows 1 character difference) - only for text queries
			fuzzyQuery := bleve.NewFuzzyQuery(queryStr)
			fuzzyQuery.SetField(field)
			fuzzyQuery.SetFuzziness(1)
			fuzzyQuery.SetBoost(1.0) // Lowest boost for fuzzy matches
			fieldQuery.AddShould(fuzzyQuery)
		}

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
			ID:      hit.ID,
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
	si.indexMutex.Lock()
	defer si.indexMutex.Unlock()

	if si.index == nil {
		return fmt.Errorf("search index not initialized")
	}

	if err := si.index.Delete(docID); err != nil {
		return err
	}

	si.memoryMutex.Lock()
	defer si.memoryMutex.Unlock()
	if contentSize, exists := si.documentSizes[docID]; exists {
		si.totalContentSize -= contentSize
		si.documentCount--
		delete(si.documentSizes, docID)
	}
	return nil
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

	// Check context before creating new index
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Create new index
	var err error
	si.index, err = bleve.NewMemOnly(si.createIndexMapping())
	if err != nil {
		return fmt.Errorf("failed to create new in-memory index: %w", err)
	}
	si.resetMemoryUsage()

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

	// Get memory usage statistics
	totalContentSize, trackedDocCount, maxMemoryUsage := si.getMemoryUsage()

	return map[string]interface{}{
		"document_count":         docCount,
		"tracked_document_count": trackedDocCount,
		"total_content_size":     totalContentSize,
		"max_memory_usage":       maxMemoryUsage,
		"memory_usage_percent":   float64(totalContentSize) / float64(maxMemoryUsage) * 100,
		"index_path":             si.indexPath,
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

func (si *SearchIndexer) resetMemoryUsage() {
	si.memoryMutex.Lock()
	defer si.memoryMutex.Unlock()
	si.totalContentSize = 0
	si.documentCount = 0
	si.documentSizes = make(map[string]int64)
}

// getMemoryUsage returns current memory usage statistics
func (si *SearchIndexer) getMemoryUsage() (int64, int64, int64) {
	si.memoryMutex.RLock()
	defer si.memoryMutex.RUnlock()
	return si.totalContentSize, si.documentCount, si.maxMemoryUsage
}

// isConfigFile checks if the content is a text/plain file (most nginx configs)
func isConfigFile(content []byte) bool {
	if len(content) == 0 {
		return false // Empty files are not useful for configuration
	}

	// Detect MIME type and only accept text/plain
	mtype := mimetype.Detect(content)

	if mtype.Is("text/plain") {
		return true
	}

	return false
}
