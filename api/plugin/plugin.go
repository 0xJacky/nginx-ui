package plugin

import (
	"context"
	"net/http"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/pkgsign"
	plugin "github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
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

// InspectPlugin reads an uploaded package without installing it. The package
// is kept for a while so the install can refer to it by id.
func InspectPlugin(c *gin.Context) {
	if !settings.PluginSettings.AllowUploads {
		cosy.ErrHandler(c, plugin.ErrUploadsDisabled)
		return
	}
	sweepUploads()

	form, err := readUpload(c)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	defer form.cleanup()
	if form.path == "" {
		cosy.ErrHandler(c, errUploadFileMissing())
		return
	}
	archivePath := form.path

	result, err := plugin.GetManager().Inspect(archivePath)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	// Without an id the install just takes a second upload.
	if id, err := keepUpload(archivePath); err != nil {
		logger.Warn("Keep inspected plugin upload: ", err)
	} else {
		result.UploadID = id
	}
	c.JSON(http.StatusOK, result)
}

// InstallPlugin installs or upgrades a plugin from an uploaded package, or
// from the package an earlier inspect kept under upload_id. A controller that
// pushes a community package names the key that signed it in
// author_public_key, since this node has no catalog entry to find it in.
func InstallPlugin(c *gin.Context) {
	if !settings.PluginSettings.AllowUploads {
		cosy.ErrHandler(c, plugin.ErrUploadsDisabled)
		return
	}
	sweepUploads()

	form, err := readUpload(c)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	defer form.cleanup()

	authorKey := strings.TrimSpace(form.fields["author_public_key"])
	if authorKey != "" {
		if _, err = pkgsign.ParseTrustedKeys([]string{authorKey}); err != nil {
			cosy.ErrHandler(c, cosy.WrapErrorWithParams(plugin.ErrSignatureInvalid, "author_public_key: "+err.Error()))
			return
		}
	}

	archivePath := form.path
	if uploadID := form.fields["upload_id"]; uploadID != "" {
		// The kept package wins over a file sent along with the id.
		var cleanup func()
		archivePath, cleanup, err = takeUpload(uploadID)
		if err != nil {
			cosy.ErrHandler(c, err)
			return
		}
		defer cleanup()
	} else if archivePath == "" {
		cosy.ErrHandler(c, errUploadFileMissing())
		return
	}

	info, err := plugin.GetManager().Install(detach(c), archivePath, plugin.InstallOptions{
		Enable:           form.fields["enable"] == "true",
		ReplaceConflicts: form.fields["replace_conflicts"] == "true",
		AuthorPublicKey:  authorKey,
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
	if err := plugin.GetManager().Uninstall(detach(c), id, c.Query("cascade") == "true"); err != nil {
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
		ReplaceConflicts   bool `json:"replace_conflicts"`
	}
	// The UI sends an empty body when it does not need to approve anything.
	_ = c.ShouldBindJSON(&body)

	info, err := plugin.GetManager().EnableWith(detach(c), id, plugin.EnableOptions{
		ApprovePermissions: body.ApprovePermissions,
		ReplaceConflicts:   body.ReplaceConflicts,
	})
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
	info, err := plugin.GetManager().Disable(detach(c), id)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, info)
}

// SetPluginChannel changes the release channel an installed plugin takes
// updates from.
func SetPluginChannel(c *gin.Context) {
	id, ok := pluginID(c)
	if !ok {
		return
	}

	var body struct {
		Channel string `json:"channel"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	info, err := plugin.GetManager().SetChannel(detach(c), id, body.Channel)
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
	if err := manager.SaveSettings(detach(c), id, body.Settings); err != nil {
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

// GetPluginUsage lists what depends on a plugin on this node, so the UI can
// warn before it is switched off.
func GetPluginUsage(c *gin.Context) {
	id, ok := pluginID(c)
	if !ok {
		return
	}
	usage, err := plugin.GetManager().Usage(c, id)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, usage)
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

// detach keeps a mutation running after the client disconnects, so files and
// database rows are never left half applied. Request values stay reachable.
func detach(c *gin.Context) context.Context {
	return context.WithoutCancel(c)
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
