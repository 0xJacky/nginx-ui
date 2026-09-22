package protocol

// NotifySendParams is the payload of notify.send.
type NotifySendParams struct {
	// Channel is the channel code declared in the manifest, without any host prefix.
	Channel string `json:"channel"`
	// Config holds the field values the user filled in, keyed by field key.
	Config map[string]string `json:"config"`
	// Title is already translated into the language of the notifier.
	Title string `json:"title"`
	// Content is already translated and rendered.
	Content string `json:"content"`
	// Severity is one of the NotifySeverity values.
	Severity string `json:"severity"`
}

// NotifyValidateParams is the payload of notify.validate.
type NotifyValidateParams struct {
	Channel string            `json:"channel"`
	Config  map[string]string `json:"config"`
}

// Values of NotifySendParams.Severity.
const (
	NotifySeverityInfo    = "info"
	NotifySeveritySuccess = "success"
	NotifySeverityWarning = "warning"
	NotifySeverityError   = "error"
)
