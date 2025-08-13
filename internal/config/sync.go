package config

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/internal/transport"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy/logger"
)

type SyncConfigPayload struct {
	Name      string `json:"name" binding:"required"`
	BaseDir   string `json:"base_dir"`
	Content   string `json:"content"`
	Overwrite bool   `json:"overwrite"`
}

func SyncToRemoteServer(c *model.Config) (err error) {
	if c.Filepath == "" || len(c.SyncNodeIds) == 0 {
		return
	}

	nginxConfPath := nginx.GetConfPath()
	if !helper.IsUnderDirectory(c.Filepath, nginxConfPath) {
		return e.NewWithParams(50006, ErrPathIsNotUnderTheNginxConfDir.Error(), c.Filepath, nginxConfPath)
	}

	configBytes, err := os.ReadFile(c.Filepath)
	if err != nil {
		return
	}

	payload := &SyncConfigPayload{
		Name:      c.Name,
		BaseDir:   strings.ReplaceAll(filepath.Dir(c.Filepath), nginx.GetConfPath(), ""),
		Content:   string(configBytes),
		Overwrite: c.SyncOverwrite,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return
	}

	q := query.Node
	nodes, _ := q.Where(q.ID.In(c.SyncNodeIds...), q.Enabled.Is(true)).Find()
	for _, node := range nodes {
		go func() {
			err := payload.deploy(node, c, payloadBytes)
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
		return e.NewWithParams(50006, ErrPathIsNotUnderTheNginxConfDir.Error(), origPath, nginxConfPath)
	}

	if !helper.IsUnderDirectory(newPath, nginxConfPath) {
		return e.NewWithParams(50006, ErrPathIsNotUnderTheNginxConfDir.Error(), newPath, nginxConfPath)
	}

	payload := &RenameConfigPayload{
		Filepath:    origPath,
		NewFilepath: newPath,
	}

	q := query.Node
	nodes, _ := q.Where(q.ID.In(syncNodeIds...)).Find()
	for _, node := range nodes {
		go func() {
			err := payload.rename(node)
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
	NodeName    string `json:"node_name"`
	Response   string `json:"response"`
}

func (p *SyncConfigPayload) deploy(node *model.Node, c *model.Config, payloadBytes []byte) (err error) {
	t, err := transport.NewTransport()
	if err != nil {
		return
	}
	client := http.Client{
		Transport: t,
	}
	url, err := node.GetUrl("/api/configs")
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return
	}
	req.Header.Set("X-Node-Secret", node.Token)
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
		NodeName:    node.Name,
		Response:   string(respBody),
	}

	if resp.StatusCode != http.StatusOK {
		notification.Error("Sync Config Error", "Sync config %{config_name} to %{node_name} failed", notificationPayload)
		return
	}

	notification.Success("Sync Config Success", "Sync config %{config_name} to %{node_name} successfully", notificationPayload)

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
	NodeName    string `json:"node_name"`
	Response   string `json:"response"`
}

func (p *RenameConfigPayload) rename(node *model.Node) (err error) {
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
	url, err := node.GetUrl("/api/config_rename")
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return
	}
	req.Header.Set("X-Node-Secret", node.Token)
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
		NodeName:    node.Name,
		Response:   string(respBody),
	}

	if resp.StatusCode != http.StatusOK {
		notification.Error("Rename Remote Config Error", "Rename %{orig_path} to %{new_path} on %{node_name} failed", notificationPayload)
		return
	}

	notification.Success("Rename Remote Config Success", "Rename %{orig_path} to %{new_path} on %{node_name} successfully", notificationPayload)

	return
}

func SyncDeleteOnRemoteServer(deletePath string, syncNodeIds []uint64) (err error) {
	if deletePath == "" || len(syncNodeIds) == 0 {
		return
	}

	nginxConfPath := nginx.GetConfPath()
	if !helper.IsUnderDirectory(deletePath, nginxConfPath) {
		return e.NewWithParams(50006, ErrPathIsNotUnderTheNginxConfDir.Error(), deletePath, nginxConfPath)
	}

	payload := &DeleteConfigPayload{
		Filepath: deletePath,
	}

	q := query.Node
	nodes, _ := q.Where(q.ID.In(syncNodeIds...)).Find()
	for _, node := range nodes {
		go func() {
			err := payload.delete(node)
			if err != nil {
				logger.Error(err)
			}
		}()
	}

	return
}

type DeleteConfigPayload struct {
	Filepath string `json:"filepath"`
}

type SyncDeleteNotificationPayload struct {
	StatusCode int    `json:"status_code"`
	Path       string `json:"path"`
	NodeName    string `json:"node_name"`
	Response   string `json:"response"`
}

func (p *DeleteConfigPayload) delete(node *model.Node) (err error) {
	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: settings.HTTPSettings.InsecureSkipVerify},
		},
	}

	payloadBytes, err := json.Marshal(gin.H{
		"base_path": strings.ReplaceAll(filepath.Dir(p.Filepath), nginx.GetConfPath(), ""),
		"name":      filepath.Base(p.Filepath),
	})
	if err != nil {
		return
	}

	url, err := node.GetUrl("/api/config_delete")
	if err != nil {
		return
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return
	}
	req.Header.Set("X-Node-Secret", node.Token)
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	notificationPayload := &SyncDeleteNotificationPayload{
		StatusCode: resp.StatusCode,
		Path:       p.Filepath,
		NodeName:    node.Name,
		Response:   string(respBody),
	}

	if resp.StatusCode != http.StatusOK {
		notification.Error("Delete Remote Config Error", "Delete %{path} on %{node_name} failed", notificationPayload)
		return
	}

	notification.Success("Delete Remote Config Success", "Delete %{path} on %{node_name} successfully", notificationPayload)

	return
}
