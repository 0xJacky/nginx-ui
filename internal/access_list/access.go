package access_list

import (
	"strconv"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/uozi-tech/cosy"
)

// Access modes of a server block or a location.
//
// A server is public or uses a list. A location inherits the server, is public
// (allow all), uses its own list, or is kept reachable for ACME challenges by
// Nginx UI. Allow and deny rules that Nginx UI did not write make a block
// manual: it is shown but never rewritten.
const (
	ModePublic  = "public"
	ModeInherit = "inherit"
	ModeList    = "list"
	ModeACME    = "acme"
	ModeManual  = "manual"
)

// ACMEMarker precedes the allow all directive Nginx UI adds to an ACME
// challenge location of a restricted server. It is a comment line of its own
// because the basic mode editor keeps leading comments but drops inline ones.
const ACMEMarker = "# nginx-ui: keep the ACME challenge reachable"

const acmeChallengePath = "/.well-known/acme-challenge"

// ServerState is the access state of one server block.
type ServerState struct {
	Index      int             `json:"index"`
	ServerName string          `json:"server_name"`
	Mode       string          `json:"mode"`
	Slug       string          `json:"slug,omitempty"`
	Locations  []LocationState `json:"locations"`
}

// LocationState is the access state of one location of a server block.
type LocationState struct {
	Index int    `json:"index"`
	Path  string `json:"path"`
	Mode  string `json:"mode"`
	Slug  string `json:"slug,omitempty"`
	// ACME marks an ACME challenge location. Nginx UI keeps it reachable while
	// the server uses a list and the location inherits.
	ACME bool `json:"acme"`
}

// Change sets the access mode of a server block or, when Location is set, of
// one of its locations. Indexes follow the order of the configuration.
type Change struct {
	Server   int    `json:"server"`
	Location *int   `json:"location,omitempty"`
	Mode     string `json:"mode"`
	Slug     string `json:"slug,omitempty"`
}

// blockAccess summarises the access directives directly inside a block.
type blockAccess struct {
	slugs   []string
	rules   int
	public  bool
	acme    bool
	managed []*statement
}

func inspectStatements(children []*statement) blockAccess {
	var a blockAccess
	var allowAll *statement
	for _, st := range children {
		switch st.Name {
		case "include":
			if slug, ok := SlugFromInclude(st.arg(0)); ok && len(st.Args) == 1 {
				a.slugs = append(a.slugs, slug)
				a.managed = append(a.managed, st)
			}
		case "allow", "deny":
			a.rules++
			if st.Name == "allow" && len(st.Args) == 1 && st.Args[0] == "all" {
				allowAll = st
			}
		}
	}
	if a.rules == 1 && allowAll != nil {
		a.public = true
		a.acme = hasMarker(allowAll)
		a.managed = append(a.managed, allowAll)
	}
	return a
}

func hasMarker(st *statement) bool {
	for _, c := range st.Comments {
		if strings.TrimSpace(c.Text) == ACMEMarker {
			return true
		}
	}
	return false
}

func serverMode(a blockAccess) (string, string) {
	switch {
	case len(a.slugs) == 0 && a.rules == 0:
		return ModePublic, ""
	case len(a.slugs) == 1 && a.rules == 0:
		return ModeList, a.slugs[0]
	default:
		return ModeManual, ""
	}
}

func locationMode(a blockAccess) (string, string) {
	switch {
	case len(a.slugs) == 0 && a.rules == 0:
		return ModeInherit, ""
	case len(a.slugs) == 1 && a.rules == 0:
		return ModeList, a.slugs[0]
	case len(a.slugs) == 0 && a.public && a.acme:
		return ModeACME, ""
	case len(a.slugs) == 0 && a.public:
		return ModePublic, ""
	default:
		return ModeManual, ""
	}
}

func isACMEPath(path string) bool {
	return strings.Contains(path, acmeChallengePath)
}

