package migrate

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

var RenameEnvGroupsToNamespaces = &gormigrate.Migration{
	ID: "20250812000001",
	Migrate: func(tx *gorm.DB) error {
		// 检查 env_groups 表是否存在
		if !tx.Migrator().HasTable("env_groups") {
			// 如果 env_groups 表不存在，说明已经迁移过了或者是新安装
			return nil
		}

		// 检查 namespaces 表是否存在
		if !tx.Migrator().HasTable("namespaces") {
			// namespaces 表不存在，直接重命名
			if err := tx.Exec("ALTER TABLE env_groups RENAME TO namespaces").Error; err != nil {
				return err
			}
		} else {
			// namespaces 表已存在，迁移数据后删除旧表
			if err := tx.Exec(`
				INSERT OR IGNORE INTO namespaces (id, created_at, updated_at, deleted_at, name, sync_node_ids, order_id, post_sync_action, upstream_test_type)
				SELECT id, created_at, updated_at, deleted_at, 
					   COALESCE(name, 'Default') as name,
					   COALESCE(sync_node_ids, '[]') as sync_node_ids,
					   COALESCE(order_id, 0) as order_id,
					   COALESCE(post_sync_action, 'reload_nginx') as post_sync_action,
					   COALESCE(upstream_test_type, 'local') as upstream_test_type
				FROM env_groups
			`).Error; err != nil {
				return err
			}

			// 删除旧表
			if err := tx.Migrator().DropTable("env_groups"); err != nil {
				return err
			}
		}

		// 更新 sites 表中的外键字段
		if tx.Migrator().HasColumn("sites", "env_group_id") {
			// 添加新列（如果不存在）
			if !tx.Migrator().HasColumn("sites", "namespace_id") {
				if err := tx.Exec("ALTER TABLE sites ADD COLUMN namespace_id BIGINT").Error; err != nil {
					return err
				}
			}

			// 复制数据
			if err := tx.Exec("UPDATE sites SET namespace_id = env_group_id WHERE namespace_id IS NULL OR namespace_id = 0").Error; err != nil {
				return err
			}
		}

		// 更新 streams 表中的外键字段
		if tx.Migrator().HasColumn("streams", "env_group_id") {
			// 添加新列（如果不存在）
			if !tx.Migrator().HasColumn("streams", "namespace_id") {
				if err := tx.Exec("ALTER TABLE streams ADD COLUMN namespace_id BIGINT").Error; err != nil {
					return err
				}
			}

			// 复制数据
			if err := tx.Exec("UPDATE streams SET namespace_id = env_group_id WHERE namespace_id IS NULL OR namespace_id = 0").Error; err != nil {
				return err
			}
		}

		// 更新 configs 表中的外键字段（如果存在）
		if tx.Migrator().HasColumn("configs", "env_group_id") {
			// 添加新列（如果不存在）
			if !tx.Migrator().HasColumn("configs", "namespace_id") {
				if err := tx.Exec("ALTER TABLE configs ADD COLUMN namespace_id BIGINT").Error; err != nil {
					return err
				}
			}

			// 复制数据
			if err := tx.Exec("UPDATE configs SET namespace_id = env_group_id WHERE namespace_id IS NULL OR namespace_id = 0").Error; err != nil {
				return err
			}
		}

		return nil
	},
}
