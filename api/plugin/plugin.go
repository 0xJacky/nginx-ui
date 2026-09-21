package plugin

import (
	"net/http"
	"os"
	"path/filepath"

	plugin "github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
)

// defaultLogLines is how much of the stderr tail the UI asks for.
const defaultLogLines = 500

// settingsResponse is the body of both settings endpoints.
type settingsResponse struct {
	Schema *protocol.SettingsSchema `json:"schema"`
	Values map[string]any           `json:"values"`
}

// logsResponse is the body of the logs endpoint.
type logsResponse struct {
	Lines []plugin.LogLine `json:"lines"`
}

// GetPluginList returns every plugin this node knows about.
func GetPluginList(c *gin.Context) {
	c.JSON(http.StatusOK, plugin.GetManager().List())
}

// GetSpec advertises the plugin protocol this node implements.
func GetSpec(c *gin.Context) {
	c.JSON(http.StatusOK, plugin.GetManager().Spec())
}

// GetWebappEntries lists the browser bundles the UI has to load.
func GetWebappEntries(c *gin.Context) {
	c.JSON(http.StatusOK, plugin.GetManager().WebappEntries())
}

// InspectPlugin reads an uploaded package without installing it.
func InspectPlugin(c *gin.Context) {
	if !settings.PluginSettings.AllowUploads {
		cosy.ErrHandler(c, plugin.ErrUploadsDisabled)
		return
	}

	archivePath, cleanup, err := saveUpload(c)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	defer cleanup()

	result, err := plugin.GetManager().Inspect(archivePath)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// InstallPlugin installs or upgrades a plugin from an uploaded package.
func InstallPlugin(c *gin.Context) {
	if !settings.PluginSettings.AllowUploads {
		cosy.ErrHandler(c, plugin.ErrUploadsDisabled)
		return
	}

	archivePath, cleanup, err := saveUpload(c)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	defer cleanup()

	info, err := plugin.GetManager().Install(c, archivePath, plugin.InstallOptions{
		Enable: c.PostForm("enable") == "true",
	})
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, info)
}

// UninstallPlugin removes a plugin, its data directory and its database rows.
func UninstallPlugin(c *gin.Context) {
	id, ok := pluginID(c)
	if !ok {
		return
	}
	if err := plugin.GetManager().Uninstall(c, id, c.Query("cascade") == "true"); err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{})
}

// EnablePlugin turns a plugin on, optionally approving its permission set.
func EnablePlugin(c *gin.Context) {
	id, ok := pluginID(c)
	if !ok {
		return
	}

	var body struct {
		ApprovePermissions bool `json:"approve_permissions"`
	}
	// The UI sends an empty body when it does not need to approve anything.
	_ = c.ShouldBindJSON(&body)

	info, err := plugin.GetManager().Enable(c, id, body.ApprovePermissions)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, info)
}

// DisablePlugin turns a plugin off together with whatever requires it.
func DisablePlugin(c *gin.Context) {
	id, ok := pluginID(c)
	if !ok {
		return
	}
	info, err := plugin.GetManager().Disable(c, id)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, info)
}

// GetPluginSettings returns the settings form and the stored values, with
// every secret replaced by the redaction placeholder.
func GetPluginSettings(c *gin.Context) {
	id, ok := pluginID(c)
	if !ok {
		return
	}
	schema, values, err := plugin.GetManager().Settings(id)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, settingsResponse{Schema: schema, Values: values})
}

// SavePluginSettings validates and stores the settings, then pushes them into
// a running plugin.
func SavePluginSettings(c *gin.Context) {
	id, ok := pluginID(c)
	if !ok {
		return
	}

	var body struct {
		Settings map[string]any `json:"settings"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	manager := plugin.GetManager()
	if err := manager.SaveSettings(c, id, body.Settings); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	schema, values, err := manager.Settings(id)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, settingsResponse{Schema: schema, Values: values})
}

// GetPluginLogs returns the tail of the plugin stderr buffer.
func GetPluginLogs(c *gin.Context) {
	id, ok := pluginID(c)
	if !ok {
		return
	}

	manager := plugin.GetManager()
	if _, err := manager.Get(id); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	lines := manager.Logs(id)
	limit := defaultLogLines
	if requested := cast.ToInt(c.Query("lines")); requested > 0 {
		limit = requested
	}
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	if lines == nil {
		lines = []plugin.LogLine{}
	}
	c.JSON(http.StatusOK, logsResponse{Lines: lines})
}

// pluginID reads and validates the :id route parameter.
func pluginID(c *gin.Context) (string, bool) {
	id := c.Param("id")
	if !plugin.IsValidID(id) {
		cosy.ErrHandler(c, plugin.ErrPluginNotFound)
		return "", false
	}
	return id, true
}

// saveUpload stores the multipart package in a temporary file.
func saveUpload(c *gin.Context) (string, func(), error) {
	noop := func() {}
	header, err := c.FormFile("file")
	if err != nil {
		return "", noop, cosy.WrapErrorWithParams(plugin.ErrPackageInvalid, err.Error())
	}

	dir, err := os.MkdirTemp("", "nginx-ui-plugin-upload-")
	if err != nil {
		return "", noop, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	target := filepath.Join(dir, "package.tar.gz")
	if err = c.SaveUploadedFile(header, target); err != nil {
		cleanup()
		return "", noop, err
	}
	return target, cleanup, nil
}
