package version

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/uozi-tech/cosy"
)

type ReleaseType string

const (
	ReleaseTypeStable     ReleaseType = "stable"
	ReleaseTypePrerelease ReleaseType = "prerelease"
	ReleaseTypeDev        ReleaseType = "dev"
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
	Type        ReleaseType     `json:"type"`
	HTMLURL     string          `json:"html_url"`
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
	resp, err := http.Get(GetGithubLatestReleaseAPIUrl())
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
		err = cosy.WrapErrorWithParams(ErrReleaseAPIFailed, string(body))
		return
	}
	err = json.Unmarshal(body, &data)
	if err != nil {
		err = errors.Wrap(err, "service.getLatestRelease json.Unmarshal err")
		return
	}
	data.Type = ReleaseTypeStable
	return
}

func getLatestPrerelease() (data TRelease, err error) {
	resp, err := http.Get(GetGithubReleasesListAPIUrl())
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
		err = cosy.WrapErrorWithParams(ErrReleaseAPIFailed, string(body))
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
			data.Type = ReleaseTypePrerelease
		}
	}

	return
}

func GetRelease(channel string) (data TRelease, err error) {
	stableRelease, err := getLatestRelease()
	if err != nil {
		return TRelease{}, err
	}

	switch ReleaseType(channel) {
	default:
		fallthrough
	case ReleaseTypeStable:
		return stableRelease, nil
	case ReleaseTypePrerelease:
		preRelease, err := getLatestPrerelease()
		if err != nil {
			return TRelease{}, err
		}
		// if preRelease is newer than stableRelease, return preRelease
		// otherwise return stableRelease
		if preRelease.PublishedAt.After(stableRelease.PublishedAt) {
			return preRelease, nil
		}
		return stableRelease, nil
	case ReleaseTypeDev:
		devRelease, err := getDevBuild()
		if err != nil {
			return TRelease{}, err
		}
		return devRelease, nil
	}
}
