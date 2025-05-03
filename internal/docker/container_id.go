package docker

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

// GetContainerID retrieves the Docker container ID by parsing /proc/self/mountinfo
func GetContainerID() (string, error) {
	// Open the mountinfo file
	file, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Regular expression to extract container ID from paths like:
	// /var/lib/docker/containers/bd4bd482f7e28566389fe7e4ce6b168e93b372c3fc18091c37923588664ca950/resolv.conf
	containerIDPattern := regexp.MustCompile(`/var/lib/docker/containers/([a-f0-9]{64})/`)

	// Scan the file line by line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// Look for container ID in the line
		if strings.Contains(line, "/var/lib/docker/containers/") {
			matches := containerIDPattern.FindStringSubmatch(line)
			if len(matches) >= 2 {
				return matches[1], nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", os.ErrNotExist
}
