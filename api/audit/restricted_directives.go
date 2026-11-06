package audit

import (
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy/logger"
)

// incompleteRestrictedDirectiveList marks a record whose directive list could
// not be rebuilt, so the entry is not read as a complete one.
const incompleteRestrictedDirectiveList = "(the directive list could not be re-read after the save)"

// RecordRestrictedDirectiveSave records that an administrator saved a
// configuration with the restricted-directive denylist lifted. The denylist
// is a defence-in-depth control over code execution in the Nginx worker, so
// every save that lifts it is written to the session log with the file, the
// directives and who did it. The session logger carries the request id, so
// the entry sits next to the request's own audit entry in whatever sink is
// configured.
//
// complete reports whether the directives could be listed at all; see
// restrictedDirectiveAuditMessage.
func RecordRestrictedDirectiveSave(c *gin.Context, path string, found []config.RestrictedDirective, complete bool) {
	message, ok := restrictedDirectiveAuditMessage(path, found, complete)
	if !ok {
		return
	}

	userID := uint64(0)
	userName := ""
	if u, ok := c.Get("user"); ok {
		if currentUser, valid := u.(*model.User); valid {
			userID = currentUser.ID
			userName = currentUser.Name
		}
	}

	logger.NewSessionLogger(c).Warnf(
		"Restricted nginx directives saved after two-factor confirmation: path=%q directives=[%s] user_id=%d user=%q",
		path, message, userID, userName,
	)
}

// restrictedDirectiveAuditMessage lists the directives for the record and
// reports whether there is anything to record.
//
// A save that used the hatch is recorded whether or not the list could be
// rebuilt: the list is detail, the fact that the denylist was lifted is the
// record. Only a save that turned out to hold no restricted directive at all
// is silent, which is why an incomplete listing must not be taken for an
// empty one — an include that changed between the save and the listing would
// otherwise erase the entry.
func restrictedDirectiveAuditMessage(path string, found []config.RestrictedDirective, complete bool) (string, bool) {
	if complete && len(found) == 0 {
		return "", false
	}

	directives := make([]string, 0, len(found)+1)
	for _, directive := range found {
		if directive.File != "" && directive.File != path {
			directives = append(directives, directive.Directive+" ("+directive.File+")")
			continue
		}
		directives = append(directives, directive.Directive)
	}
	if !complete {
		directives = append(directives, incompleteRestrictedDirectiveList)
	}

	return strings.Join(directives, ", "), true
}
