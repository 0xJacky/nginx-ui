package migrate

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

var RenameEnvironmentsToNodes = &gormigrate.Migration{
	ID: "20250812000002",
	Migrate: func(tx *gorm.DB) error {
		// 检查 environments 表是否存在
		if !tx.Migrator().HasTable("environments") {
			// 如果 environments 表不存在，说明已经迁移过了或者是新安装
			return nil
		}

		// 检查 nodes 表是否存在
		if !tx.Migrator().HasTable("nodes") {
			// nodes 表不存在，直接重命名
			if err := tx.Exec("ALTER TABLE environments RENAME TO nodes").Error; err != nil {
				return err
			}
		} else {
			// nodes 表已存在，迁移数据后删除旧表
			if err := tx.Exec(`
				INSERT OR IGNORE INTO nodes (id, created_at, updated_at, deleted_at, name, url, token, enabled)
				SELECT id, created_at, updated_at, deleted_at, 
					   COALESCE(name, 'Unknown Node') as name,
					   COALESCE(url, '') as url,
					   COALESCE(token, '') as token,
					   COALESCE(enabled, false) as enabled
				FROM environments
			`).Error; err != nil {
				return err
			}

			// 删除旧表
			if err := tx.Migrator().DropTable("environments"); err != nil {
				return err
			}
		}

		// 更新 sites 表中的外键字段
		if tx.Migrator().HasColumn("sites", "environment_id") {
			// 添加新列（如果不存在）
			if !tx.Migrator().HasColumn("sites", "node_id") {
				if err := tx.Exec("ALTER TABLE sites ADD COLUMN node_id BIGINT").Error; err != nil {
					return err
				}
			}

			// 复制数据
			if err := tx.Exec("UPDATE sites SET node_id = environment_id WHERE node_id IS NULL OR node_id = 0").Error; err != nil {
				return err
			}
		}

		// 更新 streams 表中的外键字段
		if tx.Migrator().HasColumn("streams", "environment_id") {
			// 添加新列（如果不存在）
			if !tx.Migrator().HasColumn("streams", "node_id") {
				if err := tx.Exec("ALTER TABLE streams ADD COLUMN node_id BIGINT").Error; err != nil {
					return err
				}
			}

			// 复制数据
			if err := tx.Exec("UPDATE streams SET node_id = environment_id WHERE node_id IS NULL OR node_id = 0").Error; err != nil {
				return err
			}
		}

		// 更新 configs 表中的外键字段（如果存在）
		if tx.Migrator().HasColumn("configs", "environment_id") {
			// 添加新列（如果不存在）
			if !tx.Migrator().HasColumn("configs", "node_id") {
				if err := tx.Exec("ALTER TABLE configs ADD COLUMN node_id BIGINT").Error; err != nil {
					return err
				}
			}

			// 复制数据
			if err := tx.Exec("UPDATE configs SET node_id = environment_id WHERE node_id IS NULL OR node_id = 0").Error; err != nil {
				return err
			}
		}

		// 更新 certs 表中的外键字段（如果存在）
		if tx.Migrator().HasColumn("certs", "environment_id") {
			// 添加新列（如果不存在）
			if !tx.Migrator().HasColumn("certs", "node_id") {
				if err := tx.Exec("ALTER TABLE certs ADD COLUMN node_id BIGINT").Error; err != nil {
					return err
				}
			}

			// 复制数据
			if err := tx.Exec("UPDATE certs SET node_id = environment_id WHERE node_id IS NULL OR node_id = 0").Error; err != nil {
				return err
			}
		}

		// 更新 namespaces 表中的 sync_node_ids 字段内容（JSON 格式的环境 ID 需要保持数据一致性）
		// 由于 sync_node_ids 存储的是 JSON 数组，我们只需要确保引用的 ID 仍然有效
		// 这里不需要特殊处理，因为 ID 值在重命名表后保持不变

		return nil
	},
}