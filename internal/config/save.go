package config

import (
	"os"
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
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
		return ErrPathIsNotUnderTheNginxConfDir
	}

	err = CheckAndCreateHistory(absPath, content)
	if err != nil {
		return
	}

	err = os.WriteFile(absPath, []byte(content), 0644)
	if err != nil {
		return
	}

	res := nginx.Control(nginx.Reload)
	if res.IsError() {
		return res.GetError()
	}

	err = SyncToRemoteServer(cfg)
	if err != nil {
		return
	}

	return
}
