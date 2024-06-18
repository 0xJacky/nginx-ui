package kernal

import (
	"github.com/0xJacky/Nginx-UI/internal/analytic"
	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/cluster"
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/internal/logrotate"
	"github.com/0xJacky/Nginx-UI/internal/validation"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-co-op/gocron"
	"github.com/google/uuid"
	"mime"
	"runtime"
	"time"
)

func Boot() {
	defer recovery()

	async := []func(){
		InitJsExtensionType,
		InitDatabase,
		InitNodeSecret,
		validation.Init,
	}

	syncs := []func(){
		analytic.RecordServerAnalytic,
	}

	for _, v := range async {
		v()
	}

	for _, v := range syncs {
		go v()
	}
}

func InitAfterDatabase() {
	syncs := []func(){
		registerPredefinedUser,
		cert.InitRegister,
		InitCronJobs,
		cluster.RegisterPredefinedNodes,
		analytic.RetrieveNodesStatus,
	}

	for _, v := range syncs {
		go v()
	}
}

func recovery() {
	if err := recover(); err != nil {
		buf := make([]byte, 1024)
		runtime.Stack(buf, false)
		logger.Errorf("%s\n%s", err, buf)
	}
}

func InitDatabase() {
	// Skip install
	if settings.ServerSettings.SkipInstallation {
		skipInstall()
	}

	if "" != settings.ServerSettings.JwtSecret {
		db := model.Init()
		query.Init(db)

		InitAfterDatabase()
	}
}

func InitNodeSecret() {
	if "" == settings.ServerSettings.NodeSecret {
		logger.Warn("NodeSecret is empty, generating...")
		settings.ServerSettings.NodeSecret = uuid.New().String()

		err := settings.Save()
		if err != nil {
			logger.Error("Error save settings")
		}
		logger.Warn("Generated NodeSecret: ", settings.ServerSettings.NodeSecret)
	}
}

func InitJsExtensionType() {
	// Hack: fix wrong Content Type of .js file on some OS platforms
	// See https://github.com/golang/go/issues/32350
	_ = mime.AddExtensionType(".js", "text/javascript; charset=utf-8")
}

func InitCronJobs() {
	s := gocron.NewScheduler(time.UTC)
	job, err := s.Every(6).Hours().SingletonMode().Do(cert.AutoCert)

	if err != nil {
		logger.Fatalf("AutoCert Job: %v, Err: %v\n", job, err)
	}

	job, err = s.Every(settings.LogrotateSettings.Interval).Minute().SingletonMode().Do(logrotate.Exec)

	if err != nil {
		logger.Fatalf("LogRotate Job: %v, Err: %v\n", job, err)
	}

	s.StartAsync()
}
