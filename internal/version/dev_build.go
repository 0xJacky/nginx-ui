package version

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"
)

const (
	GithubDevCommitAPI  = "https://api.github.com/repos/0xJacky/nginx-ui/commits/dev?per_page=1"
	CloudflareWorkerAPI = "https://nginx-ui-proxy.langgood.com"
)

type TCommit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message   string `json:"message"`
		Committer struct {
			Date time.Time `json:"date"`
		} `json:"committer"`
	} `json:"commit"`
}

func getDevBuild() (data TRelease, err error) {
	resp, err := http.Get(GithubDevCommitAPI)
	if err != nil {
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	commit := TCommit{}
	err = json.Unmarshal(body, &commit)
	if err != nil {
		return
	}
	if len(commit.SHA) < 7 {
		err = errors.New("invalid commit SHA")
		return
	}
	shortSHA := commit.SHA[:7]

	resp, err = http.Get(fmt.Sprintf("%s/dev-builds", CloudflareWorkerAPI))
	if err != nil {
		return
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	assets := []TReleaseAsset{}
	err = json.Unmarshal(body, &assets)
	if err != nil {
		return
	}

	data = TRelease{
		TagName:     "sha-" + shortSHA,
		Name:        "sha-" + shortSHA,
		Body:        commit.Commit.Message,
		Type:        ReleaseTypeDev,
		PublishedAt: commit.Commit.Committer.Date,
		Assets:      assets,
	}

	return
}
