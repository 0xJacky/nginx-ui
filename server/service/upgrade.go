package service

import (
	"encoding/json"
	"fmt"
	_github "github.com/0xJacky/Nginx-UI/.github"
	"github.com/0xJacky/Nginx-UI/frontend"
	"github.com/0xJacky/Nginx-UI/server/internal/helper"
	"github.com/0xJacky/Nginx-UI/server/internal/logger"
	"github.com/0xJacky/Nginx-UI/server/settings"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	GithubLatestReleaseAPI = "https://api.github.com/repos/0xJacky/nginx-ui/releases/latest"
	GithubReleasesListAPI  = "https://api.github.com/repos/0xJacky/nginx-ui/releases"
)

type RuntimeInfo struct {
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	ExPath string `json:"ex_path"`
}

func GetRuntimeInfo() (r RuntimeInfo, err error) {
	ex, err := os.Executable()
	if err != nil {
		err = errors.Wrap(err, "service.GetRuntimeInfo os.Executable() err")
		return
	}
	realPath, err := filepath.EvalSymlinks(ex)
	if err != nil {
		err = errors.Wrap(err, "service.GetRuntimeInfo filepath.EvalSymlinks() err")
		return
	}

	r = RuntimeInfo{
		OS:     runtime.GOOS,
		Arch:   runtime.GOARCH,
		ExPath: realPath,
	}

	return
}

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

type CurVersion struct {
	Version    string `json:"version"`
	BuildID    int    `json:"build_id"`
	TotalBuild int    `json:"total_build"`
}

func GetCurrentVersion() (c CurVersion, err error) {
	verJson, err := frontend.DistFS.ReadFile("dist/version.json")
	if err != nil {
		err = errors.Wrap(err, "service.GetCurrentVersion ReadFile err")
		return
	}

	err = json.Unmarshal(verJson, &c)
	if err != nil {
		err = errors.Wrap(err, "service.GetCurrentVersion json.Unmarshal err")
		return
	}

	return
}

type Upgrader struct {
	Release TRelease
	RuntimeInfo
}

func NewUpgrader(channel string) (u *Upgrader, err error) {
	data, err := GetRelease(channel)
	if err != nil {
		return
	}
	runtimeInfo, err := GetRuntimeInfo()
	if err != nil {
		return
	}
	u = &Upgrader{
		Release:     data,
		RuntimeInfo: runtimeInfo,
	}
	return
}

type ProgressWriter struct {
	io.Writer
	totalSize    int64
	currentSize  int64
	progressChan chan<- float64
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n, err := pw.Writer.Write(p)
	pw.currentSize += int64(n)
	progress := float64(pw.currentSize) / float64(pw.totalSize) * 100
	pw.progressChan <- progress
	return n, err
}

func downloadRelease(url string, dir string, progressChan chan float64) (tarName string, err error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	totalSize, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return
	}

	file, err := os.CreateTemp(dir, "nginx-ui-temp-*.tar.gz")
	if err != nil {
		err = errors.Wrap(err, "service.DownloadLatestRelease CreateTemp error")
		return
	}
	defer file.Close()

	progressWriter := &ProgressWriter{Writer: file, totalSize: totalSize, progressChan: progressChan}
	multiWriter := io.MultiWriter(progressWriter)

	_, err = io.Copy(multiWriter, resp.Body)
	close(progressChan)

	tarName = file.Name()
	return
}

func (u *Upgrader) DownloadLatestRelease(progressChan chan float64) (tarName string, err error) {
	bytes, err := _github.DistFS.ReadFile("build/build_info.json")
	if err != nil {
		err = errors.Wrap(err, "service.DownloadLatestRelease Read build_info.json error")
		return
	}
	type buildArch struct {
		Arch string `json:"arch"`
		Name string `json:"name"`
	}
	var buildJson map[string]map[string]buildArch

	_ = json.Unmarshal(bytes, &buildJson)

	build, ok := buildJson[u.OS]
	if !ok {
		err = errors.Wrap(err, "os not support upgrade")
		return
	}
	arch, ok := build[u.Arch]
	if !ok {
		err = errors.Wrap(err, "arch not support upgrade")
		return
	}

	assetsMap := u.Release.GetAssetsMap()

	// asset
	asset, ok := assetsMap[fmt.Sprintf("nginx-ui-%s.tar.gz", arch.Name)]

	if !ok {
		err = errors.Wrap(err, "upgrader core asset is empty")
		return
	}

	downloadUrl := asset.BrowserDownloadUrl
	if downloadUrl == "" {
		err = errors.New("upgrader core downloadUrl is empty")
		return
	}

	// digest
	digest, ok := assetsMap[fmt.Sprintf("nginx-ui-%s.tar.gz.digest", arch.Name)]

	if !ok || digest.BrowserDownloadUrl == "" {
		err = errors.New("upgrader core digest is empty")
		return
	}

	resp, err := http.Get(digest.BrowserDownloadUrl)

	if err != nil {
		err = errors.Wrap(err, "upgrader core download digest fail")
		return
	}

	defer resp.Body.Close()

	dir := filepath.Dir(u.ExPath)

	if settings.ServerSettings.GithubProxy != "" {
		downloadUrl, err = url.JoinPath(settings.ServerSettings.GithubProxy, downloadUrl)
		if err != nil {
			err = errors.Wrap(err, "service.DownloadLatestRelease url.JoinPath error")
			return
		}
	}

	tarName, err = downloadRelease(downloadUrl, dir, progressChan)
	if err != nil {
		err = errors.Wrap(err, "service.DownloadLatestRelease downloadFile error")
		return
	}

	// check tar digest
	digestFileBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		err = errors.Wrap(err, "digestFileContent read error")
		return
	}

	digestFileContent := strings.TrimSpace(string(digestFileBytes))

	logger.Debug("DownloadLatestRelease tar digest", helper.DigestSHA512(tarName))
	logger.Debug("DownloadLatestRelease digestFileContent", digestFileContent)

	if digestFileContent != helper.DigestSHA512(tarName) {
		err = errors.Wrap(err, "digest not equal")
		return
	}

	return
}

func (u *Upgrader) PerformCoreUpgrade(exPath string, tarPath string) (err error) {
	dir := filepath.Dir(exPath)
	err = helper.UnTar(dir, tarPath)
	if err != nil {
		err = errors.Wrap(err, "PerformCoreUpgrade unTar error")
		return
	}
	err = os.Rename(filepath.Join(dir, "nginx-ui"), exPath)
	if err != nil {
		err = errors.Wrap(err, "PerformCoreUpgrade rename error")
		return
	}

	err = os.Remove(tarPath)
	if err != nil {
		err = errors.Wrap(err, "PerformCoreUpgrade remove tar error")
		return
	}
	return
}
