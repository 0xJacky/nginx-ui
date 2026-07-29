package setup

import (
	"context"
	"strings"
)

const (
	remediationAddBindMount                  = "add_bind_mount"
	remediationAddMissingSudoersEntries      = "add_missing_sudoers_entries"
	remediationCheckHostAddress              = "check_host_address"
	remediationCheckHostAddressOutsideDocker = "check_host_address_outside_docker"
	remediationCheckSSHConnection            = "check_ssh_connection"
	remediationCheckSystemdUnit              = "check_systemd_unit"
	remediationConfirmHomebrewServiceOwner   = "confirm_homebrew_service_owner"
	remediationConfirmHostNginxDirectory     = "confirm_host_nginx_directory"
	remediationConfirmMacOSHostAlias         = "confirm_macos_host_alias"
	remediationConfirmUname                  = "confirm_uname"
	remediationCorrectParameters             = "correct_parameters"
	remediationFixNginxConfig                = "fix_nginx_config"
	remediationInspectSudoPermissions        = "inspect_sudo_permissions"
	remediationInspectSystemdUnit            = "inspect_systemd_unit"
	remediationMountNginxLogs                = "mount_nginx_logs"
	remediationMountPIDDirectory             = "mount_pid_directory"
	remediationPersistKnownHosts             = "persist_known_hosts"
	remediationReplaceBindMount              = "replace_bind_mount"
	remediationRestartWithoutExecReload      = "restart_without_exec_reload"
	remediationReviewCrossHostGuide          = "review_cross_host_guide"
	remediationReviewInstallPermissions      = "review_install_permissions"
	remediationReviewSudoersRules            = "review_sudoers_rules"
	remediationSelectServiceManager          = "select_service_manager"
	remediationStartHomebrewNginx            = "start_homebrew_nginx"
	remediationUseClusterNode                = "use_cluster_node"
	remediationUseDockerHostAlias            = "use_docker_host_alias"
	remediationUseMacOSHostAlias             = "use_macos_host_alias"
	remediationVerifyBindMount               = "verify_bind_mount"
)

// StepRemediation identifies a translatable UI message and its interpolation
// values without coupling the backend to a display language.
type StepRemediation struct {
	Code   string            `json:"code"`
	Params map[string]string `json:"params"`
}

func newStepRemediation(code string, params ...map[string]string) *StepRemediation {
	values := map[string]string{}
	if len(params) > 0 {
		values = params[0]
	}
	return &StepRemediation{Code: code, Params: values}
}

// CommandRunner is the part of the SSH client the verify pipeline uses.
// Depending on the interface rather than the concrete client lets the checks
// be exercised without a live host.
type CommandRunner interface {
	Exec(ctx context.Context, name string, args ...string) (string, error)
}

func checkKnownHostsPersistence(path string) StepOutcome {
	if path == "" {
		path = "/etc/nginx-ui/known_hosts"
	}
	if strings.HasPrefix(path, "/etc/nginx-ui/") {
		return StepOutcome{OK: true, Level: "success", Detail: path + " is under the recommended persisted data directory"}
	}
	return StepOutcome{
		OK:          false,
		Level:       "warning",
		Detail:      path + " is outside the recommended /etc/nginx-ui data directory",
		Remediation: newStepRemediation(remediationPersistKnownHosts),
	}
}
