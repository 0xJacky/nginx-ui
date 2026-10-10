package plugin

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/plugin/jsonrpc"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
)

const (
	// snippetDirName is the directory below the nginx configuration
	// directory that holds the snippets, and pluginSnippetDirName the one
	// below it with one directory of snippets per plugin. The snippets page of
	// Nginx UI leaves that directory to the plugins and never replicates it.
	snippetDirName       = "snippets"
	pluginSnippetDirName = "plugins"
	// snippetSuffix ends the file name of every snippet.
	snippetSuffix = ".conf"
	// maxSnippetBytes bounds one snippet.
	maxSnippetBytes = 256 << 10
	// maxSnippetsPerPlugin bounds the snippets of one plugin.
	maxSnippetsPerPlugin = 32
	// maxConfigFileBytes bounds what host.nginx.config.get returns.
	maxConfigFileBytes = 1 << 20
	// maxConfigFiles and maxConfigDepth bound the walk of host.nginx.config.list.
	maxConfigFiles = 5000
	maxConfigDepth = 8
)

var snippetNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// configWriter writes a generated nginx file, tests the configuration and
// reloads nginx, putting the old file back on failure.
type configWriter interface {
	Apply(path string, content []byte) (changed bool, err error)
	Remove(path string) (removed bool, err error)
}

func (b *hostBackend) configWriter() configWriter {
	if b.writer != nil {
		return b.writer
	}
	return config.GeneratedWriter{}
}

func (b *hostBackend) nginxConfDir() string {
	if b.confDir != nil {
		return b.confDir()
	}
	return nginx.GetConfPath()
}

// snippetDir is the directory of the snippets of one plugin. Plugin ids are
// validated reversed domains, so the id is a safe path element.
func (b *hostBackend) snippetDir(pluginID string) string {
	return filepath.Join(b.nginxConfDir(), snippetDirName, pluginSnippetDirName, pluginID)
}

// SnippetInclude is the directive that includes one snippet. nginx resolves
// a relative include against the directory of nginx.conf.
func SnippetInclude(pluginID, name string) string {
	return "include " + snippetDirName + "/" + pluginSnippetDirName + "/" + pluginID + "/" + name + snippetSuffix + ";"
}

func validateSnippetName(name string) error {
	if !snippetNamePattern.MatchString(name) {
		return jsonrpc.Errorf(protocol.CodeInvalidParams,
			"name must be 1 to 64 characters of a-z, 0-9, underscore and dash, starting with a letter or digit")
	}
	return nil
}

// snippetError turns a rejected configuration into an error the plugin can
// act on. nginx was not changed in that case.
func snippetError(err error, what string) error {
	var testErr *config.TestError
	if errors.As(err, &testErr) {
		return jsonrpc.Errorf(protocol.CodeInvalidParams, fmt.Sprintf("nginx rejected the configuration %s, nothing was changed: %v", what, testErr.Err))
	}
	return err
}

// NginxSnippetPut writes one snippet of a plugin, then tests and reloads
// nginx.
func (b *hostBackend) NginxSnippetPut(pluginID, name, content string) (bool, error) {
	if err := validateSnippetName(name); err != nil {
		return false, err
	}
	if len(content) > maxSnippetBytes {
		return false, jsonrpc.Errorf(protocol.CodeInvalidParams, fmt.Sprintf("content exceeds %d bytes", maxSnippetBytes))
	}
	if !utf8.ValidString(content) {
		return false, jsonrpc.Errorf(protocol.CodeInvalidParams, "content must be UTF-8")
	}

	names, err := b.snippetNames(pluginID)
	if err != nil {
		return false, err
	}
	if !slices.Contains(names, name) && len(names) >= maxSnippetsPerPlugin {
		return false, jsonrpc.Errorf(protocol.CodeInvalidParams, fmt.Sprintf("a plugin may keep at most %d snippets", maxSnippetsPerPlugin))
	}

	changed, err := b.configWriter().Apply(filepath.Join(b.snippetDir(pluginID), name+snippetSuffix), []byte(content))
	return changed, snippetError(err, "with the snippet")
}

