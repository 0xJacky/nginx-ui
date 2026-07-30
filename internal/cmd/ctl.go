package cmd

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/urfave/cli/v3"
)

const maxCLIInputSize = 16 << 20

var CtlCommand = &cli.Command{
	Name:  "ctl",
	Usage: "Manage a running Nginx UI instance through its API",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "endpoint", Usage: "Nginx UI base URL (or NGINX_UI_CTL_ENDPOINT)"},
		&cli.StringFlag{Name: "token-file", Usage: "read the API or user token from a file"},
		&cli.BoolFlag{Name: "token-stdin", Usage: "read the API or user token from standard input"},
		&cli.StringFlag{Name: "ca-file", Usage: "PEM CA bundle for the server certificate"},
		&cli.StringFlag{Name: "node-id", Usage: "target a cluster node by ID"},
		&cli.DurationFlag{Name: "timeout", Usage: "request timeout", Value: 30 * time.Second},
	},
	Commands: []*cli.Command{
		ctlAPICommand(),
		ctlUsersCommand(),
		ctlCertificatesCommand(),
		ctlNginxCommand(),
		ctlTokensCommand(),
	},
}

type ctlClient struct {
	baseURL *url.URL
	token   string
	nodeID  string
	client  *http.Client
	stdout  io.Writer
}

type ctlHTTPError struct {
	StatusCode int
	Body       string
}

