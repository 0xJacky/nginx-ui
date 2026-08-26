package upgrader

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"code.pfad.fr/risefront"
	_github "github.com/0xJacky/Nginx-UI/.github"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/version"
	"github.com/minio/selfupdate"
	"github.com/pkg/errors"
	"github.com/uozi-tech/cosy/logger"
)

const (
	UpgradeStatusInfo     = "info"
	UpgradeStatusError    = "error"
	UpgradeStatusProgress = "progress"
)

type CoreUpgradeResp struct {
	Status   string  `json:"status"`
	Progress float64 `json:"progress"`
	Message  string  `json:"message"`
}

type Upgrader struct {
	Channel string
	Release version.TRelease
	version.RuntimeInfo
}

func NewUpgrader(channel string) (u *Upgrader, err error) {
	data, err := version.GetRelease(channel)
	if err != nil {
		return
	}
	runtimeInfo, err := version.GetRuntimeInfo()
	if err != nil {
		return
	}
	u = &Upgrader{
		Channel:     channel,
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
	if pw.totalSize > 0 && pw.progressChan != nil {
		progress := float64(pw.currentSize) / float64(pw.totalSize) * 100
		pw.progressChan <- progress
	}
	return n, err
}

func downloadRelease(url string, dir string, progressChan chan float64) (tarName string, err error) {
	client, err := version.NewHTTPClient()
	if err != nil {
		return
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		err = fmt.Errorf("download release: unexpected HTTP status %s", resp.Status)
		return
	}

	var totalSize int64
	if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
		totalSize, err = strconv.ParseInt(contentLength, 10, 64)
		if err != nil {
			return
		}
	}

	file, err := os.CreateTemp(dir, "nginx-ui-temp-*.tar.gz")
	if err != nil {
		err = errors.Wrap(err, "service.DownloadLatestRelease CreateTemp error")
		return
	}
	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(file.Name())
		}
	}()

	progressWriter := &ProgressWriter{Writer: file, totalSize: totalSize, progressChan: progressChan}
	multiWriter := io.MultiWriter(progressWriter)

	_, err = io.Copy(multiWriter, resp.Body)

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

	if err = json.Unmarshal(bytes, &buildJson); err != nil {
		err = errors.Wrap(err, "service.DownloadLatestRelease parse build_info.json error")
		return
	}

	build, ok := buildJson[u.OS]
	if !ok {
		err = ErrUnsupportedPlatform
		return
	}
	arch, ok := build[u.Arch]
	if !ok {
		err = ErrUnsupportedPlatform
		return
	}

	assetsMap := u.Release.GetAssetsMap()

	// asset
	assetName := fmt.Sprintf("nginx-ui-%s.tar.gz", arch.Name)
	asset, ok := assetsMap[assetName]

	if !ok {
		err = ErrReleaseAssetEmpty
		return
	}

	downloadUrl := asset.BrowserDownloadUrl
	if downloadUrl == "" {
		err = ErrDownloadUrlEmpty
		return
	}

	// authenticity signature
	signatureAsset, ok := assetsMap[assetName+".minisig"]
	if !ok || signatureAsset.BrowserDownloadUrl == "" {
		err = ErrSignatureEmpty
		return
	}

	// digest
	digest, ok := assetsMap[fmt.Sprintf("nginx-ui-%s.tar.gz.digest", arch.Name)]
	if !ok || digest.BrowserDownloadUrl == "" {
		err = ErrDigestEmpty
		return
	}

	if u.Channel != string(version.ReleaseTypeDev) {
		digest.BrowserDownloadUrl = version.GetUrl(digest.BrowserDownloadUrl)
	}

	dir := filepath.Dir(u.ExPath)

	if u.Channel != string(version.ReleaseTypeDev) {
		downloadUrl = version.GetUrl(downloadUrl)
		signatureAsset.BrowserDownloadUrl = version.GetUrl(signatureAsset.BrowserDownloadUrl)
	}

	tarName, err = downloadRelease(downloadUrl, dir, progressChan)
	if err != nil {
		err = errors.Wrap(err, "service.DownloadLatestRelease downloadFile error")
		return
	}
	defer func() {
		if err != nil {
			_ = os.Remove(tarName)
			_ = os.Remove(tarName + ".minisig")
		}
	}()

	signatureBytes, err := downloadMetadata(signatureAsset.BrowserDownloadUrl, 64<<10)
	if err != nil {
		err = errors.Wrap(err, "upgrader core download signature fail")
		return
	}
	keyID, err := verifyArchiveSignature(tarName, signatureBytes)
	if err != nil {
		return
	}
	if err = os.WriteFile(tarName+".minisig", signatureBytes, 0o600); err != nil {
		err = errors.Wrap(err, "stage verified upgrader signature")
		return
	}
	logger.Debug("DownloadLatestRelease verified Minisign key", fmt.Sprintf("%016X", keyID))

	// check tar digest
	digestFileBytes, err := downloadMetadata(digest.BrowserDownloadUrl, 4<<10)
	if err != nil {
		err = errors.Wrap(err, "upgrader core download digest fail")
		return
	}

	digestFileContent := strings.TrimSpace(string(digestFileBytes))

	logger.Debug("DownloadLatestRelease tar digest", helper.DigestSHA512(tarName))
	logger.Debug("DownloadLatestRelease digestFileContent", digestFileContent)

	if digestFileContent == "" {
		err = ErrDigestFileEmpty
		return
	}

	exeSHA512 := helper.DigestSHA512(tarName)
	if exeSHA512 == "" {
		err = ErrExecutableBinaryEmpty
		return
	}

	if digestFileContent != exeSHA512 {
		err = ErrDigestMismatch
		return
	}

	return
}

