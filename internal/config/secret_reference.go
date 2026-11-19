package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
)

// secretFileDirectives name the files Nginx reads secrets from: session ticket
// keys, QUIC host keys, ECH keys, basic auth users and JSON Web Key Sets. A
// file one of them references is a secret whatever its name or format. The
// value tells whether Nginx evaluates variables in the argument.
var secretFileDirectives = map[string]bool{
	"ssl_session_ticket_key": false,
	"quic_host_key":          false,
	"ssl_ech_file":           false,
	"auth_basic_user_file":   true,
	"auth_jwt_key_file":      true,
}

// Every module that opens TLS connections has its own copy of these
// directives (proxy_ssl_*, grpc_ssl_*, uwsgi_ssl_*, zone_sync_ssl_*, ...), so
// they are matched by suffix. The key log holds session secrets Nginx writes.
const (
	certificateKeyDirectiveSuffix = "ssl_certificate_key"
	passwordFileDirectiveSuffix   = "ssl_password_file"
	keyLogDirectiveSuffix         = "ssl_key_log"
)

// keySourceSchemes are the certificate key values Nginx does not open as a
// file. Any other colon is part of a file name.
var keySourceSchemes = []string{"data:", "engine:"}

// storeKeySource marks a certificate key that Nginx hands to the OpenSSL store
// loader, which opens local files as well.
const storeKeySource = "store:"

const (
	// maxSecretReferenceScanSize skips files under the configuration root that
	// no include reaches and that cannot be ordinary configuration, such as a
	// GeoIP database stored next to it.
	maxSecretReferenceScanSize = 1 << 20

	// maxSecretReferenceScanBytes bounds what one scan reads through includes.
	// Files that Nginx loads through an include are read whatever their size,
	// since a large map or geo file is configuration too; this is the size the
	// include validation accepts for a whole configuration. The walk over the
	// configuration root is not charged to it: each file the walk reads is
	// small, and ordinary files under the root must not use up the budget
	// before the files only the walk reaches, such as disabled sites.
	maxSecretReferenceScanBytes = maxConfigIncludeBytes

	// maxCachedConfigSummaries bounds the summaries kept for large files.
	maxCachedConfigSummaries = 64
)

// configSummary holds what the reference scan needs from one file: the
// arguments of its secret directives and of its includes, as Nginx reads them.
type configSummary struct {
	secrets  []string
	includes []string
}

type cachedConfigSummary struct {
	size    int64
	modTime time.Time
	summary configSummary
}

// largeConfigSummaries remembers the summary of files larger than
// maxSecretReferenceScanSize until their size or modification time changes, so
// a multi-megabyte include is tokenized once, not on every read request.
// Smaller files are cheap to read and are never cached.
var largeConfigSummaries = struct {
	sync.Mutex
	entries map[string]cachedConfigSummary
}{entries: map[string]cachedConfigSummary{}}

// SecretReferences maps the absolute paths that the Nginx configuration names
// as secret files to the file each one opened when the configuration was
// scanned, or nil where it opened none.
type SecretReferences map[string]os.FileInfo

// SecretFileReferences collects the absolute paths that the Nginx
// configuration names as secret files. The configuration is read the way Nginx
// reads it: tokenized with its quoting, escaping and comment rules, starting
// at the main configuration file and following every include wherever it
// leads. Files under the configuration root that no include reaches, such as
// disabled sites, are scanned as well.
//
// Relative paths resolve against the configuration prefix, as Nginx resolves
// them. Paths built from variables and non-file key sources such as data: and
// engine: are skipped.
func SecretFileReferences() SecretReferences {
	confRoot := nginx.GetConfPath()
	references := SecretReferences{}
	if confRoot == "" {
		return references
	}

	scan := secretReferenceScan{
		bases:      []string{includeBasePath()},
		references: references,
		scanned:    map[string]struct{}{},
		included:   map[string]struct{}{},
		resolved:   map[string]string{},
		budget:     maxSecretReferenceScanBytes,
	}
	if scan.bases[0] != confRoot {
		scan.bases = append(scan.bases, confRoot)
	}
	// Include patterns resolve on the file system Nginx runs on. They are
	// followed when that is the local one, which is what this scan reads.
	if remote, err := nginx.UsesSFTPTarget(); err == nil && !remote {
		scan.followIncludes = true
	}

	scan.file(mainConfigPath(confRoot), 0)

	// WalkDir does not descend into a root that is itself a symlink.
	walkRoot := confRoot
	if resolved, err := filepath.EvalSymlinks(confRoot); err == nil {
		walkRoot = resolved
	}
	_ = filepath.WalkDir(walkRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		// Certificates and other data files are not parsed as directives
		// unless an include makes Nginx read them as such.
		if _, isData := dataFileExtensions[strings.ToLower(filepath.Ext(path))]; isData {
			return nil
		}
		scan.file(path, maxSecretReferenceScanSize)
		return nil
	})
	return references
}

