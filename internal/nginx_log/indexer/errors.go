package indexer

import "github.com/uozi-tech/cosy"

var (
	e                          = cosy.NewErrorScope("nginx_log.indexer")
	ErrLogParserNotInitialized = e.New(50201, "log parser is not initialized; call indexer.InitLogParser() before use")
)
