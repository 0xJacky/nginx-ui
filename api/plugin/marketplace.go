package plugin

import (
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"github.com/0xJacky/Nginx-UI/internal/middleware"
	plugin "github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// maxMarketplaceSources bounds the source list so one node cannot be turned
// into a crawler by a single settings write.
const maxMarketplaceSources = 16

// marketplaceListResponse is the body of the catalog endpoint.
type marketplaceListResponse struct {
	Plugins []plugin.CatalogEntry  `json:"plugins"`
	Sources []plugin.CatalogSource `json:"sources"`
	// HostPlatform is the "<goos>-<goarch>" key installable_release was
	// resolved for.
	HostPlatform string `json:"host_platform"`
}

// marketplaceDetailResponse pairs one entry with its proxied readme.
type marketplaceDetailResponse struct {
	Plugin       *plugin.CatalogEntry `json:"plugin"`
	Readme       string               `json:"readme"`
	HostPlatform string               `json:"host_platform"`
}

// marketplaceSourcesResponse is the body of both source endpoints.
type marketplaceSourcesResponse struct {
	Sources []plugin.CatalogSource `json:"sources"`
	Default string                 `json:"default"`
}

// InitMarketplaceRouter registers the marketplace endpoints. It is separate
// from InitRouter so the catalog can be mounted next to the local plugin API.
func InitMarketplaceRouter(r *gin.RouterGroup) {
	// The literal segments are registered before the parameterised route so
	// "sources" and "install" never resolve to a plugin id.
	r.GET("/plugins/marketplace", GetMarketplaceList)
	r.GET("/plugins/marketplace/sources", GetMarketplaceSources)
	r.GET("/plugins/updates", GetPluginUpdates)
	r.GET("/plugins/marketplace/:id", GetMarketplacePlugin)

	o := r.Group("", middleware.RequireSecureSession(), middleware.RejectInDemo())
	{
		o.POST("/plugins/marketplace/install", InstallFromMarketplace)
		o.POST("/plugins/marketplace/sources", SaveMarketplaceSources)
		// A probe reads a URL the admin typed, so it needs the same session
		// and checks as saving it.
		o.POST("/plugins/marketplace/sources/probe", ProbeMarketplaceSource)
		o.POST("/plugins/:id/update", UpdatePlugin)
		o.POST("/plugins/:id/replace", ReplacePlugin)
	}
}

// GetMarketplaceList returns the merged catalog, filtered by the query.
func GetMarketplaceList(c *gin.Context) {
	marketplace := plugin.GetManager().Marketplace()

	entries, err := marketplace.Search(c, plugin.CatalogFilter{
		Keyword:  c.Query("keyword"),
		Category: c.Query("category"),
		Source:   c.Query("source"),
		Refresh:  c.Query("refresh") == "true",
	})
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, marketplaceListResponse{
		Plugins:      entries,
		Sources:      marketplace.SourceList(),
		HostPlatform: plugin.HostPlatform(),
	})
}

// GetMarketplacePlugin returns one entry together with its readme.
func GetMarketplacePlugin(c *gin.Context) {
	id, ok := pluginID(c)
	if !ok {
		return
	}

	entry, readme, err := plugin.GetManager().Marketplace().Detail(c, id, c.Query("source"))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, marketplaceDetailResponse{Plugin: entry, Readme: readme, HostPlatform: plugin.HostPlatform()})
}

