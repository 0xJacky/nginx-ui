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

// RestrictedDirective names one restricted statement and the configuration
// file it was found in.
type RestrictedDirective struct {
	// Directive is the statement's name, such as "js_import", or
	// "load_module" for a restricted dynamic module.
	Directive string `json:"directive"`
	// File is the configuration file holding the statement: the file being
	// saved for a statement in the edited content, and the included file
	// otherwise.
	File string `json:"file"`
}

// RestrictedDirectiveError marks a validation that failed only because the
// configuration holds a restricted js_*, perl* or load_module statement.
// Such a statement loads code into the Nginx worker, which an interactive
// administrator may still have a reason to deploy, so the API turns this
// error into a two-factor step-up instead of a flat rejection. It wraps the
// same cosy error as before, so every caller that cannot step up — a cluster
// node, a service token, MCP — keeps exactly the response it had.
//
// "user root" is rejected without this type on purpose: it changes the
// identity the worker processes run as rather than the code the
// configuration loads, and no step-up unlocks it.
type RestrictedDirectiveError struct {
	// Found is the statement that stopped the validation.
	Found RestrictedDirective
	err   error
}

func (e *RestrictedDirectiveError) Error() string {
	return e.err.Error()
}

func (e *RestrictedDirectiveError) Unwrap() error {
	return e.err
}

// validateOptions carries the per-request decisions a caller may make about
// the directive denylist. Its zero value is the behavior every caller had
// before the step-up existed.
type validateOptions struct {
	// allowRestricted keeps the restricted js_*, perl* and load_module
	// statements instead of rejecting them. Only a request that passed the
	// two-factor step-up may set it.
	allowRestricted bool
	// found collects every restricted statement the walk meets, including
	// the ones allowRestricted lets through, so a caller can report what a
	// save would enable.
	found *[]RestrictedDirective
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
	// nginxEscapes resolves backslash escapes into the exact value
	// ngx_conf_read_token gives the directive, instead of dropping every
	// backslash. Include arguments need it: they name files, and a file
	// reached through a different name would escape validation.
	nginxEscapes bool
}

func ValidateConfigFile(path string, content string) error {
	return validateConfigFile(path, []byte(content), validateOptions{})
}

func ValidateConfigFileBytes(path string, content []byte) error {
	return validateConfigFile(path, content, validateOptions{})
}

// ValidateConfigFileWithRestricted is ValidateConfigFile with the restricted
// js_*, perl* and load_module denylist lifted when allowRestricted is set.
// Only a request that passed the two-factor step-up may set it; every other
// caller keeps using ValidateConfigFile.
func ValidateConfigFileWithRestricted(path string, content string, allowRestricted bool) error {
	return validateConfigFile(path, []byte(content), validateOptions{allowRestricted: allowRestricted})
}

// FindRestrictedDirectives reports the restricted statements in content and
// in the files it includes, in the order Nginx reads them and without
// repeating a directive from the same file. It walks exactly as validation
// does, so it names the files validation reaches.
//
// The second value is false when the walk could not finish — an include that
// changed since the content was validated, a limit, a cycle — so a caller can
// tell "this configuration holds none" from "this walk could not say". An
// audit record must not read an aborted walk as a clean one.
func FindRestrictedDirectives(path string, content string) ([]RestrictedDirective, bool) {
	found := make([]RestrictedDirective, 0, 1)
	err := validateConfigFile(path, []byte(content), validateOptions{allowRestricted: true, found: &found})

	return dedupeRestrictedDirectives(found), err == nil
}

func validateConfigFile(path string, content []byte, opts validateOptions) error {
	if err := ValidateConfigFilename(path); err != nil {
		return err
	}

	if IsDataFile(path) {
		return ValidateDataFileContent(path, content)
	}

	return validateConfigIncludes(path, content, opts)
}

// dedupeRestrictedDirectives drops repeats of the same directive in the same
// file, which a configuration using one statement several times produces,
// while keeping the order they were found in.
func dedupeRestrictedDirectives(found []RestrictedDirective) []RestrictedDirective {
	if len(found) == 0 {
		return nil
	}

	seen := make(map[RestrictedDirective]struct{}, len(found))
	unique := make([]RestrictedDirective, 0, len(found))
	for _, directive := range found {
		if _, ok := seen[directive]; ok {
			continue
		}
		seen[directive] = struct{}{}
		unique = append(unique, directive)
	}

	return unique
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
	return validateConfigContent(content, validateOptions{})
}