func serverName(children []*statement) string {
	for _, st := range children {
		if st.Name == "server_name" {
			return strings.Join(st.Args, " ")
		}
	}
	return ""
}

// ---- text mode -------------------------------------------------------------

// State reports the access state of every server block of a configuration
// file.
func State(content string) ([]ServerState, error) {
	roots, err := parseStatements(content)
	if err != nil {
		return nil, err
	}

	servers := serverBlocks(roots)
	states := make([]ServerState, 0, len(servers))
	for i, server := range servers {
		mode, slug := serverMode(inspectStatements(server.Children))
		state := ServerState{
			Index:      i,
			ServerName: serverName(server.Children),
			Mode:       mode,
			Slug:       slug,
			Locations:  make([]LocationState, 0),
		}
		for j, location := range locationBlocks(server) {
			lMode, lSlug := locationMode(inspectStatements(location.Children))
			state.Locations = append(state.Locations, LocationState{
				Index: j,
				Path:  locationPath(location),
				Mode:  lMode,
				Slug:  lSlug,
				ACME:  isACMEPath(locationPath(location)),
			})
		}
		states = append(states, state)
	}
	return states, nil
}

// Apply rewrites the access directives of a configuration file. Only the
// directives Nginx UI manages are touched; every other byte of the file is kept.
func Apply(content string, changes []Change) (string, error) {
	var err error
	for _, change := range changes {
		if err = validateChange(change); err != nil {
			return "", err
		}
		content, err = applyText(content, change)
		if err != nil {
			return "", err
		}
	}
	return reconcileACMEText(content)
}

func validateChange(change Change) error {
	switch change.Mode {
	case ModePublic:
	case ModeList:
		if !ValidSlug(change.Slug) {
			return ErrInvalidSlug
		}
	case ModeInherit:
		if change.Location == nil {
			return cosy.WrapErrorWithParams(ErrInvalidMode, change.Mode)
		}
	default:
		return cosy.WrapErrorWithParams(ErrInvalidMode, change.Mode)
	}
	return nil
}

// locateText finds the block a change addresses in freshly parsed content.
func locateText(content string, server int, location *int) (*statement, error) {
	roots, err := parseStatements(content)
	if err != nil {
		return nil, err
	}
	servers := serverBlocks(roots)
	if server < 0 || server >= len(servers) {
		return nil, cosy.WrapErrorWithParams(ErrServerNotFound, strconv.Itoa(server+1))
	}
	if location == nil {
		return servers[server], nil
	}
	locations := locationBlocks(servers[server])
	if *location < 0 || *location >= len(locations) {
		return nil, cosy.WrapErrorWithParams(ErrLocationNotFound, strconv.Itoa(*location+1), strconv.Itoa(server+1))
	}
	return locations[*location], nil
}

func applyText(content string, change Change) (string, error) {
	block, err := locateText(content, change.Server, change.Location)
	if err != nil {
		return "", err
	}
	isLocation := change.Location != nil

	access := inspectStatements(block.Children)
	mode, _ := serverMode(access)
	name := "server " + strconv.Itoa(change.Server+1)
	if isLocation {
		mode, _ = locationMode(access)
		name = "location " + locationPath(block)
	}
	if mode == ModeManual {
		return "", cosy.WrapErrorWithParams(ErrManualRules, name)
	}

	content = removeSpans(content, managedSpans(access.managed))
	if lines := targetLines(change, isLocation); len(lines) > 0 {
		block, err = locateText(content, change.Server, change.Location)
		if err != nil {
			return "", err
		}
		var anchor *statement
		if !isLocation {
			anchor = serverAnchor(block.Children)
		}
		content = insertLines(content, block, false, anchor, lines)
	}
	return content, nil
}

func managedSpans(managed []*statement) []span {
	spans := make([]span, 0, len(managed))
	for _, st := range managed {
		spans = append(spans, span{start: st.Start, end: st.End})
		for _, c := range st.Comments {
			if strings.TrimSpace(c.Text) == ACMEMarker {
				spans = append(spans, span{start: c.Start, end: c.End})
			}
		}
	}
	return spans
}

