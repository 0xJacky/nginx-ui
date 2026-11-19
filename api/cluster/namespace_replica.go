package cluster

import (
	"errors"
	"fmt"

	"github.com/0xJacky/Nginx-UI/model"
	"gorm.io/gorm"
)

type namespaceSyncRequest struct {
	SourceID         uint64 `json:"source_id"`
	OriginalName     string `json:"original_name"`
	Name             string `json:"name" binding:"required"`
	PostSyncAction   string `json:"post_sync_action" binding:"omitempty,oneof=none reload_nginx"`
	UpstreamTestType string `json:"upstream_test_type" binding:"omitempty,oneof=local remote mirror"`
}

var errNamespaceConflict = errors.New("namespace name conflicts with an existing namespace on this node")

func upsertNamespace(db *gorm.DB, payload namespaceSyncRequest, source string) (*model.Namespace, error) {
	var namespace model.Namespace
	err := db.Transaction(func(tx *gorm.DB) error {
		lookup := tx.Where("name = ?", payload.Name)
		if source != "" {
			lookup = lookup.Or("replica_source = ?", source)
			if payload.OriginalName != "" && payload.OriginalName != payload.Name {
				// Consult the old name only before the replica has been bound.
				bound := tx.Model(&model.Namespace{}).Select("1").Where("replica_source = ?", source)
				lookup = lookup.Or("replica_source IS NULL AND name = ? AND NOT EXISTS (?)", payload.OriginalName, bound)
			}
		}
		var matches []model.Namespace
		if err := lookup.Limit(2).Find(&matches).Error; err != nil {
			return err
		}
		if len(matches) > 1 {
			return fmt.Errorf("%w: %s", errNamespaceConflict, payload.Name)
		}
		if len(matches) > 0 {
			namespace = matches[0]
			if source != "" && namespace.ReplicaSource != nil && *namespace.ReplicaSource != source {
				return fmt.Errorf("%w: %s", errNamespaceConflict, payload.Name)
			}
		}
		values := model.Namespace{Name: payload.Name, PostSyncAction: payload.PostSyncAction, UpstreamTestType: payload.UpstreamTestType}
		if source != "" {
			values.ReplicaSource = &source
		}
		if namespace.ID == 0 {
			namespace = values
			return tx.Create(&namespace).Error
		}
		return tx.Model(&namespace).Updates(values).Error
	})
	return &namespace, err
}