// NginxSnippetDelete removes one snippet of a plugin, then tests and reloads
// nginx. A snippet still included somewhere fails the test and stays.
func (b *hostBackend) NginxSnippetDelete(pluginID, name string) (bool, error) {
	if err := validateSnippetName(name); err != nil {
		return false, err
	}
	removed, err := b.configWriter().Remove(filepath.Join(b.snippetDir(pluginID), name+snippetSuffix))
	return removed, snippetError(err, "without the snippet, which is probably still included")
}

// NginxSnippetList lists the snippets of a plugin, sorted by name.
func (b *hostBackend) NginxSnippetList(pluginID string) ([]protocol.HostNginxSnippet, error) {
	names, err := b.snippetNames(pluginID)
	if err != nil {
		return nil, err
	}
	snippets := make([]protocol.HostNginxSnippet, 0, len(names))
	for _, name := range names {
		snippets = append(snippets, protocol.HostNginxSnippet{Name: name, Include: SnippetInclude(pluginID, name)})
	}
	return snippets, nil
}

func (b *hostBackend) snippetNames(pluginID string) ([]string, error) {
	entries, err := nginx.ReadDir(b.snippetDir(pluginID))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		name, ok := strings.CutSuffix(entry.Name(), snippetSuffix)
		if ok && entry.Type().IsRegular() && snippetNamePattern.MatchString(name) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names, nil
}

// removeSnippets takes the snippets of an uninstalled plugin out of nginx.
// A snippet nginx still includes cannot go without breaking the
// configuration, so it is emptied instead.
func (b *hostBackend) removeSnippets(pluginID string) error {
	names, err := b.snippetNames(pluginID)
	if err != nil {
		return err
	}
	writer := b.configWriter()
	var errs []error
	for _, name := range names {
		file := filepath.Join(b.snippetDir(pluginID), name+snippetSuffix)
		_, err := writer.Remove(file)
		var testErr *config.TestError
		if errors.As(err, &testErr) {
			_, err = writer.Apply(file, []byte("# The plugin "+pluginID+" was uninstalled.\n"))
		}
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		// Only an empty directory goes, a kept snippet stays where it is.
		_ = nginx.Remove(b.snippetDir(pluginID))
	}
	return errors.Join(errs...)
}

// isReadableConfig reports whether a path relative to the configuration
// directory is a configuration file a plugin may read: a .conf file, a
// site or stream file, or nginx.conf. Keys, password files and anything
// else stay out of reach.
func isReadableConfig(rel string) bool {
	if rel == "nginx.conf" || strings.HasSuffix(rel, ".conf") {
		return true
	}
	dir, _ := path.Split(rel)
	return dir == "sites-available/" || dir == "streams-available/"
}

// validConfigPath reports whether rel is a clean relative path that stays
// inside the configuration directory.
func validConfigPath(rel string) bool {
	return rel != "" && !strings.ContainsAny(rel, "\\\x00") && !path.IsAbs(rel) &&
		path.Clean(rel) == rel && rel != ".." && !strings.HasPrefix(rel, "../")
}

// NginxConfigList lists the configuration files a plugin may read, relative
// to the configuration directory. Symbolic links are not followed, so the
// walk stays inside the directory.
func (b *hostBackend) NginxConfigList() ([]string, error) {
	root := b.nginxConfDir()
	var files []string
	var walk func(rel string, depth int) error
	walk = func(rel string, depth int) error {
		entries, err := nginx.ReadDir(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if len(files) >= maxConfigFiles {
				return nil
			}
			child := path.Join(rel, entry.Name())
			switch {
			case entry.IsDir() && depth < maxConfigDepth:
				if err := walk(child, depth+1); err != nil {
					return err
				}
			case entry.Type().IsRegular() && isReadableConfig(child):
				files = append(files, child)
			}
		}
		return nil
	}
	if err := walk("", 0); err != nil {
		return nil, err
	}
	slices.Sort(files)
	return files, nil
}