func downloadMetadata(url string, maxBytes int64) ([]byte, error) {
	client, err := version.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("download metadata: unexpected HTTP status %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, errors.New("downloaded metadata exceeds size limit")
	}
	return data, nil
}

var updateInProgress atomic.Bool

func (u *Upgrader) PerformCoreUpgrade(tarPath string) (err error) {
	if !updateInProgress.CompareAndSwap(false, true) {
		return ErrUpdateInProgress
	}
	defer updateInProgress.Store(false)
	if _, err = verifyAdjacentArchiveSignature(tarPath); err != nil {
		return err
	}

	oldExe := ""
	if runtime.GOOS == "windows" {
		oldExe = filepath.Join(filepath.Dir(u.ExPath), ".nginx-ui.old."+strconv.FormatInt(time.Now().Unix(), 10))
	}

	opts := selfupdate.Options{
		OldSavePath: oldExe,
	}

	if err = opts.CheckPermissions(); err != nil {
		return err
	}

	tempDir, err := os.MkdirTemp("", "nginx-ui-upgrade-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	err = helper.UnTar(tempDir, tarPath)
	if err != nil {
		err = errors.Wrap(err, "PerformCoreUpgrade unTar error")
		return
	}

	nginxUIExName := "nginx-ui"

	if u.OS == "windows" {
		nginxUIExName = "nginx-ui.exe"
	}

	f, err := os.Open(filepath.Join(tempDir, nginxUIExName))
	if err != nil {
		err = errors.Wrap(err, "PerformCoreUpgrade open error")
		return
	}
	defer f.Close()

	if err = selfupdate.PrepareAndCheckBinary(f, opts); err != nil {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			return pathErr.Err
		}
		return err
	}

	if err = selfupdate.CommitBinary(opts); err != nil {
		if rerr := selfupdate.RollbackError(err); rerr != nil {
			return rerr
		}
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			return pathErr.Err
		}
		return err
	}

	// wait for the file to be written
	time.Sleep(1 * time.Second)

	// gracefully restart
	risefront.Restart()
	return
}

func verifyAdjacentArchiveSignature(archivePath string) (uint64, error) {
	signature, err := os.ReadFile(archivePath + ".minisig")
	if err != nil {
		if os.IsNotExist(err) {
			return 0, ErrSignatureEmpty
		}
		return 0, err
	}
	return verifyArchiveSignature(archivePath, signature)
}
