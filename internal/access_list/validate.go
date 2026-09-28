package access_list

import (
	"net"
	"strconv"
	"strings"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy"
)

// Warning codes describe rules that are valid but most likely not what the
// author intended. They never block a save.
const (
	WarningShadowed   = "shadowed"
	WarningAllowsAll  = "allows_all"
	WarningHostBits   = "host_bits"
	WarningEmptyRules = "empty"
)

// Warning points at a rule (1-based, 0 for the list as a whole).
type Warning struct {
	Code string `json:"code"`
	Rule int    `json:"rule"`
}

// ValidAddress reports whether value is accepted by the allow and deny
// directives of ngx_http_access_module and ngx_stream_access_module.
func ValidAddress(value string) bool {
	if value == "all" || value == "unix:" {
		return true
	}
	if strings.Contains(value, "/") {
		_, _, err := net.ParseCIDR(value)
		return err == nil
	}
	return net.ParseIP(value) != nil
}

// hasHostBits reports a CIDR such as 192.168.1.1/24 whose address has bits set
// below the prefix. Nginx accepts it but warns that the low bits are ignored.
func hasHostBits(value string) bool {
	ip, network, err := net.ParseCIDR(value)
	if err != nil {
		return false
	}
	return !ip.Equal(network.IP)
}

// Validate checks a list before it is saved. lists holds every other stored
// list and is used to resolve references.
func Validate(list *model.AccessList, lists []*model.AccessList) error {
	name := strings.TrimSpace(list.Name)
	if name == "" || strings.ContainsAny(name, "\r\n") {
		return ErrInvalidName
	}
	if !ValidSlug(list.Slug) {
		return ErrInvalidSlug
	}
	for _, other := range lists {
		if other.ID != list.ID && other.Slug == list.Slug {
			return cosy.WrapErrorWithParams(ErrSlugExists, list.Slug)
		}
	}
	switch list.Fallback {
	case model.AccessFallbackDeny, model.AccessFallbackAllow:
	default:
		return cosy.WrapErrorWithParams(ErrInvalidFallback, list.Fallback)
	}

	byID := indexByID(lists)
	for i, rule := range list.Rules {
		n := strconv.Itoa(i + 1)
		if strings.ContainsAny(rule.Note, "\r\n") {
			return cosy.WrapErrorWithParams(ErrNoteMultiline, n)
		}
		switch rule.Type {
		case model.AccessRuleAllow, model.AccessRuleDeny:
			if !ValidAddress(rule.Value) {
				return cosy.WrapErrorWithParams(ErrInvalidAddress, n, rule.Value)
			}
		case model.AccessRuleRef:
			if list.ID != 0 && rule.RefID == list.ID {
				return cosy.WrapErrorWithParams(ErrReferenceSelf, n)
			}
			if _, ok := byID[rule.RefID]; !ok {
				return cosy.WrapErrorWithParams(ErrReferenceNotFound, n)
			}
		default:
			return cosy.WrapErrorWithParams(ErrInvalidRuleType, n)
		}
	}
	return nil
}

// Warnings lists the rules of an already valid list that deserve attention.
func Warnings(list *model.AccessList) []Warning {
	warnings := make([]Warning, 0)
	if len(list.Rules) == 0 {
		warnings = append(warnings, Warning{Code: WarningEmptyRules})
	}

	allowsOnly := true
	for i, rule := range list.Rules {
		if rule.Type == model.AccessRuleDeny || rule.Type == model.AccessRuleRef {
			allowsOnly = false
		}
		if rule.Type == model.AccessRuleRef {
			continue
		}
		if rule.Value == "all" && i < len(list.Rules)-1 {
			warnings = append(warnings, Warning{Code: WarningShadowed, Rule: i + 1})
		}
		if hasHostBits(rule.Value) {
			warnings = append(warnings, Warning{Code: WarningHostBits, Rule: i + 1})
		}
	}
	if allowsOnly && list.Fallback == model.AccessFallbackAllow {
		warnings = append(warnings, Warning{Code: WarningAllowsAll})
	}
	return warnings
}

func indexByID(lists []*model.AccessList) map[uint64]*model.AccessList {
	byID := make(map[uint64]*model.AccessList, len(lists))
	for _, l := range lists {
		byID[l.ID] = l
	}
	return byID
}
