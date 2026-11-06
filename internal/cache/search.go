package cache

import (
	"context"
	"fmt"
	"hash/fnv"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gabriel-vasile/mimetype"
	"github.com/uozi-tech/cosy/logger"
)

const (
	// maxIndexedFileSize skips scanned files that are too large to be useful.
	maxIndexedFileSize = 1024 * 1024
	// maxDocumentContentSize is the absolute limit accepted by IndexDocument.
	maxDocumentContentSize = 2 * 1024 * 1024
	// maxIndexedDocuments caps the number of documents kept in memory.
	maxIndexedDocuments = 1000
	// defaultSearchLimit is used when the caller passes a non-positive limit.
	defaultSearchLimit = 500
	// minFuzzyQueryLength is the shortest query that gets typo tolerance.
	minFuzzyQueryLength = 3
	// maxContentBonus caps the bonus for repeated content matches.
	maxContentBonus = 40
	// longTokenLength is the token length that allows two edits instead of one.
	longTokenLength = 8
	// maxFuzzyDistance is the most edits tolerated in any match.
	maxFuzzyDistance = 2
	// cancelCheckInterval is how many documents are scanned between context checks.
	cancelCheckInterval = 64
)

// Scores by match tier, from best to worst. Bonuses stay below the gap
// between tiers so a tier never overtakes the one above it.
const (
	scoreNameExact    = 1000.0
	scorePort         = 900.0
	scoreNamePrefix   = 800.0
	scoreNameContains = 600.0
	scoreNameFuzzy    = 400.0
	scorePathContains = 250.0
	scoreContent      = 200.0
	scoreContentTerms = 150.0
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

// indexedDocument keeps only what matching needs. The original content is
// not retained, so results carry an empty Content field.
type indexedDocument struct {
	id          string
	docType     string
	name        string
	path        string
	updatedAt   time.Time
	contentHash uint64
	contentSize int64

	lowerName    string
	lowerPath    string
	lowerContent string
	nameTokens   []string
	ports        []uint16
}

// SearchIndexer keeps config documents in memory and matches queries against them
type SearchIndexer struct {
	indexPath  string
	indexMutex sync.RWMutex
	docs       map[string]*indexedDocument
	ctx        context.Context
	cancel     context.CancelFunc

	// Content budget, guarded by indexMutex
	totalContentSize int64
	maxMemoryUsage   int64
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

// Initialize sets up the in-memory search index
func (si *SearchIndexer) Initialize(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	si.indexMutex.Lock()
	if si.cancel != nil {
		si.cancel()
	}
	// Create a derived context for cleanup
	si.ctx, si.cancel = context.WithCancel(ctx)
	watched := si.ctx
	si.docs = make(map[string]*indexedDocument)
	si.resetMemoryUsage()
	if si.maxMemoryUsage <= 0 {
		si.maxMemoryUsage = 100 * 1024 * 1024
	}
	si.indexMutex.Unlock()

	logger.Info("Creating in-memory search index")

	// Register callback for config scanning
	RegisterCallback("search.handleConfigScan", si.handleConfigScan)

	// Start cleanup goroutine
	go si.watchContext(watched)

	logger.Info("Search index initialized successfully")
	return nil
}

// watchContext releases the index when its context is cancelled
func (si *SearchIndexer) watchContext(ctx context.Context) {
	<-ctx.Done()

	si.indexMutex.Lock()
	defer si.indexMutex.Unlock()
	// A newer Initialize owns the index now
	if si.ctx == ctx {
		si.cleanupLocked()
	}
}

// cleanupLocked drops all documents and resets memory accounting.
func (si *SearchIndexer) cleanupLocked() {
	if si.docs != nil {
		logger.Info("Cleaning up search index...")
	}
	si.docs = nil
	si.resetMemoryUsage()
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

	// File size limit to prevent memory overflow and improve performance
	if len(content) > maxIndexedFileSize {
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

// IndexDocument adds or replaces a single document
func (si *SearchIndexer) IndexDocument(doc SearchDocument) (err error) {
	// Add panic recovery to prevent the entire application from crashing
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic during indexing: %v", r)
			logger.Error("Panic occurred while indexing document", "document_id", doc.ID, "error", err)
		}
	}()

	// Additional size check as a safety measure
	if len(doc.Content) > maxDocumentContentSize {
		return fmt.Errorf("document content too large: %d bytes", len(doc.Content))
	}

	hash := hashContent(doc.Content)
	contentSize := int64(len(doc.Content))

	si.indexMutex.Lock()
	defer si.indexMutex.Unlock()

	if si.docs == nil {
		return fmt.Errorf("search index not initialized")
	}

	existing, isExisting := si.docs[doc.ID]
	if isExisting && existing.contentHash == hash && existing.contentSize == contentSize {
		return nil
	}

	var previousSize int64
	documentCount := len(si.docs)
	if isExisting {
		previousSize = existing.contentSize
	} else {
		documentCount++
	}
	newTotalSize := si.totalContentSize - previousSize + contentSize
	if newTotalSize > si.maxMemoryUsage || documentCount > maxIndexedDocuments {
		logger.Warn("Skipping document due to content budget",
			"document_id", doc.ID,
			"content_size", contentSize,
			"content_budget", si.maxMemoryUsage)
		return nil
	}

	si.docs[doc.ID] = newIndexedDocument(doc, hash)
	si.totalContentSize = newTotalSize
	return nil
}

// newIndexedDocument reduces a document to the fields used for matching.
func newIndexedDocument(doc SearchDocument, hash uint64) *indexedDocument {
	lowerName := strings.ToLower(doc.Name)
	return &indexedDocument{
		id:          doc.ID,
		docType:     doc.Type,
		name:        doc.Name,
		path:        doc.Path,
		updatedAt:   doc.UpdatedAt,
		contentHash: hash,
		contentSize: int64(len(doc.Content)),

		lowerName:    lowerName,
		lowerPath:    strings.ToLower(matchablePath(doc.Path)),
		lowerContent: strings.ToLower(doc.Content),
		nameTokens:   splitNameTokens(lowerName),
		ports:        extractListenPorts(doc.Content),
	}
}

// matchablePath returns the part of path below the nginx config directory, so
// a query for a word of the config root itself does not match every document.
func matchablePath(path string) string {
	root := nginx.GetConfPath()
	if root == "" {
		return path
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	return rel
}

func hashContent(content string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(content))
	return h.Sum64()
}

// isNameSeparator reports whether r splits a name into tokens.
func isNameSeparator(r rune) bool {
	switch r {
	case '.', '-', '_', '/', '\\', ' ':
		return true
	}
	return false
}

// splitNameTokens splits a lowercased name on separators. Tokens share the
// backing memory of the name.
func splitNameTokens(name string) []string {
	return strings.FieldsFunc(name, isNameSeparator)
}

// extractListenPorts returns the distinct ports of listen directives.
func extractListenPorts(content string) []uint16 {
	var ports []uint16
	n := len(content)
	atStart := true // at the start of a directive

	for i := 0; i < n; {
		c := content[i]
		switch {
		case c == '#':
			for i < n && content[i] != '\n' {
				i++
			}
		case c == ';' || c == '{' || c == '}' || c == '\n':
			atStart = true
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		default:
			start := i
			for i < n && !isDirectiveDelimiter(content[i]) {
				i++
			}
			if atStart && strings.EqualFold(content[start:i], "listen") {
				for i < n && (content[i] == ' ' || content[i] == '\t') {
					i++
				}
				argStart := i
				for i < n && !isDirectiveDelimiter(content[i]) {
					i++
				}
				ports = appendListenPort(ports, content[argStart:i])
			}
			atStart = false
		}
	}
	return ports
}

func isDirectiveDelimiter(c byte) bool {
	switch c {
	case ' ', '\t', '\r', '\n', ';', '{', '}':
		return true
	}
	return false
}

// appendListenPort parses a listen argument such as 80, 127.0.0.1:8080 or [::]:443.
func appendListenPort(ports []uint16, arg string) []uint16 {
	if arg == "" || strings.HasPrefix(arg, "unix:") {
		return ports
	}
	if idx := strings.LastIndexByte(arg, ':'); idx >= 0 {
		// A bare IPv6 address without a port has no port part
		if strings.HasSuffix(arg, "]") {
			return ports
		}
		arg = arg[idx+1:]
	}
	value, err := strconv.ParseUint(arg, 10, 16)
	if err != nil || value == 0 {
		return ports
	}
	port := uint16(value)
	for _, existing := range ports {
		if existing == port {
			return ports
		}
	}
	return append(ports, port)
}

// Search performs a search query
func (si *SearchIndexer) Search(ctx context.Context, queryStr string, limit int) ([]SearchResult, error) {
	return si.searchWithType(ctx, queryStr, "", limit)
}

// SearchByType performs a search filtered by document type
func (si *SearchIndexer) SearchByType(ctx context.Context, queryStr string, docType string, limit int) ([]SearchResult, error) {
	return si.searchWithType(ctx, queryStr, docType, limit)
}

// searchQuery is a normalized query
type searchQuery struct {
	text    string
	terms   []string // set when the text has several words
	tokens  []string // set when the text spans several name tokens
	port    uint16   // set for numeric queries that can be a port
	numeric bool
	fuzzy   bool
}

func newSearchQuery(queryStr string) searchQuery {
	text := strings.ToLower(strings.TrimSpace(queryStr))
	// "port:8080" searches for the port itself
	if rest := strings.TrimPrefix(text, "port:"); rest != text && isNumericQuery(rest) {
		text = strings.TrimSpace(rest)
	}
	q := searchQuery{text: text, numeric: isNumericQuery(text)}

	if q.numeric {
		digits := strings.TrimPrefix(text, ":")
		if value, err := strconv.ParseUint(digits, 10, 16); err == nil && value > 0 {
			q.port = uint16(value)
		}
	} else {
		q.fuzzy = utf8.RuneCountInString(text) >= minFuzzyQueryLength
		if tokens := splitNameTokens(text); len(tokens) > 1 {
			q.tokens = tokens
		}
	}
	if fields := strings.Fields(text); len(fields) > 1 {
		q.terms = fields
	}
	return q
}

// searchWithType performs the actual search with optional type filtering
func (si *SearchIndexer) searchWithType(ctx context.Context, queryStr string, docType string, limit int) ([]SearchResult, error) {
	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if limit <= 0 {
		limit = defaultSearchLimit
	}

	q := newSearchQuery(queryStr)

	si.indexMutex.RLock()
	defer si.indexMutex.RUnlock()

	if si.docs == nil {
		return nil, fmt.Errorf("search index not initialized")
	}
	if q.text == "" {
		return []SearchResult{}, nil
	}

	var matches []scoredDocument
	scanned := 0
	for _, doc := range si.docs {
		scanned++
		if scanned%cancelCheckInterval == 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
		}
		if docType != "" && doc.docType != docType {
			continue
		}
		if score, ok := doc.score(&q); ok {
			matches = append(matches, scoredDocument{doc: doc, score: score})
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		if matches[i].doc.name != matches[j].doc.name {
			return matches[i].doc.name < matches[j].doc.name
		}
		return matches[i].doc.id < matches[j].doc.id
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}

	results := make([]SearchResult, len(matches))
	for i, m := range matches {
		results[i] = SearchResult{
			Document: SearchDocument{
				ID:        m.doc.id,
				Type:      m.doc.docType,
				Name:      m.doc.name,
				Path:      m.doc.path,
				UpdatedAt: m.doc.updatedAt,
			},
			Score: m.score,
		}
	}

	// log the search execution
	logger.Debugf("Search index query '%s' (type: %s, limit: %d) returned %d results",
		queryStr, docType, limit, len(results))

	return results, nil
}

type scoredDocument struct {
	doc   *indexedDocument
	score float64
}

// score returns the best matching tier for the query.
func (d *indexedDocument) score(q *searchQuery) (float64, bool) {
	name := d.lowerName

	// Name tiers
	if name == q.text || strings.TrimSuffix(name, ".conf") == q.text {
		return scoreNameExact, true
	}
	if q.port != 0 {
		for _, port := range d.ports {
			if port == q.port {
				return scorePort, true
			}
		}
	}
	if strings.HasPrefix(name, q.text) {
		return scoreNamePrefix + closeness(q.text, name), true
	}
	if strings.Contains(name, q.text) {
		return scoreNameContains + closeness(q.text, name), true
	}
	if q.fuzzy {
		if distance, ok := d.fuzzyNameDistance(q); ok {
			return scoreNameFuzzy - float64(distance)*20 + closeness(q.text, name), true
		}
	}

	// Path and content tiers. Digits in directory names are noise for numeric queries.
	if !q.numeric && strings.Contains(d.lowerPath, q.text) {
		return scorePathContains, true
	}
	if count := strings.Count(d.lowerContent, q.text); count > 0 {
		return scoreContent + float64(min(count, maxContentBonus)), true
	}
	if len(q.terms) > 0 && containsAllTerms(d.lowerContent, d.lowerPath, q.terms) {
		return scoreContentTerms, true
	}
	return 0, false
}

// closeness is a bonus below 50 that favors names close in length to the query.
func closeness(query, name string) float64 {
	if len(name) == 0 {
		return 0
	}
	ratio := float64(len(query)) / float64(len(name))
	if ratio > 1 {
		ratio = 1
	}
	return ratio * 49
}

func containsAllTerms(content, docPath string, terms []string) bool {
	for _, term := range terms {
		if !strings.Contains(content, term) && !strings.Contains(docPath, term) {
			return false
		}
	}
	return true
}

// fuzzyNameDistance finds the smallest edit distance between the query and a
// name token or the whole name, within the allowed tolerance. A query that
// spans several tokens matches when each of its tokens is found or close to
// a name token.
func (d *indexedDocument) fuzzyNameDistance(q *searchQuery) (int, bool) {
	best := -1
	consider := func(candidate string) {
		if distance := boundedEditDistance(q.text, candidate, fuzzyLimit(candidate)); distance >= 0 && (best < 0 || distance < best) {
			best = distance
		}
	}

	consider(d.lowerName)
	for _, token := range d.nameTokens {
		consider(token)
	}
	if best < 0 && len(q.tokens) > 1 {
		if distance, ok := d.fuzzyTokensDistance(q.tokens); ok {
			best = distance
		}
	}
	return best, best >= 0
}

// fuzzyTokensDistance sums the typo distance of each query token against the
// name. Tokens that appear in the name as they are cost nothing.
func (d *indexedDocument) fuzzyTokensDistance(queryTokens []string) (int, bool) {
	total := 0
	for _, queryToken := range queryTokens {
		if strings.Contains(d.lowerName, queryToken) {
			continue
		}
		if utf8.RuneCountInString(queryToken) < minFuzzyQueryLength {
			return 0, false
		}
		best := -1
		for _, token := range d.nameTokens {
			if distance := boundedEditDistance(queryToken, token, fuzzyLimit(token)); distance >= 0 && (best < 0 || distance < best) {
				best = distance
			}
		}
		if best < 0 {
			return 0, false
		}
		total += best
	}
	return total, total > 0 && total <= maxFuzzyDistance
}

// fuzzyLimit is the edit distance allowed for a candidate of the given length.
func fuzzyLimit(candidate string) int {
	if utf8.RuneCountInString(candidate) >= longTokenLength {
		return maxFuzzyDistance
	}
	return 1
}

// boundedEditDistance returns the optimal string alignment distance (edits
// and adjacent transpositions) between a and b, or -1 when it exceeds limit.
func boundedEditDistance(a, b string, limit int) int {
	if a == b {
		return 0
	}
	// Cheap length check before allocating rune slices
	la, lb := utf8.RuneCountInString(a), utf8.RuneCountInString(b)
	if diff := la - lb; diff > limit || -diff > limit || la == 0 || lb == 0 {
		return -1
	}
	ra, rb := []rune(a), []rune(b)

	prevPrev := make([]int, len(rb)+1)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		rowMin := cur[0]
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			v := min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				v = min(v, prevPrev[j-2]+1)
			}
			cur[j] = v
			rowMin = min(rowMin, v)
		}
		if rowMin > limit {
			return -1
		}
		prevPrev, prev, cur = prev, cur, prevPrev
	}

	if prev[len(rb)] > limit {
		return -1
	}
	return prev[len(rb)]
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