func (e *ctlHTTPError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("Nginx UI API returned HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("Nginx UI API returned HTTP %d: %s", e.StatusCode, e.Body)
}

func newCtlClient(command *cli.Command) (*ctlClient, error) {
	endpoint := strings.TrimSpace(command.String("endpoint"))
	if endpoint == "" {
		endpoint = strings.TrimSpace(os.Getenv("NGINX_UI_CTL_ENDPOINT"))
	}
	if endpoint == "" {
		return nil, errors.New("endpoint is required; use --endpoint or NGINX_UI_CTL_ENDPOINT")
	}
	baseURL, err := url.Parse(endpoint)
	if err != nil || baseURL.Host == "" || (baseURL.Scheme != "http" && baseURL.Scheme != "https") {
		return nil, fmt.Errorf("invalid endpoint %q", endpoint)
	}
	if baseURL.User != nil {
		return nil, errors.New("endpoint must not contain user information")
	}
	baseURL.RawQuery = ""
	baseURL.Fragment = ""

	token, err := ctlToken(command)
	if err != nil {
		return nil, err
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if transport.TLSClientConfig != nil {
		tlsConfig = transport.TLSClientConfig.Clone()
		tlsConfig.MinVersion = tls.VersionTLS12
	}
	transport.TLSClientConfig = tlsConfig
	if caFile := strings.TrimSpace(command.String("ca-file")); caFile != "" {
		caPEM, readErr := readLimitedFile(caFile)
		if readErr != nil {
			return nil, fmt.Errorf("read CA file: %w", readErr)
		}
		roots, rootErr := x509.SystemCertPool()
		if rootErr != nil || roots == nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(caPEM) {
			return nil, errors.New("CA file does not contain a valid certificate")
		}
		transport.TLSClientConfig.RootCAs = roots
	}

	nodeID := strings.TrimSpace(command.String("node-id"))
	if _, err = parseNodeID(nodeID); err != nil {
		return nil, fmt.Errorf("invalid node ID: %w", err)
	}
	return &ctlClient{
		baseURL: baseURL,
		token:   token,
		nodeID:  nodeID,
		client: &http.Client{
			Transport: transport,
			Timeout:   command.Duration("timeout"),
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		stdout: os.Stdout,
	}, nil
}

func ctlToken(command *cli.Command) (string, error) {
	if command.Bool("token-stdin") && command.String("token-file") != "" {
		return "", errors.New("use only one of --token-stdin and --token-file")
	}
	var raw []byte
	var err error
	switch {
	case command.Bool("token-stdin"):
		raw, err = io.ReadAll(io.LimitReader(os.Stdin, maxCLIInputSize+1))
	case command.String("token-file") != "":
		raw, err = readLimitedFile(command.String("token-file"))
	default:
		raw = []byte(os.Getenv("NGINX_UI_CTL_TOKEN"))
	}
	if err != nil {
		return "", fmt.Errorf("read token: %w", err)
	}
	if len(raw) > maxCLIInputSize {
		return "", errors.New("token input is too large")
	}
	token := strings.TrimSpace(string(raw))
	if token == "" {
		return "", errors.New("token is required; use NGINX_UI_CTL_TOKEN, --token-file, or --token-stdin")
	}
	return token, nil
}

func readLimitedFile(filename string) ([]byte, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxCLIInputSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxCLIInputSize {
		return nil, errors.New("input file is too large")
	}
	return data, nil
}

func (c *ctlClient) do(ctx context.Context, method, apiPath string, payload any) ([]byte, error) {
	requestURL, err := c.apiURL(apiPath)
	if err != nil {
		return nil, err
	}
	var body io.Reader
	if payload != nil {
		switch value := payload.(type) {
		case []byte:
			body = bytes.NewReader(value)
		default:
			encoded, encodeErr := json.Marshal(value)
			if encodeErr != nil {
				return nil, encodeErr
			}
			body = bytes.NewReader(encoded)
		}
	}
	request, err := http.NewRequestWithContext(ctx, method, requestURL.String(), body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Accept", "application/json")
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if c.nodeID != "" {
		request.Header.Set("X-Node-ID", c.nodeID)
	}
	response, err := c.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxCLIInputSize+1))
	if err != nil {
		return nil, err
	}
	if len(responseBody) > maxCLIInputSize {
		return nil, errors.New("API response is too large")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, &ctlHTTPError{StatusCode: response.StatusCode, Body: strings.TrimSpace(string(responseBody))}
	}
	return responseBody, nil
}

func (c *ctlClient) apiURL(apiPath string) (*url.URL, error) {
	relative, err := url.Parse(apiPath)
	if err != nil || relative.IsAbs() || relative.Host != "" || strings.HasPrefix(apiPath, "//") {
		return nil, errors.New("API path must be relative")
	}
	result := *c.baseURL
	cleanPath := path.Clean("/" + strings.TrimPrefix(relative.Path, "/"))
	if cleanPath == "/api" || strings.HasPrefix(cleanPath, "/api/") {
		result.Path = cleanPath
	} else {
		result.Path = path.Join(strings.TrimSuffix(result.Path, "/"), "api", cleanPath)
	}
	result.RawQuery = relative.RawQuery
	return &result, nil
}

func writeCLIResponse(writer io.Writer, body []byte) error {
	if len(bytes.TrimSpace(body)) == 0 {
		_, err := fmt.Fprintln(writer, "{}")
		return err
	}
	var formatted bytes.Buffer
	if json.Indent(&formatted, body, "", "  ") == nil {
		formatted.WriteByte('\n')
		_, err := writer.Write(formatted.Bytes())
		return err
	}
	_, err := fmt.Fprintln(writer, string(body))
	return err
}

func executeCtlRequest(ctx context.Context, command *cli.Command, method, apiPath string, payload any) error {
	client, err := newCtlClient(command)
	if err != nil {
		return err
	}
	body, err := client.do(ctx, method, apiPath, payload)
	if err != nil {
		return err
	}
	return writeCLIResponse(client.stdout, body)
}

func executeCtlRequestRedactingFields(
	ctx context.Context,
	command *cli.Command,
	method string,
	apiPath string,
	payload any,
	fields ...string,
) error {
	client, err := newCtlClient(command)
	if err != nil {
		return err
	}
	body, err := client.do(ctx, method, apiPath, payload)
	if err != nil {
		return err
	}
	redacted, err := redactJSONFields(body, fields...)
	if err != nil {
		return err
	}
	return writeCLIResponse(client.stdout, redacted)
}

func redactJSONFields(body []byte, fields ...string) ([]byte, error) {
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, errors.New("Nginx UI API returned an unexpected non-JSON response")
	}
	fieldSet := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		fieldSet[field] = struct{}{}
	}
	var redact func(any)
	redact = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			for key, child := range typed {
				if _, sensitive := fieldSet[key]; sensitive {
					delete(typed, key)
					continue
				}
				redact(child)
			}
		case []any:
			for _, child := range typed {
				redact(child)
			}
		}
	}
	redact(payload)
	return json.Marshal(payload)
}

