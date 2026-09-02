package config

import (
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy"
	"gorm.io/gen/field"
)

func Save(absPath string, content string, cfg *model.Config) (err error) {
	q := query.Config
	if cfg == nil {
		cfg, err = q.Assign(field.Attrs(&model.Config{
			Filepath: absPath,
			Name:     filepath.Base(absPath),
		})).Where(q.Filepath.Eq(absPath)).FirstOrCreate()
		if err != nil {
			return
		}
	}

	if !helper.IsUnderDirectory(absPath, nginx.GetConfPath()) {
		return cosy.WrapErrorWithParams(ErrPathIsNotUnderTheNginxConfDir, absPath, nginx.GetConfPath())
	}

	err = ValidateConfigFile(absPath, content)
	if err != nil {
		return
	}

	err = CheckAndCreateHistory(absPath, content)
	if err != nil {
		return
	}

	// Hold the apply lock for the whole write -> test -> reload sequence so a
	// concurrent mutation cannot make this save fail on somebody else's file.
	release := LockApply()
	defer release()

	tx := &FileTransaction{}
	if err = tx.Write(absPath, []byte(content), 0644); err != nil {
		return RollbackError(err, tx.Rollback)
	}

	// Reloading without `nginx -t` would leave content Nginx rejects on disk:
	// the running instance keeps its valid in-memory configuration, so the
	// failure is deferred to the next Nginx start. Test first and roll back.
	if err = tx.TestAndReload(); err != nil {
		return
	}

	err = SyncToRemoteServer(cfg)
	if err != nil {
		return
	}

	return
}