// mainConfigPath returns the file Nginx starts reading at. With a configured
// directory and no configured file it is that directory's nginx.conf, the
// prefix includeBasePath uses too, rather than a path asked from the binary.
func mainConfigPath(confRoot string) string {
	if settings.NginxSettings.ConfigPath == "" && settings.NginxSettings.ConfigDir != "" {
		return filepath.Join(confRoot, "nginx.conf")
	}
	return nginx.GetConfEntryPath()
}

type secretReferenceScan struct {
	// bases are the directories a relative secret path may resolve against.
	// The first one is the configuration prefix, which includes resolve
	// against as well.
	bases          []string
	followIncludes bool
	references     SecretReferences
	// scanned holds the symlink-resolved path of every file already read, so
	// a file reached both by an include and by the directory walk, or through
	// an include cycle, is read once.
	scanned map[string]struct{}
	// included holds every include argument already expanded. Sites tend to
	// include the same few snippets.
	included map[string]struct{}
	// resolved holds the symlink-resolved path of every directory seen.
	resolved map[string]string
	budget   int64
}

// file scans one file and everything it includes. sizeLimit applies to the
// file itself when it is positive, and such a file is not charged to the
// budget; included files have no limit of their own and are charged.
func (scan *secretReferenceScan) file(path string, sizeLimit int64) {
	pending := []string{path}
	for len(pending) > 0 {
		last := len(pending) - 1
		path, pending = pending[last], pending[:last]

		summary, ok := scan.summarize(path, sizeLimit)
		sizeLimit = 0
		if !ok {
			continue
		}
		for _, secret := range summary.secrets {
			for _, base := range scan.bases {
				scan.reference(includeFullPath(base, secret))
			}
		}
		if !scan.followIncludes {
			continue
		}
		for _, include := range summary.includes {
			if _, done := scan.included[include]; done {
				continue
			}
			scan.included[include] = struct{}{}
			matches, err := resolveIncludePattern(includeFullPath(scan.bases[0], include))
			if err != nil {
				continue
			}
			pending = append(pending, matches...)
		}
	}
}

func (scan *secretReferenceScan) summarize(path string, sizeLimit int64) (configSummary, bool) {
	// Stat follows symlinks, as Nginx does when it opens an enabled site.
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return configSummary{}, false
	}
	if sizeLimit > 0 && info.Size() > sizeLimit {
		return configSummary{}, false
	}

	canonical, err := scan.evalSymlinks(path)
	if err != nil {
		canonical = filepath.Clean(path)
	}
	if _, done := scan.scanned[canonical]; done {
		return configSummary{}, false
	}

	isLarge := info.Size() > maxSecretReferenceScanSize
	if isLarge {
		if summary, ok := cachedConfigSummaryFor(canonical, info); ok {
			scan.scanned[canonical] = struct{}{}
			return summary, true
		}
	}

	if sizeLimit == 0 {
		// A file the budget no longer covers is left unscanned, not marked
		// as scanned, so the walk over the configuration root still reads it
		// when it is small enough.
		if info.Size() > scan.budget {
			return configSummary{}, false
		}
		scan.budget -= info.Size()
	}
	scan.scanned[canonical] = struct{}{}
	content, err := os.ReadFile(path)
	if err != nil {
		return configSummary{}, false
	}

	summary := summarizeConfig(content)
	if isLarge {
		cacheConfigSummary(canonical, info, summary)
	}
	return summary, true
}

func cachedConfigSummaryFor(path string, info os.FileInfo) (configSummary, bool) {
	largeConfigSummaries.Lock()
	defer largeConfigSummaries.Unlock()

	cached, ok := largeConfigSummaries.entries[path]
	if !ok || cached.size != info.Size() || !cached.modTime.Equal(info.ModTime()) {
		return configSummary{}, false
	}
	return cached.summary, true
}

func cacheConfigSummary(path string, info os.FileInfo, summary configSummary) {
	largeConfigSummaries.Lock()
	defer largeConfigSummaries.Unlock()

	if _, ok := largeConfigSummaries.entries[path]; !ok && len(largeConfigSummaries.entries) >= maxCachedConfigSummaries {
		clear(largeConfigSummaries.entries)
	}
	largeConfigSummaries.entries[path] = cachedConfigSummary{size: info.Size(), modTime: info.ModTime(), summary: summary}
}

