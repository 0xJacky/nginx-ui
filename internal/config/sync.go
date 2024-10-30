package config

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/internal/transport"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy/logger"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type SyncConfigPayload struct {
	Name        string `json:"name"`
	Filepath    string `json:"filepath"`
	NewFilepath string `json:"new_filepath"`
	Content     string `json:"content"`
	Overwrite   bool   `json:"overwrite"`
}

func SyncToRemoteServer(c *model.Config, newFilepath string) (err error) {
	if c.Filepath == "" || len(c.SyncNodeIds) == 0 {
		return
	}

	nginxConfPath := nginx.GetConfPath()
	if !helper.IsUnderDirectory(c.Filepath, nginxConfPath) {
		return fmt.Errorf("config: %s is not under the nginx conf path: %s",
			c.Filepath, nginxConfPath)
	}

	if newFilepath != "" && !helper.IsUnderDirectory(newFilepath, nginxConfPath) {
		return fmt.Errorf("config: %s is not under the nginx conf path: %s",
			c.Filepath, nginxConfPath)
	}

	currentPath := c.Filepath
	if newFilepath != "" {
		currentPath = newFilepath
	}
	configBytes, err := os.ReadFile(currentPath)
	if err != nil {
		return
	}

	payload := &SyncConfigPayload{
		Name:        c.Name,
		Filepath:    c.Filepath,
		NewFilepath: newFilepath,
		Content:     string(configBytes),
		Overwrite:   c.SyncOverwrite,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return
	}

	q := query.Environment
	envs, _ := q.Where(q.ID.In(c.SyncNodeIds...)).Find()
	for _, env := range envs {
		go func() {
			err := payload.deploy(env, c, payloadBytes)
			if err != nil {
				logger.Error(err)
			}
		}()
	}

	return
}

func SyncRenameOnRemoteServer(origPath, newPath string, syncNodeIds []uint64) (err error) {
	if origPath == "" || newPath == "" || len(syncNodeIds) == 0 {
		return
	}

	nginxConfPath := nginx.GetConfPath()
	if !helper.IsUnderDirectory(origPath, nginxConfPath) {
		return fmt.Errorf("config: %s is not under the nginx conf path: %s",
			origPath, nginxConfPath)
	}

	if !helper.IsUnderDirectory(newPath, nginxConfPath) {
		return fmt.Errorf("config: %s is not under the nginx conf path: %s",
			newPath, nginxConfPath)
	}

	payload := &RenameConfigPayload{
		Filepath:    origPath,
		NewFilepath: newPath,
	}

	q := query.Environment
	envs, _ := q.Where(q.ID.In(syncNodeIds...)).Find()
	for _, env := range envs {
		go func() {
			err := payload.rename(env)
			if err != nil {
				logger.Error(err)
			}
		}()
	}

	return
}

type SyncNotificationPayload struct {
	StatusCode int    `json:"status_code"`
	ConfigName string `json:"config_name"`
	EnvName    string `json:"env_name"`
	RespBody   string `json:"resp_body"`
}

func (p *SyncConfigPayload) deploy(env *model.Environment, c *model.Config, payloadBytes []byte) (err error) {
	t, err := transport.NewTransport()
	if err != nil {
		return
	}
	client := http.Client{
		Transport: t,
	}
	url, err := env.GetUrl("/api/configs")
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return
	}
	req.Header.Set("X-Node-Secret", env.Token)
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	notificationPayload := &SyncNotificationPayload{
		StatusCode: resp.StatusCode,
		ConfigName: c.Name,
		EnvName:    env.Name,
		RespBody:   string(respBody),
	}

	notificationPayloadBytes, err := json.Marshal(notificationPayload)
	if err != nil {
		return
	}

	if resp.StatusCode != http.StatusOK {
		notification.Error("Sync Config Error", string(notificationPayloadBytes))
		return
	}

	notification.Success("Sync Config Success", string(notificationPayloadBytes))

	// handle rename
	if p.NewFilepath == "" || p.Filepath == p.NewFilepath {
		return
	}

	payload := &RenameConfigPayload{
		Filepath:    p.Filepath,
		NewFilepath: p.NewFilepath,
	}

	err = payload.rename(env)

	return
}

type RenameConfigPayload struct {
	Filepath    string `json:"filepath"`
	NewFilepath string `json:"new_filepath"`
}

type SyncRenameNotificationPayload struct {
	StatusCode int    `json:"status_code"`
	OrigPath   string `json:"orig_path"`
	NewPath    string `json:"new_path"`
	EnvName    string `json:"env_name"`
	RespBody   string `json:"resp_body"`
}

func (p *RenameConfigPayload) rename(env *model.Environment) (err error) {
	// handle rename
	if p.NewFilepath == "" || p.Filepath == p.NewFilepath {
		return
	}

	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: settings.HTTPSettings.InsecureSkipVerify},
		},
	}

	payloadBytes, err := json.Marshal(gin.H{
		"base_path": strings.ReplaceAll(filepath.Dir(p.Filepath), nginx.GetConfPath(), ""),
		"orig_name": filepath.Base(p.Filepath),
		"new_name":  filepath.Base(p.NewFilepath),
	})
	if err != nil {
		return
	}
	url, err := env.GetUrl("/api/config_rename")
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return
	}
	req.Header.Set("X-Node-Secret", env.Token)
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	notificationPayload := &SyncRenameNotificationPayload{
		StatusCode: resp.StatusCode,
		OrigPath:   p.Filepath,
		NewPath:    p.NewFilepath,
		EnvName:    env.Name,
		RespBody:   string(respBody),
	}

	notificationPayloadBytes, err := json.Marshal(notificationPayload)
	if err != nil {
		return
	}

	if resp.StatusCode != http.StatusOK {
		notification.Error("Rename Remote Config Error", string(notificationPayloadBytes))
		return
	}

	notification.Success("Rename Remote Config Success", string(notificationPayloadBytes))

	return
}