func validateConfigContent(content []byte, opts validateOptions) error {
	if err := ValidateDataContent(content); err != nil {
		return err
	}

	if err := validateConfigDirectives(content, opts); err != nil {
		return err
	}

	return nil
}

func ValidateConfigDirectives(content []byte) error {
	return validateConfigDirectives(content, validateOptions{})
}

func validateConfigDirectives(content []byte, opts validateOptions) error {
	return visitConfigStatements(content, func(statement []string) error {
		return validateConfigStatement(statement, opts)
	})
}

func visitConfigStatements(content []byte, visit func([]string) error) error {
	return visitStatements(&nginxConfigLexer{content: content, tokenStart: true}, visit)
}

// visitConfigStatementsAsNginx is visitConfigStatements with every word
// holding the value nginx reads for it, backslashes kept where nginx keeps
// them. The denylist uses the stricter visitConfigStatements; this is for
// arguments that must match exactly, such as include paths.
func visitConfigStatementsAsNginx(content []byte, visit func([]string) error) error {
	return visitStatements(&nginxConfigLexer{content: content, tokenStart: true, nginxEscapes: true}, visit)
}

func visitStatements(lexer *nginxConfigLexer, visit func([]string) error) error {
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

func validateConfigStatement(statement []string, opts validateOptions) error {
	if len(statement) == 0 {
		return nil
	}

	directive := strings.ToLower(statement[0])
	if directive == "" {
		return nil
	}

	// "user root" is outside the step-up: it decides which system account the
	// worker processes run as, which is not what an administrator confirms
	// when they accept loading their own njs or Perl code.
	if directive == "user" && configuresRootWorkers(statement) {
		return cosy.WrapErrorWithParams(ErrConfigDirectiveNotAllowed, "user root")
	}

	if directive == "load_module" && loadsRestrictedDynamicModule(statement) {
		return restrictedDirectiveFound(directive, opts)
	}

	if isBlockedConfigDirective(directive) {
		return restrictedDirectiveFound(directive, opts)
	}

	return nil
}

// restrictedDirectiveFound records a restricted statement and reports whether
// it stops the validation. The file it was found in is filled in by the
// include validator, the only place that knows it.
func restrictedDirectiveFound(directive string, opts validateOptions) error {
	if opts.found != nil {
		*opts.found = append(*opts.found, RestrictedDirective{Directive: directive})
	}

	if opts.allowRestricted {
		return nil
	}

	return &RestrictedDirectiveError{
		Found: RestrictedDirective{Directive: directive},
		err:   cosy.WrapErrorWithParams(ErrConfigDirectiveNotAllowed, directive),
	}
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
			lexer.writeEscape(&builder)
			continue
		}

		builder.WriteRune(r)
	}

	return nginxConfigToken{kind: nginxConfigTokenWord, value: builder.String()}
}

// writeEscape writes the character after a backslash, which the lexer has
// consumed. By default the backslash is dropped. With nginxEscapes it follows
// the copy loop of ngx_conf_read_token: a backslash before a quote or a
// backslash is dropped, \t, \r and \n become the control characters, and any
// other backslash stays in the word.
func (lexer *nginxConfigLexer) writeEscape(builder *strings.Builder) {
	escaped, escapedSize := utf8.DecodeRune(lexer.content[lexer.offset:])
	lexer.offset += escapedSize
	if lexer.nginxEscapes {
		switch escaped {
		case '"', '\'', '\\':
		case 't':
			escaped = '\t'
		case 'r':
			escaped = '\r'
		case 'n':
			escaped = '\n'
		default:
			builder.WriteByte('\\')
		}
	}
	builder.WriteRune(escaped)
}

// readUnquotedToken reads a token up to the characters that end one in
// ngx_conf_read_token: whitespace, ';' and '{'. '#', '}' and quotes are
// ordinary characters inside a token, and a backslash keeps the next
// character in it. A '{' right after '$' also stays in the token: nginx sets
// its variable flag on '$' and skips '{' while the flag is set, which keeps
// "${name}" intact.
//
// By default an escape drops the backslash before every character, while
// nginx keeps it unless a quote, a backslash, t, r or n follows. That only
// makes the words the denylist compares stricter; token boundaries are the
// same. Include paths use nginxEscapes to get nginx's exact value.
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
			lexer.writeEscape(&builder)
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
// their existing configs uneditable. lua_package_path and lua_package_cpath
// stay allowed for the same reason: Lua add-ons such as the CrowdSec bouncer
// set them, and code that already runs through *_by_lua* can change
// package.path or load native code itself, so blocking them adds nothing.
// The "allow existing lua block directives" and "allow lua package paths"
// cases in validation_test.go codify this.
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