// targetLines returns the directives that express a change.
func targetLines(change Change, isLocation bool) []string {
	switch change.Mode {
	case ModeList:
		return []string{"include " + IncludePath(change.Slug) + ";"}
	case ModePublic:
		if isLocation {
			return []string{"allow all;"}
		}
	}
	return nil
}

// serverAnchor picks the directive the server level include follows: the
// server name when there is one, so the include sits next to the identity of
// the server, otherwise the last listen directive.
func serverAnchor(children []*statement) *statement {
	var anchor *statement
	for _, st := range children {
		switch st.Name {
		case "server_name":
			return st
		case "listen":
			anchor = st
		}
	}
	return anchor
}

// reconcileACMEText keeps the ACME challenge locations of restricted servers
// reachable and drops the exception again once a server is public.
func reconcileACMEText(content string) (string, error) {
	states, err := State(content)
	if err != nil {
		return "", err
	}
	for _, server := range states {
		restricted := server.Mode == ModeList
		for _, location := range server.Locations {
			if !location.ACME {
				continue
			}
			idx := location.Index
			switch {
			case restricted && location.Mode == ModeInherit:
				block, err := locateText(content, server.Index, &idx)
				if err != nil {
					return "", err
				}
				content = insertLines(content, block, false, nil, []string{ACMEMarker, "allow all;"})
			case !restricted && location.Mode == ModeACME:
				block, err := locateText(content, server.Index, &idx)
				if err != nil {
					return "", err
				}
				content = removeSpans(content, managedSpans(inspectStatements(block.Children).managed))
			}
		}
	}
	return content, nil
}

// ---- basic mode ------------------------------------------------------------

// StateOf reports the access state of the servers of a basic mode editor
// configuration.
func StateOf(cfg *nginx.NgxConfig) ([]ServerState, error) {
	states := make([]ServerState, 0, len(cfg.Servers))
	for i, server := range cfg.Servers {
		mode, slug := serverMode(inspectDirectives(server.Directives))
		state := ServerState{
			Index:      i,
			ServerName: directiveParams(server.Directives, "server_name"),
			Mode:       mode,
			Slug:       slug,
			Locations:  make([]LocationState, 0, len(server.Locations)),
		}
		for j, location := range server.Locations {
			roots, err := parseStatements(location.Content)
			if err != nil {
				return nil, err
			}
			lMode, lSlug := locationMode(inspectStatements(roots))
			state.Locations = append(state.Locations, LocationState{
				Index: j,
				Path:  location.Path,
				Mode:  lMode,
				Slug:  lSlug,
				ACME:  isACMEPath(location.Path),
			})
		}
		states = append(states, state)
	}
	return states, nil
}

// ApplyTo rewrites the access directives of a basic mode editor configuration
// in place.
func ApplyTo(cfg *nginx.NgxConfig, changes []Change) error {
	for _, change := range changes {
		if err := validateChange(change); err != nil {
			return err
		}
		if change.Server < 0 || change.Server >= len(cfg.Servers) {
			return cosy.WrapErrorWithParams(ErrServerNotFound, strconv.Itoa(change.Server+1))
		}
		server := cfg.Servers[change.Server]

		if change.Location == nil {
			if err := applyServerDirectives(server, change); err != nil {
				return err
			}
			continue
		}

		idx := *change.Location
		if idx < 0 || idx >= len(server.Locations) {
			return cosy.WrapErrorWithParams(ErrLocationNotFound, strconv.Itoa(idx+1), strconv.Itoa(change.Server+1))
		}
		content, err := applyLocationContent(server.Locations[idx], change)
		if err != nil {
			return err
		}
		server.Locations[idx].Content = content
	}
	return reconcileACMEStructured(cfg)
}

