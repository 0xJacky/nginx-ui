package access_list

import (
	"fmt"
	"slices"
	"strings"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy"
)

// Render produces the file content of a list. Reference rules are expanded in
// place, recursively. Only the fallback of the rendered list itself is written:
// the fallback of a referenced list would end the rule set early and hide
// every rule after the reference.
//
// lists must contain every list a reference can reach; list itself may be a
// changed copy of a stored list.
func Render(list *model.AccessList, lists []*model.AccessList) (string, error) {
	byID := indexByID(lists)
	if list.ID != 0 {
		byID[list.ID] = list
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Managed by Nginx UI: access list %q (%s).\n", singleLine(list.Name), list.Slug)
	b.WriteString("# Changes made to this file are overwritten when the list is saved in Nginx UI.\n")

	if err := renderRules(&b, list, byID, []*model.AccessList{list}); err != nil {
		return "", err
	}

	fmt.Fprintf(&b, "%s all;\n", list.Fallback)
	return b.String(), nil
}

func renderRules(b *strings.Builder, list *model.AccessList, byID map[uint64]*model.AccessList, chain []*model.AccessList) error {
	for _, rule := range list.Rules {
		switch rule.Type {
		case model.AccessRuleAllow, model.AccessRuleDeny:
			b.WriteString(rule.Type)
			b.WriteByte(' ')
			b.WriteString(rule.Value)
			b.WriteByte(';')
			if note := singleLine(rule.Note); note != "" {
				b.WriteString(" # ")
				b.WriteString(note)
			}
			b.WriteByte('\n')
		case model.AccessRuleRef:
			ref, ok := byID[rule.RefID]
			if !ok {
				return cosy.WrapErrorWithParams(ErrReferenceNotFound, list.Name)
			}
			if slices.ContainsFunc(chain, func(l *model.AccessList) bool { return l.ID == ref.ID }) {
				return cosy.WrapErrorWithParams(ErrReferenceCycle, chainNames(append(chain, ref)))
			}
			label := fmt.Sprintf("%s (%s)", singleLine(ref.Name), ref.Slug)
			fmt.Fprintf(b, "# >>> %s\n", label)
			if err := renderRules(b, ref, byID, append(chain, ref)); err != nil {
				return err
			}
			fmt.Fprintf(b, "# <<< %s\n", label)
		}
	}
	return nil
}

// Dependents returns every list that reaches id through references, directly
// or through other lists. The result never contains id itself.
func Dependents(id uint64, lists []*model.AccessList) []*model.AccessList {
	seen := map[uint64]bool{id: true}
	var result []*model.AccessList
	queue := []uint64{id}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, l := range lists {
			if seen[l.ID] || !references(l, current) {
				continue
			}
			seen[l.ID] = true
			result = append(result, l)
			queue = append(queue, l.ID)
		}
	}
	return result
}

// DirectDependents returns the lists that reference id with one of their own
// rules.
func DirectDependents(id uint64, lists []*model.AccessList) []*model.AccessList {
	var result []*model.AccessList
	for _, l := range lists {
		if l.ID != id && references(l, id) {
			result = append(result, l)
		}
	}
	return result
}

func references(list *model.AccessList, id uint64) bool {
	for _, rule := range list.Rules {
		if rule.Type == model.AccessRuleRef && rule.RefID == id {
			return true
		}
	}
	return false
}

func chainNames(chain []*model.AccessList) string {
	names := make([]string, len(chain))
	for i, l := range chain {
		names[i] = singleLine(l.Name)
	}
	return strings.Join(names, " → ")
}

func singleLine(s string) string {
	return strings.TrimSpace(strings.NewReplacer("\r", " ", "\n", " ").Replace(s))
}
