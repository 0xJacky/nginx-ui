package kernel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"mime"
	"os"
	"path"
	"path/filepath"
	"runtime"

	"github.com/0xJacky/Nginx-UI/internal/analytic"
	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/cluster"
	"github.com/0xJacky/Nginx-UI/internal/cron"
	"github.com/0xJacky/Nginx-UI/internal/docker"
	"github.com/0xJacky/Nginx-UI/internal/event"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log"
	"github.com/0xJacky/Nginx-UI/internal/passkey"
	"github.com/0xJacky/Nginx-UI/internal/self_check"
	"github.com/0xJacky/Nginx-UI/internal/sitecheck"
	"github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/internal/validation"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/google/uuid"
	"github.com/uozi-tech/cosy"
	sqlite "github.com/uozi-tech/cosy-driver-sqlite"
	"github.com/uozi-tech/cosy/logger"
	cModel "github.com/uozi-tech/cosy/model"
	cSettings "github.com/uozi-tech/cosy/settings"
)

var Context context.Context

func Boot(ctx context.Context) {
	defer recovery()

	Context = ctx

	async := []func(){
		InitJsExtensionType,
		InitNodeSecret,
		InitCryptoSecret,
		validation.Init,
		self_check.Init,
		func() {
			InitDatabase(ctx)
			cache.Init(ctx)
		},
		CheckAndCleanupOTA,
	}

	syncs := []func(ctx context.Context){
		analytic.RecordServerAnalytic,
		event.InitEventSystem,
		event.InitWebSocketHub,
	}

	for _, v := range async {
		v()
	}

	for _, v := range syncs {
		go v(ctx)
	}
}

func InitAfterDatabase(ctx context.Context) {
	syncs := []func(ctx context.Context){
		InitUser,
		registerPredefinedUser,
		cluster.RegisterPredefinedNodes,
		RegisterAcmeUser,
	}

	for _, v := range syncs {
		v(ctx)
	}

	asyncs := []func(ctx context.Context){
		cert.InitRegister,
		cron.InitCronJobs,
		analytic.RetrieveNodesStatus,
		passkey.Init,
		mcp.Init,
		sitecheck.Init,
		nginx_log.InitializeModernServices,
		nginx_log.InitTaskRecovery,
		user.InitTokenCache,
	}

	for _, v := range asyncs {
		go v(ctx)
	}
}

func recovery() {
	if err := recover(); err != nil {
		buf := make([]byte, 1024)
		runtime.Stack(buf, false)
		logger.Errorf("%s\n%s", err, buf)
	}
}

func InitDatabase(ctx context.Context) {
	cModel.ResolvedModels()
	// Skip install
	if settings.NodeSettings.SkipInstallation {
		skipInstall()
	}

	db := cosy.InitDB(sqlite.Open(path.Dir(cSettings.ConfPath), settings.DatabaseSettings))
	model.Use(db)
	query.Init(db)

	InitAfterDatabase(ctx)
}

func InitNodeSecret() {
	if settings.NodeSettings.Secret == "" {
		logger.Info("Secret is empty, generating...")
		uuidStr := uuid.New().String()
		settings.NodeSettings.Secret = uuidStr

		err := settings.Save()
		if err != nil {
			logger.Error("Error save settings", err)
		}
		logger.Info("Generated Secret: ", uuidStr)
	}
}

func InitCryptoSecret() {
	if settings.CryptoSettings.Secret == "" {
		logger.Info("Secret is empty, generating...")

		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			logger.Error("Generate Secret failed: ", err)
			return
		}

		settings.CryptoSettings.Secret = hex.EncodeToString(key)

		err := settings.Save()
		if err != nil {
			logger.Error("Error save settings", err)
		}
		logger.Info("Secret Generated")
	}
}

func InitJsExtensionType() {
	// Hack: fix wrong Content Type of .js file on some OS platforms
	// See https://github.com/golang/go/issues/32350
	_ = mime.AddExtensionType(".js", "text/javascript; charset=utf-8")
}

// CheckAndCleanupOTA Check and cleanup OTA update temporary containers
func CheckAndCleanupOTA() {
	if !helper.InNginxUIOfficialDocker() {
		// If running on Windows, clean up .nginx-ui.old.* files
		if runtime.GOOS == "windows" {
			execPath, err := os.Executable()
			if err != nil {
				logger.Error("Failed to get executable path:", err)
				return
			}

			execDir := filepath.Dir(execPath)
			logger.Info("Cleaning up .nginx-ui.old.* files on Windows in:", execDir)

			pattern := filepath.Join(execDir, ".nginx-ui.old.*")
			files, err := filepath.Glob(pattern)
			if err != nil {
				logger.Error("Failed to list .nginx-ui.old.* files:", err)
			} else {
				for _, file := range files {
					_ = os.Remove(file)
				}
			}
		}
		return
	}
	// Execute the third step cleanup operation at startup
	err := docker.UpgradeStepThree()
	if err != nil {
		logger.Error("Failed to cleanup OTA containers:", err)
	}
}
