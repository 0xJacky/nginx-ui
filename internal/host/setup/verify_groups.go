package setup

// CheckGroup selects a subset of the verification pipeline so a wizard step can
// verify what it just configured instead of waiting for the final step.
type CheckGroup string

const (
	// CheckGroupConnection covers reachability and the known_hosts allow list.
	CheckGroupConnection CheckGroup = "connection"
	// CheckGroupPlatform covers the service manager and the nginx paths.
	CheckGroupPlatform CheckGroup = "platform"
	// CheckGroupPrivileges covers passwordless sudo and the sudoers entries.
	CheckGroupPrivileges CheckGroup = "privileges"
	// CheckGroupNginx covers nginx -t, which is the only check with a cost.
	CheckGroupNginx CheckGroup = "nginx"
)

// AllCheckGroups lists every group in wizard order.
var AllCheckGroups = []CheckGroup{
	CheckGroupConnection,
	CheckGroupPlatform,
	CheckGroupPrivileges,
	CheckGroupNginx,
}

// ParseCheckGroups maps request values to known groups. Unknown values are
// dropped so an older client cannot widen the pipeline by accident.
func ParseCheckGroups(values []string) []CheckGroup {
	if len(values) == 0 {
		return nil
	}
	// A request that named groups but matched none must not widen back to the
	// full pipeline, so the slice stays non nil.
	groups := []CheckGroup{}
	for _, value := range values {
		for _, known := range AllCheckGroups {
			if CheckGroup(value) == known {
				groups = append(groups, known)
				break
			}
		}
	}
	return groups
}

type checkGroupFilter map[CheckGroup]bool

// newCheckGroupFilter returns a predicate over the requested groups. Only a nil
// slice means the full pipeline. ParseCheckGroups returns an empty non nil
// slice when a request named groups but matched none, and that must stay
// narrow rather than widening back to everything.
func newCheckGroupFilter(groups []CheckGroup) func(CheckGroup) bool {
	if groups == nil {
		return func(CheckGroup) bool { return true }
	}
	if len(groups) == 0 {
		return func(CheckGroup) bool { return false }
	}
	filter := make(checkGroupFilter, len(groups))
	for _, group := range groups {
		filter[group] = true
	}
	return func(group CheckGroup) bool { return filter[group] }
}