// DeleteDocument removes a document from the index
func (si *SearchIndexer) DeleteDocument(docID string) error {
	si.indexMutex.Lock()
	defer si.indexMutex.Unlock()

	if si.docs == nil {
		return fmt.Errorf("search index not initialized")
	}

	if doc, exists := si.docs[docID]; exists {
		si.totalContentSize -= doc.contentSize
		delete(si.docs, docID)
	}
	return nil
}

// RebuildIndex drops every document so the next scan repopulates the index
func (si *SearchIndexer) RebuildIndex(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	si.indexMutex.Lock()
	defer si.indexMutex.Unlock()

	si.docs = make(map[string]*indexedDocument)
	si.resetMemoryUsage()

	logger.Info("Search index rebuilt successfully")
	return nil
}

// GetIndexStats returns statistics about the search index
func (si *SearchIndexer) GetIndexStats() (map[string]interface{}, error) {
	si.indexMutex.RLock()
	defer si.indexMutex.RUnlock()

	if si.docs == nil {
		return nil, fmt.Errorf("search index not initialized")
	}

	// Get memory usage statistics
	totalContentSize, documentCount, maxMemoryUsage := si.getMemoryUsage()

	return map[string]interface{}{
		"document_count":         documentCount,
		"tracked_document_count": documentCount,
		"total_content_size":     totalContentSize,
		"max_memory_usage":       maxMemoryUsage,
		"memory_usage_percent":   float64(totalContentSize) / float64(maxMemoryUsage) * 100,
		"index_path":             si.indexPath,
	}, nil
}

// Close closes the search index and triggers cleanup
func (si *SearchIndexer) Close() error {
	si.indexMutex.Lock()
	cancel := si.cancel
	si.cleanupLocked()
	si.indexMutex.Unlock()

	if cancel != nil {
		cancel()
	}
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

// resetMemoryUsage clears the content budget. The caller holds indexMutex.
func (si *SearchIndexer) resetMemoryUsage() {
	si.totalContentSize = 0
}

// getMemoryUsage returns current memory usage statistics. The caller holds indexMutex.
func (si *SearchIndexer) getMemoryUsage() (int64, int64, int64) {
	return si.totalContentSize, int64(len(si.docs)), si.maxMemoryUsage
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
