package version

import (
	"strings"

	"github.com/0xJacky/Nginx-UI/settings"
)

const (
	GithubDevCommitAPI     = "https://api.github.com/repos/0xJacky/nginx-ui/commits/dev?per_page=1"
	CloudflareWorkerAPI    = "https://cloud.nginxui.com/"
	GithubLatestReleaseAPI = "https://api.github.com/repos/0xJacky/nginx-ui/releases/latest"
	GithubReleasesListAPI  = "https://api.github.com/repos/0xJacky/nginx-ui/releases"
)

func GetGithubDevCommitAPIUrl() string {
	return CloudflareWorkerAPI + GithubDevCommitAPI
}

func GetGithubLatestReleaseAPIUrl() string {
	return CloudflareWorkerAPI + GithubLatestReleaseAPI
}

func GetGithubReleasesListAPIUrl() string {
	return CloudflareWorkerAPI + GithubReleasesListAPI
}

func GetCloudflareWorkerAPIUrl() string {
	return CloudflareWorkerAPI
}

func GetUrl(path string) string {
	githubProxy := settings.HTTPSettings.GithubProxy
	if githubProxy == "" {
		githubProxy = CloudflareWorkerAPI
	}
	githubProxy = strings.TrimSuffix(githubProxy, "/")

	return githubProxy + "/" + path
}
