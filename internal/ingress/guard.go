package ingress

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/internal/upstream/managed"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy"
)

// UpstreamOwner returns the ingress that generates the managed upstream
// called upstreamName, or "" when no ingress does.
func UpstreamOwner(upstreamName string) (string, error) {
	if !strings.HasPrefix(upstreamName, managed.IngressPrefix) {
		return "", nil
	}
	owners, err := UpstreamOwners()
	if err != nil {
		return "", err
	}
	return owners[upstreamName], nil
}

// UpstreamOwners maps the name of every managed upstream an ingress generates
// to that ingress, so a list can resolve all its owners with one query.
func UpstreamOwners() (map[string]string, error) {
	var all []*model.Ingress
	if err := model.UseDB().Select("name", "spec").Find(&all).Error; err != nil {
		return nil, err
	}
	owners := make(map[string]string)
	for _, ing := range all {
		for _, b := range ing.Spec.Backends {
			owners[UpstreamName(ing.Name, b.Name)] = ing.Name
		}
	}
	return owners, nil
}

// CheckUpstream refuses a change to a managed upstream an ingress generates
// unless it is forced. The next apply or reconcile overwrites a forced
// change.
func CheckUpstream(upstreamName string, force bool) error {
	if force {
		return nil
	}
	owner, err := UpstreamOwner(upstreamName)
	if err != nil || owner == "" {
		return err
	}
	return cosy.WrapErrorWithParams(managed.ErrManagedByIngress, upstreamName, owner)
}

// CheckManagedPath refuses, unless forced, a change to a file an ingress
// generates: its site file (site.CheckManagedPath) or one of its upstream
// files in conf.d. The errors are ErrSiteManagedByIngress and
// managed.ErrManagedByIngress, both 409xx codes.
func CheckManagedPath(absPath string, force bool) error {
	if force {
		return nil
	}
	if err := site.CheckManagedPath(absPath); err != nil {
		return err
	}
	if filepath.Clean(filepath.Dir(absPath)) != filepath.Clean(managed.Dir()) {
		return nil
	}
	name, ok := managed.NameFromFile(filepath.Base(absPath))
	if !ok {
		return nil
	}
	return CheckUpstream(name, false)
}

// IsManagedError reports whether err refuses a change to something an
// ingress generates.
func IsManagedError(err error) bool {
	return site.IsManagedByIngress(err) || isCosyError(err, managed.ErrManagedByIngress)
}

// isCosyError reports whether err carries the scope and code of target.
func isCosyError(err, target error) bool {
	var got, want *cosy.Error
	if !errors.As(err, &got) || !errors.As(target, &want) {
		return false
	}
	return got.Scope == want.Scope && got.Code == want.Code
}
