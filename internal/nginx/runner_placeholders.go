package nginx

// Placeholders replaced by runner_docker.go (Task 3) and runner_ssh.go (Task 5).
// They MUST remain in this file until both follow-up tasks are committed,
// otherwise the package fails to compile.

func newSSHRunner() Runner    { return &localRunner{} }
func newDockerRunner() Runner { return &localRunner{} }