// NginxConfigGet reads one configuration file host.nginx.config.list returns.
func (b *hostBackend) NginxConfigGet(rel string) (string, error) {
	if !validConfigPath(rel) || !isReadableConfig(rel) {
		return "", jsonrpc.Errorf(protocol.CodeInvalidParams, "path must be a configuration file relative to the nginx configuration directory")
	}
	root := b.nginxConfDir()
	file := filepath.Join(root, filepath.FromSlash(rel))

	info, err := nginx.Lstat(file)
	if errors.Is(err, fs.ErrNotExist) {
		return "", jsonrpc.Errorf(protocol.CodeInvalidParams, "no such configuration file")
	}
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", jsonrpc.Errorf(protocol.CodeInvalidParams, "path is not a regular file")
	}
	// A linked directory on the way could lead outside.
	resolvedRoot, err := nginx.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	resolved, err := nginx.EvalSymlinks(file)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(resolved, resolvedRoot+string(filepath.Separator)) {
		return "", jsonrpc.Errorf(protocol.CodeInvalidParams, "path leaves the nginx configuration directory")
	}
	if info.Size() > maxConfigFileBytes {
		return "", jsonrpc.Errorf(protocol.CodeInvalidParams, fmt.Sprintf("file exceeds %d bytes", maxConfigFileBytes))
	}
	content, err := nginx.ReadFile(resolved)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// SitesList lists the sites, sorted by name.
func (b *hostBackend) SitesList() ([]protocol.HostSite, error) {
	dir := filepath.Join(b.nginxConfDir(), "sites-available")
	entries, err := nginx.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []protocol.HostSite{}, nil
	}
	if err != nil {
		return nil, err
	}
	sites := make([]protocol.HostSite, 0, len(entries))
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		name := entry.Name()
		urls := site.GetIndexedSite(filepath.Join(dir, name)).Urls
		if urls == nil {
			urls = []string{}
		}
		sites = append(sites, protocol.HostSite{
			Name:       name,
			Status:     string(site.GetSiteStatus(name)),
			URLs:       urls,
			ConfigFile: "sites-available/" + name,
		})
	}
	slices.SortFunc(sites, func(a, b protocol.HostSite) int { return strings.Compare(a.Name, b.Name) })
	return sites, nil
}

// CertsList lists the certificates without their private keys, sorted by
// name.
func (b *hostBackend) CertsList() ([]protocol.HostCert, error) {
	rows, err := query.Cert.Find()
	if err != nil {
		return nil, err
	}
	certs := make([]protocol.HostCert, 0, len(rows))
	for _, row := range rows {
		domains := row.Domains
		if domains == nil {
			domains = []string{}
		}
		entry := protocol.HostCert{
			ID:              strconv.FormatUint(row.ID, 10),
			Name:            row.Name,
			Domains:         domains,
			AutoRenew:       row.AutoCert == model.AutoCertEnabled,
			ChallengeMethod: row.ChallengeMethod,
			KeyType:         string(row.KeyType),
		}
		if row.SSLCertificatePath != "" {
			if info, err := cert.GetCertInfo(row.SSLCertificatePath); err == nil {
				entry.NotBefore = info.NotBefore.UTC().Format(time.RFC3339)
				entry.NotAfter = info.NotAfter.UTC().Format(time.RFC3339)
				entry.Issuer = info.IssuerOrganization
				if entry.Issuer == "" {
					entry.Issuer = info.IssuerName
				}
			}
		}
		certs = append(certs, entry)
	}
	slices.SortFunc(certs, func(a, b protocol.HostCert) int { return strings.Compare(a.Name, b.Name) })
	return certs, nil
}