func ctlAPICommand() *cli.Command {
	return &cli.Command{
		Name:      "api",
		Usage:     "Call any Nginx UI management API endpoint",
		UsageText: "nginx-ui ctl api [options] PATH",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "method", Value: http.MethodGet, Usage: "HTTP method"},
			&cli.StringFlag{Name: "data", Usage: "JSON request body"},
			&cli.StringFlag{Name: "data-file", Usage: "read the JSON request body from a file"},
		},
		Action: func(ctx context.Context, command *cli.Command) error {
			apiPath := command.Args().First()
			if apiPath == "" {
				return errors.New("API path is required")
			}
			if command.String("data") != "" && command.String("data-file") != "" {
				return errors.New("use only one of --data and --data-file")
			}
			var payload []byte
			if command.String("data") != "" {
				payload = []byte(command.String("data"))
			} else if command.String("data-file") != "" {
				var err error
				payload, err = readLimitedFile(command.String("data-file"))
				if err != nil {
					return err
				}
			}
			if len(payload) > 0 && !json.Valid(payload) {
				return errors.New("request body must be valid JSON")
			}
			var requestPayload any
			if len(payload) > 0 {
				requestPayload = payload
			}
			return executeCtlRequest(ctx, command, strings.ToUpper(command.String("method")), apiPath, requestPayload)
		},
	}
}

func ctlUsersCommand() *cli.Command {
	return &cli.Command{
		Name:  "users",
		Usage: "Manage users",
		Commands: []*cli.Command{
			{
				Name: "list", Usage: "List users",
				Action: func(ctx context.Context, command *cli.Command) error {
					return executeCtlRequest(ctx, command, http.MethodGet, "users", nil)
				},
			},
			{
				Name: "create", Usage: "Create a user",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "name", Required: true, Usage: "user name"},
					&cli.BoolFlag{Name: "password-stdin", Usage: "read the password from standard input"},
					&cli.StringFlag{Name: "password-file", Usage: "read the password from a file"},
				},
				Action: func(ctx context.Context, command *cli.Command) error {
					password, err := ctlPassword(command)
					if err != nil {
						return err
					}
					return executeCtlRequest(ctx, command, http.MethodPost, "users", map[string]any{
						"name": command.String("name"), "password": password, "status": true,
					})
				},
			},
		},
	}
}

func ctlPassword(command *cli.Command) (string, error) {
	if command.Bool("password-stdin") && command.String("password-file") != "" {
		return "", errors.New("use only one of --password-stdin and --password-file")
	}
	if !command.Bool("password-stdin") && command.String("password-file") == "" {
		return "", errors.New("use --password-stdin or --password-file")
	}
	if command.Bool("password-stdin") && command.Bool("token-stdin") {
		return "", errors.New("password and token cannot both be read from standard input")
	}
	var raw []byte
	var err error
	if command.Bool("password-stdin") {
		raw, err = io.ReadAll(io.LimitReader(os.Stdin, 4097))
	} else {
		raw, err = readLimitedFile(command.String("password-file"))
	}
	if err != nil {
		return "", err
	}
	return normalizeCtlPassword(raw)
}

func normalizeCtlPassword(raw []byte) (string, error) {
	password := strings.TrimRight(string(raw), "\r\n")
	if password == "" {
		return "", errors.New("password is empty")
	}
	if utf8.RuneCountInString(password) > 20 {
		return "", errors.New("password must not exceed 20 characters")
	}
	return password, nil
}

