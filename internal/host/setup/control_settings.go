package setup

import (
	"regexp"

	"github.com/0xJacky/Nginx-UI/settings"
)

// Validation failures for the nginx control settings form. Codes continue the
// host_setup scope; 520011-520019 are left free for the SSH key handling and
// 520023 is retired (it rejected non-key SSH auth, which no longer exists).
var (
	ErrContainerNameRequired     = e.New(520020, "container name is required for external container mode")
	ErrInvalidContainerName      = e.New(520021, "container name contains invalid characters")
	ErrSSHConnectionRequired     = e.New(520022, "host address and user are required for SSH mode")
	ErrInvalidKeySource          = e.New(520024, "SSH mode requires a valid private key source")
	ErrSSHKeyPathsRequired       = e.New(520025, "private key and known hosts paths are required for SSH mode")
	ErrUnsupportedServiceManager = e.New(520026, "SSH mode requires a supported service manager")
	ErrInvalidControlMode        = e.New(520027, "invalid nginx control mode")
)

// containerNamePattern mirrors the docker daemon's name rule. The value is
// passed to the docker API, never to a shell, so the check only guards
// against obviously malformed input.
var containerNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

const maxContainerNameLength = 255

// ControlSettings is the mode-dependent subset of the nginx control settings
// that must be consistent before it is persisted. Callers trim the values
// before validation; this type deliberately does not.
type ControlSettings struct {
	Mode               string
	ContainerName      string
	HostAddress        string
	HostUser           string
	HostAccessMode     string
	HostKeySource      string
	HostPrivateKeyPath string
	HostKnownHostsPath string
	HostServiceManager string
}

// ValidateControlSettings checks that the fields required by the selected
// control mode are present and well formed. Fields belonging to other modes
// are ignored so that switching modes never trips on stale values.
func ValidateControlSettings(s ControlSettings) error {
	switch s.Mode {
	case settings.ControlModeLocal:
		return nil
	case settings.ControlModeExternalContainer:
		return validateContainerSettings(s)
	case settings.ControlModeHostViaSSH:
		return validateSSHSettings(s)
	default:
		return ErrInvalidControlMode
	}
}

func validateContainerSettings(s ControlSettings) error {
	if s.ContainerName == "" {
		return ErrContainerNameRequired
	}
	if len(s.ContainerName) > maxContainerNameLength || !containerNamePattern.MatchString(s.ContainerName) {
		return ErrInvalidContainerName
	}
	return nil
}

func validateSSHSettings(s ControlSettings) error {
	if s.HostAddress == "" || s.HostUser == "" {
		return ErrSSHConnectionRequired
	}
	if err := ValidateHostUser(s.HostUser); err != nil {
		return err
	}
	if s.HostAccessMode != settings.HostAccessModeSFTP &&
		s.HostAccessMode != settings.HostAccessModeMounted {
		return ErrInvalidAccessMode
	}
	if s.HostKeySource != settings.HostKeySourceGenerated &&
		s.HostKeySource != settings.HostKeySourceExisting &&
		s.HostKeySource != settings.HostKeySourceProvided {
		return ErrInvalidKeySource
	}
	if s.HostPrivateKeyPath == "" || s.HostKnownHostsPath == "" {
		return ErrSSHKeyPathsRequired
	}
	if s.HostServiceManager != settings.HostServiceManagerSystemd &&
		s.HostServiceManager != settings.HostServiceManagerLaunchd {
		return ErrUnsupportedServiceManager
	}
	return nil
}
