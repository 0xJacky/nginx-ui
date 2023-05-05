package service

import (
	"encoding/json"
	"fmt"
	_github "github.com/0xJacky/Nginx-UI/.github"
	"github.com/0xJacky/Nginx-UI/frontend"
	"github.com/0xJacky/Nginx-UI/server/internal/helper"
	"github.com/0xJacky/Nginx-UI/server/settings"
	"github.com/pkg/errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

const GithubLatestReleaseAPI = "https://api.github.com/repos/0xJacky/nginx-ui/releases/latest"

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
	Assets      []TReleaseAsset `json:"assets"`
	RuntimeInfo
}

func GetRelease() (data TRelease, err error) {
	resp, err := http.Get(GithubLatestReleaseAPI)
	if err != nil {
		err = errors.Wrap(err, "service.GetReleaseList http.Get err")
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		err = errors.Wrap(err, "service.GetReleaseList io.ReadAll err")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		err = errors.New(string(body))
		log.Println(string(body))
		return
	}
	err = json.Unmarshal(body, &data)
	if err != nil {
		err = errors.Wrap(err, "service.GetReleaseList json.Unmarshal err")
		return
	}
	data.RuntimeInfo, err = GetRuntimeInfo()
	return
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
}

func NewUpgrader() (u *Upgrader, err error) {
	data, err := GetRelease()
	if err != nil {
		return
	}
	u = &Upgrader{
		Release: data,
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

	build, ok := buildJson[u.Release.OS]
	if !ok {
		err = errors.Wrap(err, "os not support upgrade")
		return
	}
	arch, ok := build[u.Release.Arch]
	if !ok {
		err = errors.Wrap(err, "arch not support upgrade")
		return
	}
	var downloadUrl string
	for _, v := range u.Release.Assets {
		if fmt.Sprintf("nginx-ui-%s.tar.gz", arch.Name) == v.Name {
			downloadUrl = v.BrowserDownloadUrl
			break
		}
	}

	if downloadUrl == "" {
		err = errors.Wrap(err, "Nginx UI core downloadUrl is empty")
		return
	}

	dir := filepath.Dir(u.Release.ExPath)

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
