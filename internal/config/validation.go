package config

import (
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/uozi-tech/cosy"
)

var allowedConfigBaseNames = map[string]struct{}{
	"nginx.conf":     {},
	"mime.types":     {},
	"fastcgi_params": {},
	"fastcgi.conf":   {},
	"scgi_params":    {},
	"uwsgi_params":   {},
	"proxy_params":   {},
	"koi-utf":        {},
	"koi-win":        {},
	"win-utf":        {},
}

var managedConfigDirs = map[string]struct{}{
	"sites-available":   {},
	"sites-enabled":     {},
	"streams-available": {},
	"streams-enabled":   {},
}

// Keep managed site/stream names flexible for common host-based naming such as
// example.com, while still rejecting obviously unsafe non-config extensions.
var blockedManagedConfigExtensions = map[string]struct{}{
	".html":   {},
	".htm":    {},
	".css":    {},
	".js":     {},
	".jsx":    {},
	".ts":     {},
	".tsx":    {},
	".json":   {},
	".xml":    {},
	".svg":    {},
	".map":    {},
	".woff":   {},
	".woff2":  {},
	".ttf":    {},
	".eot":    {},
	".otf":    {},
	".png":    {},
	".jpg":    {},
	".jpeg":   {},
	".gif":    {},
	".ico":    {},
	".webp":   {},
	".bmp":    {},
	".tiff":   {},
	".avif":   {},
	".zip":    {},
	".tar":    {},
	".gz":     {},
	".bz2":    {},
	".xz":     {},
	".rar":    {},
	".7z":     {},
	".pdf":    {},
	".doc":    {},
	".docx":   {},
	".xls":    {},
	".xlsx":   {},
	".ppt":    {},
	".pptx":   {},
	".mp3":    {},
	".mp4":    {},
	".avi":    {},
	".mov":    {},
	".wmv":    {},
	".flv":    {},
	".webm":   {},
	".ogg":    {},
	".wav":    {},
	".exe":    {},
	".dll":    {},
	".so":     {},
	".dylib":  {},
	".bin":    {},
	".py":     {},
	".rb":     {},
	".php":    {},
	".java":   {},
	".go":     {},
	".rs":     {},
	".c":      {},
	".cpp":    {},
	".h":      {},
	".hpp":    {},
	".sh":     {},
	".bat":    {},
	".ps1":    {},
	".db":     {},
	".sqlite": {},
	".sql":    {},
	".csv":    {},
	".yml":    {},
	".yaml":   {},
	".toml":   {},
	".md":     {},
	".txt":    {},
	".log":    {},
	".lock":   {},
	".pl":     {},
}

func ValidateConfigFile(path string, content string) error {
	if err := ValidateConfigFilename(path); err != nil {
		return err
	}

	return ValidateConfigContent(content)
}

func ValidateConfigFileBytes(path string, content []byte) error {
	if err := ValidateConfigFilename(path); err != nil {
		return err
	}

	return ValidateConfigContentBytes(content)
}

func ValidateConfigFilename(path string) error {
	confPath := filepath.Clean(nginx.GetConfPath())
	cleanPath := filepath.Clean(path)
	if !helper.IsUnderDirectory(cleanPath, confPath) {
		return cosy.WrapErrorWithParams(ErrPathIsNotUnderTheNginxConfDir, cleanPath, confPath)
	}

	baseName := filepath.Base(cleanPath)
	if baseName == "." || baseName == string(filepath.Separator) || baseName == "" {
		return cosy.WrapErrorWithParams(ErrConfigFilenameNotAllowed, baseName)
	}

	lowerBaseName := strings.ToLower(baseName)
	if strings.ToLower(filepath.Ext(baseName)) == ".conf" {
		return nil
	}

	if _, ok := allowedConfigBaseNames[lowerBaseName]; ok {
		return nil
	}

	relativePath, err := filepath.Rel(confPath, cleanPath)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrConfigFilenameNotAllowed, baseName)
	}

	segments := strings.Split(filepath.ToSlash(relativePath), "/")
	if len(segments) == 0 {
		return cosy.WrapErrorWithParams(ErrConfigFilenameNotAllowed, baseName)
	}

	firstSegment := strings.ToLower(segments[0])
	if _, ok := managedConfigDirs[firstSegment]; ok {
		ext := strings.ToLower(filepath.Ext(baseName))
		if ext == "" {
			return nil
		}

		if _, blocked := blockedManagedConfigExtensions[ext]; blocked {
			return cosy.WrapErrorWithParams(ErrConfigFilenameNotAllowed, baseName)
		}

		return nil
	}

	return cosy.WrapErrorWithParams(ErrConfigFilenameNotAllowed, baseName)
}

func ValidateConfigContent(content string) error {
	return ValidateConfigContentBytes([]byte(content))
}

func ValidateConfigContentBytes(content []byte) error {
	if !utf8.Valid(content) {
		return ErrConfigContentMustBeUTF8Text
	}

	for len(content) > 0 {
		r, size := utf8.DecodeRune(content)
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return ErrConfigContentHasControlChars
		}
		content = content[size:]
	}

	return nil
}
