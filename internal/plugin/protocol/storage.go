package protocol

import (
	"encoding/json"
	"fmt"
	"math"
)

// StorageValidateParams is the payload of storage.validate.
type StorageValidateParams struct {
	// Backend is the backend code declared in the manifest, without any host prefix.
	Backend string `json:"backend"`
	// Config holds the field values the user filled in, keyed by field key.
	Config map[string]string `json:"config"`
}

// StoragePutParams is the payload of storage.put.
type StoragePutParams struct {
	Backend string            `json:"backend"`
	Config  map[string]string `json:"config"`
	// Key is a relative path with "/" separated segments.
	Key string `json:"key"`
	// SourcePath is the absolute path of the file to store, inside the
	// exchange directory of the plugin's data directory.
	SourcePath string `json:"source_path"`
}

// StorageGetParams is the payload of storage.get.
type StorageGetParams struct {
	Backend string            `json:"backend"`
	Config  map[string]string `json:"config"`
	Key     string            `json:"key"`
	// TargetPath is the absolute path the plugin writes the object to, inside
	// the exchange directory of the plugin's data directory.
	TargetPath string `json:"target_path"`
}

// StorageSizeResult is the reply to storage.put and storage.get.
type StorageSizeResult struct {
	// Size is the number of bytes stored or written.
	Size ByteSize `json:"size"`
}

// StorageListParams is the payload of storage.list.
type StorageListParams struct {
	Backend string            `json:"backend"`
	Config  map[string]string `json:"config"`
	// Prefix is a plain string prefix of the keys to list. Empty lists every
	// object.
	Prefix string `json:"prefix,omitempty"`
}

// StorageListResult is the reply to storage.list.
type StorageListResult struct {
	Objects []StorageObject `json:"objects"`
}

// StorageObject is one stored object.
type StorageObject struct {
	Key  string   `json:"key"`
	Size ByteSize `json:"size"`
	// ModifiedAt is an RFC 3339 timestamp, empty when unknown.
	ModifiedAt string `json:"modified_at,omitempty"`
}

// StorageDeleteParams is the payload of storage.delete.
type StorageDeleteParams struct {
	Backend string            `json:"backend"`
	Config  map[string]string `json:"config"`
	Key     string            `json:"key"`
}

// ByteSize is a number of bytes. The contract carries it as a double, so it
// stays a JSON number past 4 GiB; it decodes with or without a fraction.
type ByteSize int64

// UnmarshalJSON accepts any JSON number that is a whole, non-negative byte
// count, such as 5242880 or 5242880.0.
func (s *ByteSize) UnmarshalJSON(data []byte) error {
	var value float64
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("size: %w", err)
	}
	if value < 0 || value != math.Trunc(value) || value > 1<<53 {
		return fmt.Errorf("size %v is not a byte count", value)
	}
	*s = ByteSize(value)
	return nil
}
