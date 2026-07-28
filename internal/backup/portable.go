package backup

import (
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type portableCredentialColumn struct {
	table  string
	column string
}

var portableCredentialColumns = []portableCredentialColumn{
	{table: "users", column: "otp_secret"},
	{table: "users", column: "recovery_codes"},
	{table: "dns_credentials", column: "config"},
	{table: "acme_users", column: "key"},
	{table: "certs", column: "resource"},
	{table: "external_notifies", column: "config"},
	{table: "auto_backups", column: "s3_access_key_id"},
	{table: "auto_backups", column: "s3_secret_access_key"},
	{table: "nodes", column: "encrypted_legacy_secret"},
}

// invalidatePortableCredentials removes data encrypted with the source
// instance key. Keeping opaque ciphertext would make restored records fail in
// unpredictable paths and could become usable after a later key change.
func invalidatePortableCredentials(databasePath string) error {
	database, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fmt.Errorf("open staged portable database: %w", err)
	}
	sqlDatabase, err := database.DB()
	if err != nil {
		return fmt.Errorf("access staged portable database: %w", err)
	}
	defer sqlDatabase.Close()

	for _, credential := range portableCredentialColumns {
		if !database.Migrator().HasTable(credential.table) || !database.Migrator().HasColumn(credential.table, credential.column) {
			continue
		}
		statement := fmt.Sprintf("UPDATE %s SET %s = NULL", quoteSQLiteIdentifier(credential.table), quoteSQLiteIdentifier(credential.column))
		if err := database.Exec(statement).Error; err != nil {
			return fmt.Errorf("invalidate portable credential %s.%s: %w", credential.table, credential.column, err)
		}
	}
	// Tokens issued by the source instance are never portable credentials.
	if database.Migrator().HasTable("auth_tokens") {
		if err := database.Exec("DELETE FROM auth_tokens").Error; err != nil {
			return fmt.Errorf("invalidate portable auth tokens: %w", err)
		}
	}
	if database.Migrator().HasTable("node_credentials") {
		if err := database.Exec("DELETE FROM node_credentials").Error; err != nil {
			return fmt.Errorf("invalidate portable node credentials: %w", err)
		}
	}
	if database.Migrator().HasTable("nodes") {
		updates := map[string]any{}
		for column, value := range map[string]any{
			"token":                   "",
			"encrypted_legacy_secret": nil,
			"credential_status":       "unpaired",
			"last_credential_use_at":  nil,
		} {
			if database.Migrator().HasColumn("nodes", column) {
				updates[column] = value
			}
		}
		if len(updates) != 0 {
			if err := database.Table("nodes").Where("1 = 1").Updates(updates).Error; err != nil {
				return fmt.Errorf("invalidate portable node authentication state: %w", err)
			}
		}
	}
	if database.Migrator().HasTable("node_controller_credentials") {
		updates := map[string]any{}
		if database.Migrator().HasColumn("node_controller_credentials", "revoked_at") {
			updates["revoked_at"] = gorm.Expr("CURRENT_TIMESTAMP")
		}
		if database.Migrator().HasColumn("node_controller_credentials", "status") {
			updates["status"] = "revoked"
		}
		if len(updates) != 0 {
			query := database.Table("node_controller_credentials")
			if database.Migrator().HasColumn("node_controller_credentials", "revoked_at") {
				query = query.Where("revoked_at IS NULL")
			}
			if err := query.Updates(updates).Error; err != nil {
				return fmt.Errorf("revoke portable controller credentials: %w", err)
			}
		}
	}
	if database.Migrator().HasTable("mcp_service_tokens") && database.Migrator().HasColumn("mcp_service_tokens", "revoked_at") {
		if err := database.Exec("UPDATE mcp_service_tokens SET revoked_at = CURRENT_TIMESTAMP WHERE revoked_at IS NULL").Error; err != nil {
			return fmt.Errorf("revoke portable MCP service tokens: %w", err)
		}
	}
	return nil
}

func validateSQLiteDatabase(databasePath string) error {
	database, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fmt.Errorf("open staged restore database: %w", err)
	}
	sqlDatabase, err := database.DB()
	if err != nil {
		return fmt.Errorf("access staged restore database: %w", err)
	}
	defer sqlDatabase.Close()

	var result string
	if err := database.Raw("PRAGMA integrity_check").Scan(&result).Error; err != nil {
		return fmt.Errorf("validate staged restore database: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("staged restore database failed integrity check: %s", result)
	}
	return nil
}

func quoteSQLiteIdentifier(identifier string) string {
	return `"` + identifier + `"`
}
