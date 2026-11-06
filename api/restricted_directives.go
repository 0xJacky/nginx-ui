package api

import (
	"errors"
	"strings"

	"github.com/0xJacky/Nginx-UI/api/audit"
	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// AllowRestrictedDirectives reports whether a save may keep the restricted
// js_*, perl* and load_module statements of its content. Those statements
// load code into the Nginx worker, so they stay rejected unless the request
// asked for them and comes from an interactive administrator whose
// two-factor session was verified for this very request.
//
// A cluster node, a service token and MCP never pass the second condition, so
// a restricted configuration is neither accepted from them nor replicated to
// them. "user root" is not part of this at all: the validator rejects it for
// everyone.
func AllowRestrictedDirectives(c *gin.Context, optIn bool) bool {
	return optIn && middleware.RestrictedDirectiveSaveAllowed(c)
}

// RecordRestrictedDirectiveSave writes the audit entry for a save that kept
// restricted directives. Call it once the save has actually been applied. It
// does nothing for an ordinary save, so a handler can call it
// unconditionally.
func RecordRestrictedDirectiveSave(c *gin.Context, path string, content string, allowedRestricted bool) {
	if !allowedRestricted {
		return
	}

	found, complete := config.FindRestrictedDirectives(path, content)
	audit.RecordRestrictedDirectiveSave(c, path, found, complete)
}

// RespondRestrictedDirectiveError answers a save that was rejected over a
// restricted directive and reports true when it did, so a handler keeps its
// own handling for every other error.
//
// An interactive administrator who did not ask for the directives gets
// ErrConfigRestrictedDirectiveConfirmRequired, which the UI turns into the
// confirmation dialog, with every restricted statement the save would enable
// and the files holding them. An account with no second factor to step up
// with gets ErrConfigRestrictedDirectiveNeedsTwoFactor instead. A cluster
// node and a service token keep the plain ErrConfigDirectiveNotAllowed
// rejection they had before.
func RespondRestrictedDirectiveError(c *gin.Context, path string, content string, err error) bool {
	var restricted *config.RestrictedDirectiveError
	if !errors.As(err, &restricted) {
		return false
	}

	cosy.ErrHandler(c, restrictedDirectiveError(c, path, content, restricted))

	return true
}

// ConfigSaveError writes the response for a failed configuration save,
// translating a restricted-directive rejection and handing everything else to
// cosy unchanged.
func ConfigSaveError(c *gin.Context, path string, content string, err error) {
	if RespondRestrictedDirectiveError(c, path, content, err) {
		return
	}

	cosy.ErrHandler(c, err)
}

func restrictedDirectiveError(c *gin.Context, path string, content string, restricted *config.RestrictedDirectiveError) error {
	stepUp, err := middleware.RestrictedDirectiveStepUpFor(c)
	if err != nil {
		return err
	}
	if stepUp == middleware.RestrictedDirectiveStepUpImpossible {
		return restricted
	}

	found, _ := config.FindRestrictedDirectives(path, content)
	if len(found) == 0 {
		// The validation stopped at a statement this second walk no longer
		// reaches, for instance because an included file changed in between.
		// Report what stopped it rather than an empty list.
		found = []config.RestrictedDirective{restricted.Found}
	}

	directives := make([]string, 0, len(found))
	files := make([]string, 0, len(found))
	seenFiles := make(map[string]struct{}, len(found))
	for _, directive := range found {
		directives = append(directives, directive.Directive)

		file := directive.File
		if file == "" {
			file = path
		}
		if _, ok := seenFiles[file]; ok {
			continue
		}
		seenFiles[file] = struct{}{}
		files = append(files, file)
	}

	target := config.ErrConfigRestrictedDirectiveConfirmRequired
	if stepUp == middleware.RestrictedDirectiveStepUpUnavailable {
		target = config.ErrConfigRestrictedDirectiveNeedsTwoFactor
	}

	return cosy.WrapErrorWithParams(target, strings.Join(directives, ", "), strings.Join(files, ", "))
}
