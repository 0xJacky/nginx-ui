package cron

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/0xJacky/Nginx-UI/model"
)

type stubLogIndexProvider struct {
	idx *model.NginxLogIndex
	err error
}

func (s stubLogIndexProvider) GetLogIndex(path string) (*model.NginxLogIndex, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.idx != nil {
		s.idx.Path = path
	}
	return s.idx, nil
}

func TestNeedsIncrementalIndexingSkipsWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "access.log")
	if err := os.WriteFile(logPath, []byte("initial\n"), 0o644); err != nil {
		t.Fatalf("write temp log: %v", err)
	}

	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("stat temp log: %v", err)
	}

	persisted := &model.NginxLogIndex{
		Path:         logPath,
		LastModified: info.ModTime(),
		LastSize:     info.Size(),
		LastIndexed:  time.Now(),
	}

	logData := &nginx_log.NginxLogWithIndex{
		Path:         logPath,
		Type:         "access",
		IndexStatus:  string(indexer.IndexStatusIndexed),
		LastModified: info.ModTime().Unix(),
		LastSize:     info.Size() * 10, // simulate grouped size inflation
		LastIndexed:  time.Now().Unix(),
	}

	if needsIncrementalIndexing(logData, stubLogIndexProvider{idx: persisted}) {
		t.Fatalf("expected no incremental indexing when file metadata is unchanged")
	}
}

func TestNeedsIncrementalIndexingDetectsGrowth(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "access.log")
	if err := os.WriteFile(logPath, []byte("initial\n"), 0o644); err != nil {
		t.Fatalf("write temp log: %v", err)
	}

	initialInfo, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("stat temp log: %v", err)
	}

	persisted := &model.NginxLogIndex{
		Path:         logPath,
		LastModified: initialInfo.ModTime().Add(-time.Minute),
		LastSize:     initialInfo.Size(),
		LastIndexed:  time.Now().Add(-time.Minute),
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open temp log: %v", err)
	}
	if _, err := f.WriteString("more data\n"); err != nil {
		f.Close()
		t.Fatalf("append temp log: %v", err)
	}
	_ = f.Close()

	finalInfo, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("restat temp log: %v", err)
	}

	logData := &nginx_log.NginxLogWithIndex{
		Path:         logPath,
		Type:         "access",
		IndexStatus:  string(indexer.IndexStatusIndexed),
		LastModified: finalInfo.ModTime().Unix(),
		LastSize:     initialInfo.Size(),
		LastIndexed:  time.Now().Unix(),
	}

	if !needsIncrementalIndexing(logData, stubLogIndexProvider{idx: persisted}) {
		t.Fatalf("expected incremental indexing when file grew")
	}
}
