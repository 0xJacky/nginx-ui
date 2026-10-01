package clustersync

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/snippet"
	"github.com/uozi-tech/cosy/logger"
)

// maxSyncFileSize keeps binaries such as GeoIP databases out of a config sync.
const maxSyncFileSize = 5 << 20

// ConfigFile is a configuration file payload addressed relative to the Nginx
// configuration root.
type ConfigFile struct {
	BaseDir string `json:"base_dir"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

// RelativePath returns the file location relative to the Nginx config root.
func (f ConfigFile) RelativePath() string {
	if f.BaseDir == "" {
		return f.Name
	}
	return strings.TrimPrefix(filepath.ToSlash(filepath.Join(f.BaseDir, f.Name)), "/")
}

// CollectConfigFiles walks a directory below the Nginx configuration root and
// returns every replicable file it contains.
//
// Sites and streams are excluded on purpose: they are replicated as sites and
// streams so their enabled state travels with the content instead of being
// recreated as a plain file in sites-available.
func CollectConfigFiles(root string) ([]ConfigFile, error) {
	files, _, err := CollectConfigFilesReport(root)

	return files, err
}

// CollectConfigFilesReport is CollectConfigFiles that also reports every file it
// left out and why, so the user can be told what was not replicated.
func CollectConfigFilesReport(root string) ([]ConfigFile, []SkippedFile, error) {
	confPath := filepath.Clean(nginx.GetConfPath())
	root = filepath.Clean(root)

	if !helper.IsUnderDirectory(root, confPath) && root != confPath {
		return nil, nil, ErrPathOutsideConfDir
	}

	var files []ConfigFile
	var skipped []SkippedFile
	skip := func(path string, reason SkipReason) {
		relative, err := filepath.Rel(confPath, path)
		if err != nil {
			relative = path
		}
		skipped = append(skipped, SkippedFile{Path: filepath.ToSlash(relative), Reason: reason})
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			// An unreadable entry must not abort the whole directory.
			logger.Debugf("cluster sync skips unreadable entry %s: %v", path, err)
			return nil
		}

		if entry.IsDir() {
			if isManagedDir(confPath, path) {
				return fs.SkipDir
			}
			return nil
		}

		if !entry.Type().IsRegular() {
			return nil
		}

		// The entry configuration is node specific: it names the pid file, the
		// worker user and the include roots of that host. Replicating it in bulk
		// would break a node whose layout differs. It can still be deployed
		// deliberately from the configuration editor.
		if filepath.Clean(path) == filepath.Clean(nginx.GetConfEntryPath()) {
			skip(path, SkipEntryConfig)
			return nil
		}

		// Check the name first so an unsupported file is never read.
		if err := config.ValidateConfigFilename(path); err != nil {
			logger.Debugf("cluster sync skips unsupported config name %s: %v", path, err)
			skip(path, SkipUnsupportedType)
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			logger.Debugf("cluster sync skips unreadable file %s: %v", path, err)
			skip(path, SkipUnreadable)
			return nil
		}
		if info.Size() > maxSyncFileSize {
			logger.Debugf("cluster sync skips oversized file %s", path)
			skip(path, SkipTooLarge)
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			logger.Debugf("cluster sync skips unreadable file %s: %v", path, err)
			skip(path, SkipUnreadable)
			return nil
		}

		if !utf8.Valid(content) {
			logger.Debugf("cluster sync skips non-text file %s", path)
			skip(path, SkipNotText)
			return nil
		}

		relativeDir, err := filepath.Rel(confPath, filepath.Dir(path))
		if err != nil {
			return nil
		}
		if relativeDir == "." {
			relativeDir = ""
		}

		files = append(files, ConfigFile{
			BaseDir: filepath.ToSlash(relativeDir),
			Name:    entry.Name(),
			Content: string(content),
		})

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return files, skipped, nil
}

// isManagedDir reports whether a directory below the config root is owned by the
// site or stream synchronization instead of the plain config synchronization,
// or by software that manages its own snippets on every node.
func isManagedDir(confPath, path string) bool {
	relative, err := filepath.Rel(confPath, path)
	if err != nil {
		return false
	}

	relative = filepath.ToSlash(relative)
	if relative == snippet.DirName+"/"+snippet.ReservedDirName {
		return true
	}
	top := strings.SplitN(relative, "/", 2)[0]

	return strings.HasPrefix(top, "sites-") || strings.HasPrefix(top, "streams-")
}