// InstallFromMarketplace downloads, verifies and installs a catalog release.
func InstallFromMarketplace(c *gin.Context) {
	var body struct {
		ID                 string `json:"id"`
		Version            string `json:"version"`
		Source             string `json:"source"`
		Enable             bool   `json:"enable"`
		ApprovePermissions bool   `json:"approve_permissions"`
		ReplaceConflicts   bool   `json:"replace_conflicts"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	if !plugin.IsValidID(body.ID) {
		cosy.ErrHandler(c, plugin.ErrPluginNotFound)
		return
	}

	info, err := plugin.GetManager().Marketplace().Install(detach(c), body.ID, body.Version, body.Source,
		plugin.InstallOptions{
			Enable:             body.Enable,
			ApprovePermissions: body.ApprovePermissions,
			ReplaceConflicts:   body.ReplaceConflicts,
		})
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, info)
}

// GetPluginUpdates lists the installed plugins with a newer catalog release.
func GetPluginUpdates(c *gin.Context) {
	updates, err := plugin.GetManager().Marketplace().Updates(c)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, updates)
}

// UpdatePlugin upgrades one installed plugin from the catalog, keeping its
// enabled state and its stored settings.
func UpdatePlugin(c *gin.Context) {
	id, ok := pluginID(c)
	if !ok {
		return
	}

	var body struct {
		Version            string `json:"version"`
		ApprovePermissions bool   `json:"approve_permissions"`
	}
	// The UI sends an empty body when it takes the newest release as is.
	_ = c.ShouldBindJSON(&body)

	info, err := plugin.GetManager().Marketplace().Update(detach(c), id, body.Version, body.ApprovePermissions)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, info)
}

// ReplacePlugin swaps an installed plugin for its more trusted marketplace
// package, keeping its settings, data and enabled state.
func ReplacePlugin(c *gin.Context) {
	id, ok := pluginID(c)
	if !ok {
		return
	}

	var body struct {
		Source string `json:"source"`
	}
	_ = c.ShouldBindJSON(&body)

	info, err := plugin.GetManager().Marketplace().Replace(detach(c), id, body.Source)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, info)
}

// GetMarketplaceSources returns the configured catalog URLs.
func GetMarketplaceSources(c *gin.Context) {
	c.JSON(http.StatusOK, marketplaceSourcesResponse{
		Sources: plugin.GetManager().Marketplace().SourceList(),
		Default: settings.DefaultPluginMarketplaceSource,
	})
}

// ProbeMarketplaceSource reads one catalog URL and reports whether it answers,
// the name it declares and how many plugins it lists.
func ProbeMarketplaceSource(c *gin.Context) {
	var body struct {
		URL string `json:"url"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	rawURL, err := validSourceURL(body.URL)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, plugin.GetManager().Marketplace().Probe(c, rawURL))
}

// SaveMarketplaceSources replaces the catalog URL list and drops the cache.
func SaveMarketplaceSources(c *gin.Context) {
	var body struct {
		Sources []string `json:"sources"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	sources, err := normalizeSources(body.Sources)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	if err = settings.Update(func() {
		settings.PluginSettings.MarketplaceSources = sources
	}); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	marketplace := plugin.GetManager().Marketplace()
	marketplace.ClearCache()
	c.JSON(http.StatusOK, marketplaceSourcesResponse{
		Sources: marketplace.SourceList(),
		Default: settings.DefaultPluginMarketplaceSource,
	})
}

// normalizeSources trims, de-duplicates and validates the submitted URLs.
func normalizeSources(raw []string) ([]string, error) {
	sources := make([]string, 0, len(raw))
	for _, item := range raw {
		if strings.TrimSpace(item) == "" {
			continue
		}
		rawURL, err := validSourceURL(item)
		if err != nil {
			return nil, err
		}
		if !contains(sources, rawURL) {
			sources = append(sources, rawURL)
		}
		if len(sources) > maxMarketplaceSources {
			return nil, cosy.WrapErrorWithParams(plugin.ErrCatalogInvalid, "too many sources")
		}
	}
	return sources, nil
}

// validSourceURL trims one catalog URL and checks it is an http(s) URL the
// settings allow.
func validSourceURL(raw string) (string, error) {
	item := strings.TrimSpace(raw)
	parsed, err := url.Parse(item)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") ||
		strings.ContainsFunc(item, unicode.IsSpace) {
		return "", cosy.WrapErrorWithParams(plugin.ErrCatalogInvalid, item)
	}
	if parsed.Scheme == "http" && !settings.PluginSettings.AllowInsecureDownloadURL {
		return "", plugin.ErrInsecureURL
	}
	return item, nil
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
