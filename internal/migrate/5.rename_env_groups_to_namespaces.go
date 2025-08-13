package migrate

import (
	"github.com/0xJacky/Nginx-UI/model"
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
			// 使用 GORM 查询和创建来迁移数据
			var envGroups []model.Namespace
			if err := tx.Table("env_groups").Find(&envGroups).Error; err != nil {
				return err
			}

			// 为每个 env_group 创建对应的 namespace
			for _, envGroup := range envGroups {
				// 设置默认值
				if envGroup.Name == "" {
					envGroup.Name = "Default"
				}
				if envGroup.PostSyncAction == "" {
					envGroup.PostSyncAction = "reload_nginx"
				}
				if envGroup.UpstreamTestType == "" {
					envGroup.UpstreamTestType = "local"
				}

				// 使用 FirstOrCreate 避免重复插入
				var existingNamespace model.Namespace
				if err := tx.Where("id = ?", envGroup.ID).FirstOrCreate(&existingNamespace, &envGroup).Error; err != nil {
					return err
				}
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