// summarizeConfig tokenizes content the way ngx_conf_read_token does and
// returns the arguments of its secret directives and includes.
func summarizeConfig(content []byte) configSummary {
	// Nginx reads its configuration byte by byte, while the lexer decodes
	// runes and would rewrite a byte that is not valid UTF-8, such as a file
	// name in a legacy encoding. Widening every byte to the rune of the same
	// value keeps the ASCII syntax in place, and narrowing the words back
	// returns the exact bytes Nginx sees.
	word := func(value string) string { return value }
	if !utf8.Valid(content) {
		content, word = widenBytes(content), narrowRunes
	}

	var summary configSummary
	_ = visitConfigStatementsAsNginx(content, func(statement []string) error {
		if len(statement) < 2 {
			return nil
		}
		argument := word(statement[1])
		if strings.EqualFold(statement[0], "include") {
			summary.includes = append(summary.includes, argument)
			return nil
		}
		summary.secrets = append(summary.secrets, secretFilePaths(statement[0], argument)...)
		return nil
	})
	return summary
}

// secretFilePaths returns the files that the first argument of a directive
// makes Nginx read a secret from, or nil if it names none.
func secretFilePaths(directive, argument string) []string {
	if argument == "" {
		return nil
	}

	paths := []string{argument}
	hasVariables, known := secretFileDirectives[directive]
	switch {
	case strings.HasSuffix(directive, certificateKeyDirectiveSuffix):
		for _, scheme := range keySourceSchemes {
			if strings.HasPrefix(argument, scheme) {
				return nil
			}
		}
		if uri, ok := strings.CutPrefix(argument, storeKeySource); ok {
			paths = storeURIPaths(uri)
		}
		hasVariables = true
	case strings.HasSuffix(directive, passwordFileDirectiveSuffix),
		strings.HasSuffix(directive, keyLogDirectiveSuffix):
	case !known:
		return nil
	}

	// Where Nginx evaluates variables the file is only known per request.
	// Everywhere else '$' is an ordinary character of the file name.
	if hasVariables && strings.Contains(argument, "$") {
		return nil
	}
	return paths
}

// storeURIPaths returns the files the OpenSSL store loader may open for a
// store URI. Its file loader is tried first for every URI and takes the URI
// as a path, and a file: URI names an absolute path as well. Recording a name
// that turns out to be no file costs nothing, so other schemes are not told
// apart.
func storeURIPaths(uri string) []string {
	const fileScheme = "file:"
	if len(uri) < len(fileScheme) || !strings.EqualFold(uri[:len(fileScheme)], fileScheme) {
		return []string{uri}
	}
	path := uri[len(fileScheme):]
	authority, ok := strings.CutPrefix(path, "//")
	if !ok {
		return []string{uri, path}
	}
	// With an authority the URI itself is not tried as a path.
	if len(authority) >= len("localhost/") && strings.EqualFold(authority[:len("localhost/")], "localhost/") {
		authority = authority[len("localhost"):]
	}
	return []string{authority}
}

func widenBytes(content []byte) []byte {
	widened := make([]byte, 0, len(content)+len(content)/2)
	for _, c := range content {
		widened = utf8.AppendRune(widened, rune(c))
	}
	return widened
}

func narrowRunes(value string) string {
	narrowed := make([]byte, 0, len(value))
	for _, r := range value {
		narrowed = append(narrowed, byte(r))
	}
	return string(narrowed)
}

// reference records a path the way Nginx opens it, together with the file it
// opens. The path is not cleaned before symlinks are resolved: cleaning folds
// "dir/../file" lexically, while the kernel follows ".." from wherever "dir"
// really points.
func (scan *secretReferenceScan) reference(path string) {
	cleaned := filepath.Clean(path)
	if _, done := scan.references[cleaned]; done {
		return
	}
	// Stat follows symlinks and walks ".." the way the kernel does, so this
	// is the file Nginx opens. A failed stat leaves only the name to match.
	var info os.FileInfo
	if opened, err := os.Stat(path); err == nil {
		info = opened
	}
	scan.references[cleaned] = info
	if resolved, err := scan.evalSymlinks(path); err == nil {
		scan.references[resolved] = info
	}
}

// evalSymlinks is filepath.EvalSymlinks with the directory part remembered.
// A configuration keeps hundreds of files in a handful of directories, so
// resolving every component of every path again would dominate the scan.
func (scan *secretReferenceScan) evalSymlinks(path string) (string, error) {
	if filepath.Clean(path) != path {
		return filepath.EvalSymlinks(path)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return filepath.EvalSymlinks(path)
	}

	dir := filepath.Dir(path)
	resolved, ok := scan.resolved[dir]
	if !ok {
		if resolved, err = filepath.EvalSymlinks(dir); err != nil {
			return "", err
		}
		scan.resolved[dir] = resolved
	}
	return filepath.Join(resolved, filepath.Base(path)), nil
}
