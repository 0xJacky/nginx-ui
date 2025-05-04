package kernel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"mime"
	"path"
	"runtime"

	"github.com/0xJacky/Nginx-UI/internal/analytic"
	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/cluster"
	"github.com/0xJacky/Nginx-UI/internal/cron"
	"github.com/0xJacky/Nginx-UI/internal/docker"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/passkey"
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

func Boot(ctx context.Context) {
	defer recovery()

	async := []func(){
		InitJsExtensionType,
		InitNodeSecret,
		InitCryptoSecret,
		validation.Init,
		cache.Init,
		CheckAndCleanupOTAContainers,
	}

	syncs := []func(ctx context.Context){
		analytic.RecordServerAnalytic,
		InitDatabase,
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
		registerPredefinedUser,
		cert.InitRegister,
		cron.InitCronJobs,
		cluster.RegisterPredefinedNodes,
		analytic.RetrieveNodesStatus,
		passkey.Init,
		RegisterAcmeUser,
		mcp.Init,
	}

	for _, v := range syncs {
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

var installChan = make(chan struct{})

func PostInstall() {
	installChan <- struct{}{}
}

func InitDatabase(ctx context.Context) {
	cModel.ResolvedModels()
	// Skip install
	if settings.NodeSettings.SkipInstallation {
		skipInstall()
	}

	if cSettings.AppSettings.JwtSecret == "" {
		<-installChan
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

// CheckAndCleanupOTAContainers Check and cleanup OTA update temporary containers
func CheckAndCleanupOTAContainers() {
	if !helper.InNginxUIOfficialDocker() {
		return
	}
	// Execute the third step cleanup operation at startup
	err := docker.UpgradeStepThree()
	if err != nil {
		logger.Error("Failed to cleanup OTA containers:", err)
	}
}
