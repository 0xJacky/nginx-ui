package config

import (
	"os"

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
		})).Where(q.Filepath.Eq(absPath)).FirstOrCreate()
		if err != nil {
			return
		}
	}

	if !helper.IsUnderDirectory(absPath, nginx.GetConfPath()) {
		return ErrPathIsNotUnderTheNginxConfDir
	}

	origContent, err := os.ReadFile(absPath)
	if err != nil {
		return
	}

	if content == string(origContent) {
		return
	}

	err = CheckAndCreateHistory(absPath, content)
	if err != nil {
		return
	}

	err = os.WriteFile(absPath, []byte(content), 0644)
	if err != nil {
		return
	}

	output := nginx.Reload()
	if nginx.GetLogLevel(output) >= nginx.Warn {
		return cosy.WrapErrorWithParams(ErrNginxReloadFailed, output)
	}

	err = SyncToRemoteServer(cfg)
	if err != nil {
		return
	}

	return
}
