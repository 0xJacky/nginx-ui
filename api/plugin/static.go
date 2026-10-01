package plugin

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	plugin "github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/gin-gonic/gin"
)

// ServeWebapp serves the browser bundle of an enabled plugin.
func ServeWebapp(c *gin.Context) {
	serveStatic(c, "webapp")
}

// ServePage serves a zero-build iframe page of an enabled plugin.
func ServePage(c *gin.Context) {
	serveStatic(c, "pages")
}

// serveStatic serves one file inside a plugin directory. The id is validated
// before it reaches the filesystem, and the file is opened through an
// os.Root, so neither the path nor a symlink can lead outside the directory
// the route is rooted at.
func serveStatic(c *gin.Context, kind string) {
	id := c.Param("id")
	if !plugin.IsValidID(id) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	rel := strings.TrimPrefix(c.Param("filepath"), "/")
	if rel == "" || strings.ContainsRune(rel, 0) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	dir, err := plugin.GetManager().StaticRoot(id, kind)
	if err != nil {
		// The plugin pages show the icon of a disabled plugin as well.
		dir, err = plugin.GetManager().IconRoot(id, kind, rel)
	}
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	defer root.Close()

	file, err := root.Open(filepath.FromSlash(rel))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	// A manifest may root its bundle at the plugin directory, so the browser
	// routes never hand out the manifest itself, under any name, or anything
	// runnable.
	if manifest, err := root.Stat(plugin.ManifestFileName); err == nil && os.SameFile(info, manifest) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if info.Mode().Perm()&0o111 != 0 {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	// Browsers revalidate on every load, so an upgraded plugin shows up
	// without a host restart. ServeContent answers with 304 from the mtime.
	c.Header("Cache-Control", "no-cache")
	// ServeFile would redirect a request for index.html to its directory,
	// which has no route, so the content is served directly.
	http.ServeContent(c.Writer, c.Request, info.Name(), info.ModTime(), file)
}