func inspectDirectives(directives []*nginx.NgxDirective) blockAccess {
	var a blockAccess
	for _, d := range directives {
		switch d.Directive {
		case "include":
			if slug, ok := SlugFromInclude(strings.TrimSuffix(strings.TrimSpace(d.Params), ";")); ok {
				a.slugs = append(a.slugs, slug)
			}
		case "allow", "deny":
			a.rules++
		}
	}
	return a
}

func directiveParams(directives []*nginx.NgxDirective, name string) string {
	for _, d := range directives {
		if d.Directive == name {
			return strings.TrimSpace(d.Params)
		}
	}
	return ""
}

func applyServerDirectives(server *nginx.NgxServer, change Change) error {
	if mode, _ := serverMode(inspectDirectives(server.Directives)); mode == ModeManual {
		return cosy.WrapErrorWithParams(ErrManualRules, "server "+strconv.Itoa(change.Server+1))
	}

	kept := make([]*nginx.NgxDirective, 0, len(server.Directives)+1)
	for _, d := range server.Directives {
		if d.Directive == "include" {
			if _, ok := SlugFromInclude(strings.TrimSuffix(strings.TrimSpace(d.Params), ";")); ok {
				continue
			}
		}
		kept = append(kept, d)
	}

	if change.Mode == ModeList {
		at := 0
		for i, d := range kept {
			if d.Directive == "server_name" {
				at = i + 1
				break
			}
			if d.Directive == "listen" {
				at = i + 1
			}
		}
		include := &nginx.NgxDirective{Directive: "include", Params: IncludePath(change.Slug)}
		kept = append(kept[:at], append([]*nginx.NgxDirective{include}, kept[at:]...)...)
	}
	server.Directives = kept
	return nil
}

func applyLocationContent(location *nginx.NgxLocation, change Change) (string, error) {
	content := location.Content
	roots, err := parseStatements(content)
	if err != nil {
		return "", err
	}
	access := inspectStatements(roots)
	if mode, _ := locationMode(access); mode == ModeManual {
		return "", cosy.WrapErrorWithParams(ErrManualRules, "location "+location.Path)
	}

	content = removeSpans(content, managedSpans(access.managed))
	if lines := targetLines(change, true); len(lines) > 0 {
		roots, err = parseStatements(content)
		if err != nil {
			return "", err
		}
		root := &statement{BodyStart: 0, BodyEnd: len(content), Children: roots}
		content = insertLines(content, root, true, nil, lines)
	}
	return content, nil
}

func reconcileACMEStructured(cfg *nginx.NgxConfig) error {
	states, err := StateOf(cfg)
	if err != nil {
		return err
	}
	for _, server := range states {
		restricted := server.Mode == ModeList
		for _, location := range server.Locations {
			if !location.ACME {
				continue
			}
			target := cfg.Servers[server.Index].Locations[location.Index]
			switch {
			case restricted && location.Mode == ModeInherit:
				target.Content = ACMEMarker + "\nallow all;\n" + target.Content
			case !restricted && location.Mode == ModeACME:
				roots, err := parseStatements(target.Content)
				if err != nil {
					return err
				}
				target.Content = removeSpans(target.Content, managedSpans(inspectStatements(roots).managed))
			}
		}
	}
	return nil
}

// ModeMixed summarises a file whose server blocks use different access modes
// or lists.
const ModeMixed = "mixed"

// Summarize condenses the server level access of a site or stream file for
// list pages. It reports an empty mode when the file cannot be parsed.
func Summarize(content string) (mode, slug string) {
	states, err := State(content)
	if err != nil {
		return "", ""
	}
	if len(states) == 0 {
		return ModePublic, ""
	}
	mode, slug = states[0].Mode, states[0].Slug
	for _, state := range states[1:] {
		if state.Mode == ModeManual {
			return ModeManual, ""
		}
		if state.Mode != mode || state.Slug != slug {
			mode, slug = ModeMixed, ""
		}
	}
	if mode == ModeManual {
		return ModeManual, ""
	}
	return mode, slug
}
