package upgrader

import (
	"encoding/json"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"time"
)

const (
	GithubLatestReleaseAPI = "https://api.github.com/repos/0xJacky/nginx-ui/releases/latest"
	GithubReleasesListAPI  = "https://api.github.com/repos/0xJacky/nginx-ui/releases"
)

type TReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadUrl string `json:"browser_download_url"`
	Size               uint   `json:"size"`
}

type TRelease struct {
	TagName     string          `json:"tag_name"`
	Name        string          `json:"name"`
	PublishedAt time.Time       `json:"published_at"`
	Body        string          `json:"body"`
	Prerelease  bool            `json:"prerelease"`
	Assets      []TReleaseAsset `json:"assets"`
}

func (t *TRelease) GetAssetsMap() (m map[string]TReleaseAsset) {
	m = make(map[string]TReleaseAsset)
	for _, v := range t.Assets {
		m[v.Name] = v
	}
	return
}

func getLatestRelease() (data TRelease, err error) {
	resp, err := http.Get(GithubLatestReleaseAPI)
	if err != nil {
		err = errors.Wrap(err, "service.getLatestRelease http.Get err")
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		err = errors.Wrap(err, "service.getLatestRelease io.ReadAll err")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		err = errors.New(string(body))
		return
	}
	err = json.Unmarshal(body, &data)
	if err != nil {
		err = errors.Wrap(err, "service.getLatestRelease json.Unmarshal err")
		return
	}
	return
}

func getLatestPrerelease() (data TRelease, err error) {
	resp, err := http.Get(GithubReleasesListAPI)
	if err != nil {
		err = errors.Wrap(err, "service.getLatestPrerelease http.Get err")
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		err = errors.Wrap(err, "service.getLatestPrerelease io.ReadAll err")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		err = errors.New(string(body))
		return
	}

	var releaseList []TRelease

	err = json.Unmarshal(body, &releaseList)
	if err != nil {
		err = errors.Wrap(err, "service.getLatestPrerelease json.Unmarshal err")
		return
	}

	latestDate := time.Time{}

	for _, release := range releaseList {
		if release.Prerelease && release.PublishedAt.After(latestDate) {
			data = release
			latestDate = release.PublishedAt
		}
	}

	return
}

func GetRelease(channel string) (data TRelease, err error) {
	switch channel {
	default:
		fallthrough
	case "stable":
		return getLatestRelease()
	case "prerelease":
		return getLatestPrerelease()
	}
}
