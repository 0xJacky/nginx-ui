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

var blockedConfigDirectives = map[string]struct{}{
	"js_access":                         {},
	"js_body_filter":                    {},
	"js_content":                        {},
	"js_fetch_buffer_size":              {},
	"js_fetch_ciphers":                  {},
	"js_fetch_keepalive":                {},
	"js_fetch_keepalive_requests":       {},
	"js_fetch_keepalive_time":           {},
	"js_fetch_keepalive_timeout":        {},
	"js_fetch_max_response_buffer_size": {},
	"js_fetch_protocols":                {},
	"js_fetch_proxy":                    {},
	"js_fetch_timeout":                  {},
	"js_fetch_trusted_certificate":      {},
	"js_fetch_verify":                   {},
	"js_fetch_verify_depth":             {},
	"js_filter":                         {},
	"js_header_filter":                  {},
	"js_import":                         {},
	"js_include":                        {},
	"js_load_http_native_module":        {},
	"js_load_stream_native_module":      {},
	"js_path":                           {},
	"js_periodic":                       {},
	"js_preload_object":                 {},
	"js_preread":                        {},
	"js_set":                            {},
	"js_shared_dict_zone":               {},
	"js_var":                            {},
	"lua_package_cpath":                 {},
	"lua_package_path":                  {},
	"perl":                              {},
	"perl_modules":                      {},
	"perl_require":                      {},
	"perl_set":                          {},
}

var blockedDynamicModules = []string{
	"ngx_http_js_module.so",
	"ngx_stream_js_module.so",
	"ngx_http_perl_module.so",
}

type nginxConfigTokenKind int

const (
	nginxConfigTokenWord nginxConfigTokenKind = iota
	nginxConfigTokenSymbol
)

type nginxConfigToken struct {
	kind   nginxConfigTokenKind
	value  string
	symbol rune
}

type nginxConfigLexer struct {
	content []byte
	offset  int
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

	for remaining := content; len(remaining) > 0; {
		r, size := utf8.DecodeRune(remaining)
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return ErrConfigContentHasControlChars
		}
		remaining = remaining[size:]
	}

	if err := ValidateConfigDirectives(content); err != nil {
		return err
	}

	return nil
}

func ValidateConfigDirectives(content []byte) error {
	lexer := &nginxConfigLexer{content: content}
	statement := make([]string, 0, 4)

	for {
		token, ok := lexer.nextToken()
		if !ok {
			return validateConfigStatement(statement)
		}

		if token.kind == nginxConfigTokenWord {
			statement = append(statement, token.value)
			continue
		}

		switch token.symbol {
		case ';', '{':
			if err := validateConfigStatement(statement); err != nil {
				return err
			}
			statement = statement[:0]
		case '}':
			statement = statement[:0]
		}
	}
}

func validateConfigStatement(statement []string) error {
	if len(statement) == 0 {
		return nil
	}

	directive := strings.ToLower(statement[0])
	if directive == "" {
		return nil
	}

	if directive == "user" && configuresRootWorkers(statement) {
		return cosy.WrapErrorWithParams(ErrConfigDirectiveNotAllowed, "user root")
	}

	if directive == "load_module" && loadsRestrictedDynamicModule(statement) {
		return cosy.WrapErrorWithParams(ErrConfigDirectiveNotAllowed, "load_module")
	}

	if isBlockedConfigDirective(directive) {
		return cosy.WrapErrorWithParams(ErrConfigDirectiveNotAllowed, directive)
	}

	return nil
}

func (lexer *nginxConfigLexer) nextToken() (nginxConfigToken, bool) {
	for lexer.offset < len(lexer.content) {
		r, size := utf8.DecodeRune(lexer.content[lexer.offset:])
		if unicode.IsSpace(r) {
			lexer.offset += size
			continue
		}

		if r == '#' {
			lexer.skipComment()
			continue
		}

		if r == ';' || r == '{' || r == '}' {
			lexer.offset += size
			return nginxConfigToken{kind: nginxConfigTokenSymbol, symbol: r}, true
		}

		if r == '"' || r == '\'' {
			return lexer.readQuotedToken(r, size), true
		}

		return lexer.readUnquotedToken(), true
	}

	return nginxConfigToken{}, false
}

func (lexer *nginxConfigLexer) skipComment() {
	for lexer.offset < len(lexer.content) {
		r, size := utf8.DecodeRune(lexer.content[lexer.offset:])
		lexer.offset += size
		if r == '\n' || r == '\r' {
			return
		}
	}
}

func (lexer *nginxConfigLexer) readQuotedToken(quote rune, quoteSize int) nginxConfigToken {
	var builder strings.Builder
	lexer.offset += quoteSize

	for lexer.offset < len(lexer.content) {
		r, size := utf8.DecodeRune(lexer.content[lexer.offset:])
		lexer.offset += size

		if r == quote {
			break
		}

		if r == '\\' && lexer.offset < len(lexer.content) {
			escaped, escapedSize := utf8.DecodeRune(lexer.content[lexer.offset:])
			lexer.offset += escapedSize
			builder.WriteRune(escaped)
			continue
		}

		builder.WriteRune(r)
	}

	return nginxConfigToken{kind: nginxConfigTokenWord, value: builder.String()}
}

func (lexer *nginxConfigLexer) readUnquotedToken() nginxConfigToken {
	var builder strings.Builder

	for lexer.offset < len(lexer.content) {
		r, size := utf8.DecodeRune(lexer.content[lexer.offset:])
		if unicode.IsSpace(r) || r == ';' || r == '{' || r == '}' || r == '#' {
			break
		}

		lexer.offset += size
		if r == '\\' && lexer.offset < len(lexer.content) {
			escaped, escapedSize := utf8.DecodeRune(lexer.content[lexer.offset:])
			lexer.offset += escapedSize
			builder.WriteRune(escaped)
			continue
		}

		builder.WriteRune(r)
	}

	return nginxConfigToken{kind: nginxConfigTokenWord, value: builder.String()}
}

func isBlockedConfigDirective(directive string) bool {
	if strings.HasPrefix(directive, "js_") || strings.HasPrefix(directive, "perl_") {
		return true
	}

	_, blocked := blockedConfigDirectives[directive]
	return blocked
}

func configuresRootWorkers(statement []string) bool {
	if len(statement) < 2 {
		return false
	}

	return strings.EqualFold(statement[1], "root")
}

func loadsRestrictedDynamicModule(statement []string) bool {
	for _, token := range statement[1:] {
		lowerToken := strings.ToLower(token)
		for _, moduleName := range blockedDynamicModules {
			if strings.Contains(lowerToken, strings.ToLower(moduleName)) {
				return true
			}
		}
	}

	return false
}