func ctlCertificatesCommand() *cli.Command {
	return &cli.Command{
		Name:    "certificates",
		Aliases: []string{"certs"},
		Usage:   "Manage certificates through the API",
		Commands: []*cli.Command{
			{Name: "list", Usage: "List certificates", Action: func(ctx context.Context, command *cli.Command) error {
				return executeCtlRequestRedactingFields(
					ctx,
					command,
					http.MethodGet,
					"certs",
					nil,
					"ssl_certificate",
					"ssl_certificate_key",
				)
			}},
			{
				Name: "import", Usage: "Register certificate files already present on the server",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "name", Usage: "certificate name"},
					&cli.StringFlag{Name: "cert", Required: true, Usage: "server-side certificate path"},
					&cli.StringFlag{Name: "key", Required: true, Usage: "server-side private key path"},
					&cli.StringFlag{Name: "key-type", Usage: "private key type override"},
				},
				Action: func(ctx context.Context, command *cli.Command) error {
					return executeCtlRequestRedactingFields(
						ctx,
						command,
						http.MethodPost,
						"cert_import",
						map[string]any{
							"name":                     command.String("name"),
							"ssl_certificate_path":     command.String("cert"),
							"ssl_certificate_key_path": command.String("key"),
							"key_type":                 command.String("key-type"),
						},
						"ssl_certificate",
						"ssl_certificate_key",
					)
				},
			},
		},
	}
}

func ctlNginxCommand() *cli.Command {
	commands := make([]*cli.Command, 0, 4)
	for _, operation := range []struct {
		name, method, apiPath string
	}{
		{"status", http.MethodGet, "nginx/status"},
		{"test", http.MethodPost, "nginx/test"},
		{"reload", http.MethodPost, "nginx/reload"},
		{"restart", http.MethodPost, "nginx/restart"},
	} {
		op := operation
		commands = append(commands, &cli.Command{
			Name: op.name, Usage: op.name + " nginx",
			Action: func(ctx context.Context, command *cli.Command) error {
				return executeCtlRequest(ctx, command, op.method, op.apiPath, nil)
			},
		})
	}
	return &cli.Command{Name: "nginx", Usage: "Inspect and control nginx", Commands: commands}
}

func ctlTokensCommand() *cli.Command {
	return &cli.Command{
		Name: "tokens", Usage: "Manage service tokens as an interactive administrator",
		Commands: []*cli.Command{
			{Name: "list", Usage: "List service tokens", Action: func(ctx context.Context, command *cli.Command) error {
				return executeCtlRequest(ctx, command, http.MethodGet, "service_tokens", nil)
			}},
			{
				Name: "create", Usage: "Create a service token",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "name", Required: true, Usage: "token name"},
					&cli.StringSliceFlag{Name: "scope", Required: true, Usage: "scope (repeat for multiple scopes)"},
					&cli.StringFlag{Name: "expires-at", Usage: "RFC3339 expiry time"},
				},
				Action: func(ctx context.Context, command *cli.Command) error {
					payload := map[string]any{"name": command.String("name"), "scopes": command.StringSlice("scope")}
					if value := command.String("expires-at"); value != "" {
						if _, err := time.Parse(time.RFC3339, value); err != nil {
							return fmt.Errorf("invalid --expires-at: %w", err)
						}
						payload["expires_at"] = value
					}
					return executeCtlRequest(ctx, command, http.MethodPost, "service_tokens", payload)
				},
			},
			{
				Name: "rotate", Usage: "Rotate a service token",
				Action: func(ctx context.Context, command *cli.Command) error {
					id := command.Args().First()
					if id == "" {
						return errors.New("token ID is required")
					}
					return executeCtlRequest(ctx, command, http.MethodPost, "service_tokens/"+url.PathEscape(id)+"/rotate", nil)
				},
			},
			{
				Name: "revoke", Usage: "Revoke a service token",
				Action: func(ctx context.Context, command *cli.Command) error {
					id := command.Args().First()
					if id == "" {
						return errors.New("token ID is required")
					}
					return executeCtlRequest(ctx, command, http.MethodDelete, "service_tokens/"+url.PathEscape(id), nil)
				},
			},
		},
	}
}

func parseNodeID(value string) (uint64, error) {
	if value == "" {
		return 0, nil
	}
	return strconv.ParseUint(value, 10, 64)
}
