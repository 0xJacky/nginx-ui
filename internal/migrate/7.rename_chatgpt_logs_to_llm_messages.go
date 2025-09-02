package migrate

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

var RenameChatGPTLogsToLLMSessions = &gormigrate.Migration{
	ID: "20250831000001",
	Migrate: func(tx *gorm.DB) error {
		// 检查 chatgpt_logs 表是否存在
		if !tx.Migrator().HasTable("chat_gpt_logs") {
			return nil
		}

		// llm_messages 表已存在，迁移数据后删除旧表
		// 使用原生 SQL 迁移数据，因为两个表结构相同
		if err := tx.Exec("INSERT INTO llm_messages (path, content) SELECT name, content FROM chat_gpt_logs WHERE NOT EXISTS (SELECT 1 FROM llm_messages WHERE llm_messages.name = chat_gpt_logs.name)").Error; err != nil {
			return err
		}

		// 删除旧表
		if err := tx.Migrator().DropTable("chat_gpt_logs"); err != nil {
			return err
		}

		return nil
	},
	Rollback: func(tx *gorm.DB) error {
		// 回滚：将 llm_messages 表重命名回 chatgpt_logs
		if !tx.Migrator().HasTable("chat_gpt_logs") && tx.Migrator().HasTable("llm_messages") {
			if err := tx.Exec("ALTER TABLE llm_messages RENAME TO chat_gpt_logs").Error; err != nil {
				return err
			}
		}
		return nil
	},
}
