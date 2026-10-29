package config

import (
	"bytes"
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

// dataFileExtensions are non-directive files that live next to the
// configuration, such as TLS certificates, keys and basic auth files. They are
// allowed outside the managed directories but never parsed as Nginx directives.
var dataFileExtensions = map[string]struct{}{
	".pem":       {},
	".crt":       {},
	".cer":       {},
	".key":       {},
	".csr":       {},
	".crl":       {},
	".ca-bundle": {},
	".htpasswd":  {},
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

// nginxConfigLexer splits a configuration into the tokens nginx itself reads,
// following ngx_conf_read_token in src/core/ngx_conf_file.c. The denylist is
// only as good as this agreement: wherever the lexer ends a token, starts a
// comment or opens a quote where nginx does not, a statement can be hidden
// from the validator while nginx still runs it.
type nginxConfigLexer struct {
	content []byte
	offset  int
	// tokenStart mirrors nginx's last_space flag: the next byte may begin a
	// new token. Only there does '#' start a comment and a quote open a
	// quoted token.
	tokenStart bool
}

func ValidateConfigFile(path string, content string) error {
	if err := ValidateConfigFilename(path); err != nil {
		return err
	}

	if IsDataFile(path) {
		return ValidateDataFileContent(path, []byte(content))
	}

	return validateConfigIncludes(path, []byte(content))
}

func ValidateConfigFileBytes(path string, content []byte) error {
	if err := ValidateConfigFilename(path); err != nil {
		return err
	}

	if IsDataFile(path) {
		return ValidateDataFileContent(path, content)
	}

	return validateConfigIncludes(path, content)
}

// IsDataFile reports whether path is a certificate, key or similar data file
// outside the site and stream directories. Such files are stored verbatim.
func IsDataFile(path string) bool {
	confPath := filepath.Clean(nginx.GetConfPath())
	cleanPath := filepath.Clean(path)
	if !helper.IsUnderDirectory(cleanPath, confPath) {
		return false
	}

	if _, ok := dataFileExtensions[strings.ToLower(filepath.Ext(cleanPath))]; !ok {
		return false
	}

	relativePath, err := filepath.Rel(confPath, cleanPath)
	if err != nil {
		return false
	}

	firstSegment := strings.ToLower(strings.SplitN(filepath.ToSlash(relativePath), "/", 2)[0])
	_, managed := managedConfigDirs[firstSegment]

	return !managed
}

// ValidateDataContent only checks that a data file is plain text without
// control characters, since it holds no Nginx directives.
func ValidateDataContent(content []byte) error {
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

	return nil
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

	if IsDataFile(cleanPath) {
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
	if err := ValidateDataContent(content); err != nil {
		return err
	}

	if err := ValidateConfigDirectives(content); err != nil {
		return err
	}

	return nil
}

func ValidateConfigDirectives(content []byte) error {
	return visitConfigStatements(content, validateConfigStatement)
}

func visitConfigStatements(content []byte, visit func([]string) error) error {
	lexer := &nginxConfigLexer{content: content, tokenStart: true}
	statement := make([]string, 0, 4)

	for {
		token, ok := lexer.nextToken()
		if !ok {
			return visit(statement)
		}

		if token.kind == nginxConfigTokenWord {
			statement = append(statement, token.value)
			continue
		}

		switch token.symbol {
		case ';', '{':
			if err := visit(statement); err != nil {
				return err
			}
			opensLuaBlock := token.symbol == '{' && isLuaBlockDirective(statement)
			statement = statement[:0]
			if opensLuaBlock {
				lexer.skipLuaBlock()
			}
		case '}':
			// nginx rejects a '}' that arrives while a statement is still
			// open ("unexpected }"), so checking the pending words can only
			// reject configs nginx refuses as well.
			if err := visit(statement); err != nil {
				return err
			}
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

// isNginxConfigSpace reports the characters ngx_conf_read_token treats as
// whitespace: space, tab, CR and LF. Other Unicode spaces such as U+00A0 are
// ordinary token characters for nginx, so they must not end a token here.
func isNginxConfigSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\r' || r == '\n'
}

func (lexer *nginxConfigLexer) nextToken() (nginxConfigToken, bool) {
	for lexer.offset < len(lexer.content) {
		r, size := utf8.DecodeRune(lexer.content[lexer.offset:])
		if isNginxConfigSpace(r) {
			lexer.offset += size
			lexer.tokenStart = true
			continue
		}

		// nginx only enters sharp_comment when '#' begins a token: at the
		// start of the file, after whitespace, or after ';', '{' or '}'.
		// Inside a token '#' is an ordinary character.
		if r == '#' && lexer.tokenStart {
			lexer.skipComment()
			continue
		}

		// ';' and '{' end a statement wherever they appear outside quotes.
		// '}' only reaches this point at a token start, since nginx treats a
		// '}' inside a token as an ordinary character.
		if r == ';' || r == '{' || r == '}' {
			lexer.offset += size
			lexer.tokenStart = true
			return nginxConfigToken{kind: nginxConfigTokenSymbol, symbol: r}, true
		}

		// A quote opens a quoted token only at a token start; elsewhere it
		// is an ordinary character. Right after a closing quote nginx
		// requires whitespace, ';', '{' or ')' and rejects anything else
		// ("unexpected ..."). Reading what follows as an unquoted token, in
		// which '#' is literal, therefore only differs on configs nginx
		// refuses, and never turns it into a comment that hides statements.
		atTokenStart := lexer.tokenStart
		lexer.tokenStart = false
		if atTokenStart && (r == '"' || r == '\'') {
			return lexer.readQuotedToken(r, size), true
		}

		return lexer.readUnquotedToken(), true
	}

	return nginxConfigToken{}, false
}

// isLuaBlockDirective reports a statement that opens an ngx_lua code block,
// such as content_by_lua_block or set_by_lua_block $var.
func isLuaBlockDirective(statement []string) bool {
	return len(statement) > 0 && strings.HasSuffix(strings.ToLower(statement[0]), "_by_lua_block")
}

// skipLuaBlock consumes a *_by_lua_block body up to its closing '}'. ngx_lua
// reads these bodies with its own Lua lexer (ngx_http_lua_conf_lua_block_parse),
// not ngx_conf_read_token: quoted strings, long brackets and "--" comments are
// opaque and only unquoted braces nest. On stock nginx the directive is unknown,
// so nginx -t fails before anything in the body runs. Tokenizing the body with
// nginx rules would misread ordinary Lua such as `ngx.say("a ", x)` or a table
// field named js_enabled; skipping it the way ngx_lua does keeps the block
// boundary where OpenResty puts it.
func (lexer *nginxConfigLexer) skipLuaBlock() {
	content := lexer.content
	depth := 0
	for lexer.offset < len(content) {
		c := content[lexer.offset]
		switch {
		case c == '-' && lexer.offset+1 < len(content) && content[lexer.offset+1] == '-':
			lexer.offset += 2
			if level, ok := luaLongBracketLevel(content, lexer.offset); ok {
				lexer.skipLuaLongBracket(level)
				continue
			}
			for lexer.offset < len(content) && content[lexer.offset] != '\n' {
				lexer.offset++
			}
		case c == '"' || c == '\'':
			lexer.offset++
			for lexer.offset < len(content) && content[lexer.offset] != c && content[lexer.offset] != '\n' {
				if content[lexer.offset] == '\\' {
					lexer.offset++
				}
				lexer.offset++
			}
			lexer.offset++
		case c == '[':
			if level, ok := luaLongBracketLevel(content, lexer.offset); ok {
				lexer.skipLuaLongBracket(level)
				continue
			}
			lexer.offset++
		case c == '{':
			depth++
			lexer.offset++
		case c == '}':
			lexer.offset++
			if depth == 0 {
				lexer.tokenStart = true
				return
			}
			depth--
		default:
			lexer.offset++
		}
	}
	lexer.tokenStart = true
}

// luaLongBracketLevel reports whether a Lua long bracket ("[[", "[=[", ...)
// opens at offset, and its level.
func luaLongBracketLevel(content []byte, offset int) (int, bool) {
	if offset >= len(content) || content[offset] != '[' {
		return 0, false
	}
	level := 0
	for i := offset + 1; i < len(content); i++ {
		switch content[i] {
		case '=':
			level++
		case '[':
			return level, true
		default:
			return 0, false
		}
	}
	return 0, false
}

// skipLuaLongBracket consumes a long bracket of the given level, opening
// bracket included, up to its matching close.
func (lexer *nginxConfigLexer) skipLuaLongBracket(level int) {
	closing := "]" + strings.Repeat("=", level) + "]"
	lexer.offset += level + 2
	end := bytes.Index(lexer.content[lexer.offset:], []byte(closing))
	if end < 0 {
		lexer.offset = len(lexer.content)
		return
	}
	lexer.offset += end + len(closing)
}

// skipComment drops a comment up to the line feed. nginx clears
// sharp_comment only on LF, so a bare CR does not end a comment.
func (lexer *nginxConfigLexer) skipComment() {
	for lexer.offset < len(lexer.content) {
		r, size := utf8.DecodeRune(lexer.content[lexer.offset:])
		lexer.offset += size
		if r == '\n' {
			lexer.tokenStart = true
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

// readUnquotedToken reads a token up to the characters that end one in
// ngx_conf_read_token: whitespace, ';' and '{'. '#', '}' and quotes are
// ordinary characters inside a token, and a backslash keeps the next
// character in it. A '{' right after '$' also stays in the token: nginx sets
// its variable flag on '$' and skips '{' while the flag is set, which keeps
// "${name}" intact.
//
// An escape drops the backslash before every character, while nginx keeps it
// unless a quote, a backslash, t, r or n follows. That only makes the words
// the denylist compares stricter; token boundaries are the same.
func (lexer *nginxConfigLexer) readUnquotedToken() nginxConfigToken {
	var builder strings.Builder
	variable := false

	for lexer.offset < len(lexer.content) {
		r, size := utf8.DecodeRune(lexer.content[lexer.offset:])

		// The flag stays set across consecutive '{', as in nginx.
		if r == '{' && variable {
			lexer.offset += size
			builder.WriteRune(r)
			continue
		}
		variable = r == '$'

		if isNginxConfigSpace(r) || r == ';' || r == '{' {
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

// isBlockedConfigDirective implements a defense-in-depth denylist for
// stock-nginx deployments, not a security boundary: whoever can write the
// Nginx configuration is responsible for what that deployment runs. Lua
// code-execution directives (*_by_lua*, loading the lua module) are
// intentionally allowed for OpenResty compatibility — those deployments
// manage real Lua through the UI, and rejecting the directives would make
// their existing configs uneditable. The "allow existing lua block
// directives" case in validation_test.go codifies this.
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
