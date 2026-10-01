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

// serveStatic resolves one file inside a plugin directory. The id is validated
// before it reaches the filesystem and the resolved path may not leave the
// directory the route is rooted at.
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

	root, err := plugin.GetManager().StaticRoot(id, kind)
	if err != nil {
		// The plugin pages show the icon of a disabled plugin as well.
		root, err = plugin.GetManager().IconRoot(id, kind, rel)
	}
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	target := filepath.Join(root, filepath.FromSlash(rel))
	if !isInside(root, target) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	// A symlink inside the plugin must not escape either, and the plugin
	// directory itself may sit behind one, so both sides are resolved.
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil || !isInside(resolvedRoot, resolved) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	// A manifest may root its bundle at the plugin directory, so the browser
	// routes never hand out the manifest itself or anything runnable.
	if filepath.Base(resolved) == plugin.ManifestFileName || info.Mode().Perm()&0o111 != 0 {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	file, err := os.Open(resolved)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	defer file.Close()

	// Browsers revalidate on every load, so an upgraded plugin shows up
	// without a host restart. ServeContent answers with 304 from the mtime.
	c.Header("Cache-Control", "no-cache")
	// ServeFile would redirect a request for index.html to its directory,
	// which has no route, so the content is served directly.
	http.ServeContent(c.Writer, c.Request, filepath.Base(resolved), info.ModTime(), file)
}

// isInside reports whether target stays under root.
func isInside(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
