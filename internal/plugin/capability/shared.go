package capability

import (
	"errors"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
)

// PluginTypePrefix marks a notifier type or a probe kind that a plugin
// provides. It keeps the plugin codes apart from the names of the built-in
// notifiers and checks, so a plugin can neither replace a built-in one nor be
// shadowed by one added later (spec NOTIFY-9, PROBE-7).
const PluginTypePrefix = "plugin:"

// PluginType is the host side name of a plugin channel or probe kind code.
func PluginType(code string) string {
	return PluginTypePrefix + code
}

// pluginCode strips PluginTypePrefix. ok is false for a built-in name.
func pluginCode(name string) (code string, ok bool) {
	code, ok = strings.CutPrefix(name, PluginTypePrefix)
	return code, ok && code != ""
}

// invalidConfigField returns the field and message of a CodeInvalidConfig
// error. ok is false for any other error.
func invalidConfigField(err error) (field, message string, ok bool) {
	var perr *protocol.Error
	if !errors.As(err, &perr) || perr.Code != protocol.CodeInvalidConfig {
		return "", "", false
	}
	switch data := perr.Data.(type) {
	case map[string]any:
		field, _ = data["field"].(string)
	case protocol.InvalidConfigData:
		field = data.Field
	case *protocol.InvalidConfigData:
		if data != nil {
			field = data.Field
		}
	}
	return field, perr.Message, true
}

// rpcMessage is the peer message of a JSON-RPC error, or the error text.
func rpcMessage(err error) string {
	var perr *protocol.Error
	if errors.As(err, &perr) {
		return perr.Message
	}
	return err.Error()
}

// configurationFields flattens the manifest form schema.
func configurationFields(schema *protocol.ConfigurationSchema) []protocol.ConfigurationField {
	if schema == nil {
		return nil
	}
	return schema.Fields
}
