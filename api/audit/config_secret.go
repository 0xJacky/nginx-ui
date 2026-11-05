package audit

import (
	"path/filepath"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
)

// configFileRoutes read or write the content of one configuration file, or
// its history, in the request or response body. The MCP endpoints carry the
// nginx_config_* tools, whose arguments name the file the same way.
var configFileRoutes = map[string]struct{}{
	"GET /api/config":           {},
	"POST /api/config":          {},
	"POST /api/configs":         {},
	"GET /api/config_histories": {},
	"POST /mcp":                 {},
	"POST /mcp_message":         {},
}

// mcpConfigToolPrefix names the MCP configuration tools (mcp/config).
const mcpConfigToolPrefix = "nginx_config_"

// configPathQueryKeys and configPathFields are where these routes name the
// target file: the query for reads, the JSON body for writes and MCP tool
// arguments, and the response for history entries.
var (
	configPathQueryKeys = []string{"path", "filepath"}
	configPathFields    = map[string]struct{}{
		"path":          {},
		"filepath":      {},
		"file_path":     {},
		"relative_path": {},
		"name":          {},
	}
)

// targetsSecretConfigFile reports whether a configuration request reads or
// writes a secret file: a private key, a password file, or a file a secret
// directive references. The decision uses only the request and the bodies, so
// it also holds for requests proxied to a remote node, where no local handler
// runs. Directive references are only known for local files; a proxied request
// relies on the file extension and on content inspection instead.
func targetsSecretConfigFile(snapshot *requestSnapshot, reqBody, respBody string) bool {
	if _, ok := configFileRoutes[snapshot.method+" "+snapshot.fullPath]; !ok {
		return false
	}
	// Other MCP tools cannot return a configuration file's content, and their
	// "name" argument is the tool name, so skip the file lookup for them.
	if (snapshot.fullPath == "/mcp" || snapshot.fullPath == "/mcp_message") &&
		!strings.Contains(reqBody, `"`+mcpConfigToolPrefix) {
		return false
	}

	var candidates []string
	query := snapshot.url.Query()
	for _, key := range configPathQueryKeys {
		if value := query.Get(key); value != "" {
			decoded, _ := helper.DecodePathParam(value)
			candidates = append(candidates, decoded)
		}
	}
	for _, body := range []string{reqBody, respBody} {
		fields := jsonStringValues(body)
		for name, values := range fields {
			if _, ok := configPathFields[name]; ok {
				candidates = append(candidates, values...)
			}
		}
		// AddConfig names the file by base_dir and name.
		for _, baseDir := range fields["base_dir"] {
			for _, name := range fields["name"] {
				candidates = append(candidates, filepath.Join(baseDir, name))
			}
		}
	}

	var references map[string]struct{}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if config.IsSecretFileContent(candidate, nil) {
			return true
		}
		if snapshot.proxied {
			continue
		}
		absPath, err := config.ResolveAbsoluteOrRelativeConfPath(candidate)
		if err != nil {
			continue
		}
		if references == nil {
			references = config.SecretFileReferences()
		}
		if config.IsSecretFile(absPath, nil, references) {
			return true
		}
	}
	return false
}

// containsPrivateKey reports whether a body carries PEM private key material,
// whatever route it came through.
func containsPrivateKey(body string) bool {
	return body != "" && config.IsSecretFileContent("", []byte(body))
}

// jsonStringValues collects the string values of JSON object members by
// member name. Values are returned as they appear in the JSON text; config
// paths rarely need escapes, and an escaped path only misses the reference
// lookup, not the extension and content checks.
func jsonStringValues(body string) map[string][]string {
	values := map[string][]string{}
	for _, match := range jsonStringField.FindAllStringSubmatch(body, -1) {
		member := match[0]
		prefix := `"` + match[1] + `"` + match[2] + `"`
		value := member[len(prefix):]
		if match[3] == `"` && len(value) > 0 {
			value = value[:len(value)-1]
		}
		values[match[1]] = append(values[match[1]], value)
	}
	return values
}
