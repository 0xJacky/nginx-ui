package demo

import (
	"context"

	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy/logger"
)

// externalNotifiers are every handler name registered by internal/notification.
// Each is re-registered over the real one; the registry is a plain map, so the
// later write wins.
var externalNotifiers = []string{
	"bark", "dingding", "gotify", "lark", "lark_custom",
	"mattermost", "ntfy", "telegram", "wecom",
}

// installNotifierStubs makes every external notifier a no-op.
//
// This is the one subsystem with no shared transport to swap: each notifier
// builds its own HTTP client internally, so the registry itself is the only
// unified point. It also closes a real gap — a demo visitor can create an
// enabled notifier pointing at any address, and the real handler would post to
// it.
func installNotifierStubs() {
	for _, name := range externalNotifiers {
		notification.RegisterExternalNotifier(name,
			func(_ context.Context, n *model.ExternalNotify, _ *notification.ExternalMessage) error {
				logger.Debugf("demo: dropped an outbound %s notification", n.Type)
				return nil
			})
	}
}
